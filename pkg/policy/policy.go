package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"github.com/theMagicalKarp/agronomist/pkg/storage"
)

type ScalingPolicy struct {
	Name            string
	Deployment      string
	Namespace       string
	ResourceVersion string
	Compiler        *ast.Compiler

	Min           int
	Max           int
	MaxStepUp     int
	MaxStepDown   int
	UpThrottle    time.Duration
	DownThrottle  time.Duration
	CheckInterval int
	LastScale     time.Time
}


func CreateScalingPolicy(obj *unstructured.Unstructured) (*ScalingPolicy, error) {
	deployment, exists, err := unstructured.NestedString(obj.Object, "spec", "deployment")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.deployment` not specified!")
	}

	regoSrc, exists, err := unstructured.NestedString(obj.Object, "spec", "rego")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.rego` not specified!")
	}

	min, exists, err := unstructured.NestedInt64(obj.Object, "spec", "min")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.min` not specified!")
	}

	max, exists, err := unstructured.NestedInt64(obj.Object, "spec", "max")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.max` not specified!")
	}

	maxStepUp, exists, err := unstructured.NestedInt64(obj.Object, "spec", "maxStepUp")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.maxStepUp` not specified!")
	}

	maxStepDown, exists, err := unstructured.NestedInt64(obj.Object, "spec", "maxStepDown")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.maxStepDown` not specified!")
	}

	upDelay, exists, err := unstructured.NestedInt64(obj.Object, "spec", "upDelay")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.upDelay` not specified!")
	}

	downDelay, exists, err := unstructured.NestedInt64(obj.Object, "spec", "downDelay")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.downDelay` not specified!")
	}

	interval, exists, err := unstructured.NestedInt64(obj.Object, "spec", "interval")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%s Scaling Policy `spec.interval` not specified!")
	}

	compiler, err := ast.CompileModules(map[string]string{
		"main.rego": regoSrc,
	})

	if err != nil {
		return nil, err
	}

	return &ScalingPolicy{
		Name:            obj.GetName(),
		Deployment:      deployment,
		Namespace:       obj.GetNamespace(),
		ResourceVersion: obj.GetResourceVersion(),
		Compiler:        compiler,

		Min:           int(min),
		Max:           int(max),
		MaxStepUp:     int(maxStepUp),
		MaxStepDown:   int(maxStepDown),
		UpThrottle:    time.Duration(upDelay)*time.Second,
		DownThrottle:  time.Duration(downDelay)*time.Second,
		CheckInterval: int(interval),
	}, nil
}

func (s *ScalingPolicy) Run(ctx context.Context, store *storage.Store) {

	for {
		select {
		case <-time.After(time.Duration(s.CheckInterval) * time.Second):
			scale, err := s.DetermineScale(ctx, store)
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = s.Scale(ctx, scale, store)

			if err != nil {
				fmt.Println(err)
				continue
			}
		case <-ctx.Done():
			fmt.Println("I'm dead!")
			return
		}
	}
}

func (s *ScalingPolicy) Normalize(scale, replicas int) int {
	if scale < s.Min {
		scale = s.Min
	}

	if scale > s.Max {
		scale = s.Max
	}

	// going up
	if scale > replicas {
		if scale-replicas > s.MaxStepUp {
			return replicas + s.MaxStepUp
		}
		return scale
	}

	// going down
	if scale < replicas {
		if replicas-scale > s.MaxStepDown {
			return replicas - s.MaxStepUp
		}
		return scale
	}

	return scale
}

func (s *ScalingPolicy) Scale(ctx context.Context, scale int, store *storage.Store) error {
	deployment, exists, err := store.DeploymentCache.GetDeployment(s.Namespace, s.Deployment)

	if !exists {
		fmt.Printf("Deployment DNE %s/%s\n", s.Namespace, s.Deployment)
		return nil
	}

	if err != nil {
		return err
	}

	replicas := 1
	if deployment.Spec.Replicas != nil {
		replicas = int(*deployment.Spec.Replicas)
	}

	scale = s.Normalize(scale, replicas)

	if replicas == scale {
		fmt.Println("nothing to do")
		return nil
	}

	if scale > replicas && time.Since(s.LastScale) < s.UpThrottle {
		fmt.Println("TOO SOON UP!")
		return nil
	}

	if scale < replicas && time.Since(s.LastScale) < s.DownThrottle {
		fmt.Println("TOO SOON DOWN!")
		return nil
	}

	s.LastScale = time.Now()
	fmt.Printf("scaling to %d\n", scale)

	deploymentsClient := store.ClientSet.AppsV1().Deployments(s.Namespace)

	foo, err := deploymentsClient.GetScale(ctx, s.Deployment, metav1.GetOptions{})
	if err != nil {
		return err
	}

	foo.Spec.Replicas = int32(scale)

	_, err = deploymentsClient.UpdateScale(ctx, s.Deployment, foo, metav1.UpdateOptions{})
	return err

}

func (s *ScalingPolicy) DetermineScale(ctx context.Context, storage *storage.Store) (int, error) {
	deployment, exists, err := storage.DeploymentCache.GetDeployment(s.Namespace, s.Deployment)

	if err != nil {
		return 0, err
	}

	if !exists {
		return 0, fmt.Errorf("Deployment DNE")
	}

	var podNames []string
	for _, replicaSet := range storage.ReplicaSetCache.GetReplicaSetsByOwnerUID(deployment.UID) {
		rs, exists, err := storage.ReplicaSetCache.GetReplicaSet(s.Namespace, replicaSet)

		if err != nil {
			return 0, err
		}

		if !exists {
			fmt.Println("? Replicaset DNE ?")
			continue
		}

		podNames = append(podNames, storage.PodCache.GetPodsByOwnerUID(rs.UID)...)
	}

	var podMetrics []*metricsv1beta1.PodMetrics
	var pods []*coreV1.Pod
	for _, podName := range podNames {
		podMetric, err := storage.MetricsClientset.MetricsV1beta1().PodMetricses(s.Namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			// shoulds pods be included if metrics DNE?
			fmt.Printf("Pod Metrics not ready %s\n", podName)
			continue
		}
		podMetrics = append(podMetrics, podMetric)

		pod, exists, err := storage.PodCache.GetPod(s.Namespace, podName)

		if err != nil {
			return 0, err
		}

		if !exists {
			fmt.Println("? POD DNE ?")
			continue
		}

		pods = append(pods, pod)
	}

	input := map[string]interface{}{
		"podMetrics": podMetrics,
		"deployment": deployment,
		"pods":       pods,
	}

	r := rego.New(
		rego.Query("data.main.scale"),
		rego.Compiler(s.Compiler),
		rego.Input(input),
	)

	query, err := r.PrepareForEval(ctx)
	if err != nil {
		return 0, err
	}
	rs, err := query.Eval(ctx)
	if err != nil {
		return 0, err
	}

	if len(rs) < 1 {
		return 0,fmt.Errorf("INVALID REGO RESPONSE")
	}

	if len(rs[0].Expressions) < 1 {
		return 0,fmt.Errorf("INVALID REGO RESPONSE")
	}

	jsonNumber, ok := rs[0].Expressions[0].Value.(json.Number)
	if !ok {
		return 0,fmt.Errorf("INCORRECT RESPONSE TYPE")
	}

	num, err := jsonNumber.Int64()
	if err != nil {
		return 0, err
	}

	return int(num), nil
}
