package reconciler

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/theMagicalKarp/agronomist/pkg/policy"
	"github.com/theMagicalKarp/agronomist/pkg/storage"
)

type ScalingPolicyReconciler struct {
	OwnerName      string
	OwnerNamespace string
	OwnerUID       types.UID

	Interval int

	PolicyRegistry *policy.PolicyRegistry

	Store *storage.Store
}

func CreateScalingPolicyReconciler(ownerNamespace, ownerName string, ownerUID types.UID, store *storage.Store) *ScalingPolicyReconciler {
	return &ScalingPolicyReconciler{
		OwnerName:      ownerName,
		OwnerNamespace: ownerNamespace,
		OwnerUID:       ownerUID,

		Interval: 1,

		PolicyRegistry: policy.CreatePolicyRegistry(),

		Store: store,
	}
}

func (s *ScalingPolicyReconciler) AttemptClaims(ctx context.Context) error {
	scalingPolicyStatuses := s.Store.ScalingPolicyStatusCache.ListScalingPolicyStatuses()
	scalingPolicies := s.Store.ScalingPolicyCache.ListScalingPolicies()

	scalingPolicyStatusSet := make(map[string]bool)
	for _, item := range scalingPolicyStatuses {
		scalingPolicyStatus := item.(*unstructured.Unstructured)

		scalingPolicyStatusSet[scalingPolicyStatus.GetName()] = true
	}

	// create status if not exists
	for _, item := range scalingPolicies {
		scalingPolicy := item.(*unstructured.Unstructured)

		key := fmt.Sprintf("%s--%s", scalingPolicy.GetNamespace(), scalingPolicy.GetName())

		if scalingPolicyStatusSet[key] {
			continue
		}

		un := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       "ScalingPolicyStatus",
				"apiVersion": "agronomist.io/v1",
				"metadata": map[string]interface{}{
					// Need to encode names better to avoid namespace/name collisions
					"name":      fmt.Sprintf("%s--%s", scalingPolicy.GetNamespace(), scalingPolicy.GetName()),
					"namespace": s.OwnerNamespace, // since ownerReferences is namespaced
					"labels": map[string]string{
						"policy-namespace": scalingPolicy.GetNamespace(),
						"policy-name":      scalingPolicy.GetName(),
					},
					"ownerReferences": []metav1.OwnerReference{
						metav1.OwnerReference{
							APIVersion: "v1",
							Kind:       "Pod",
							Name:       s.OwnerName,
							UID:        s.OwnerUID,
						},
					},
				},
				"spec": map[string]interface{}{
					"error": "",
				},
			},
		}

		_, err := s.Store.DynamicClientset.Resource(schema.GroupVersionResource{
			Group:    "agronomist.io",
			Version:  "v1",
			Resource: "scalingpolicystatuses",
		}).Namespace(s.OwnerNamespace).Create(ctx, un, metav1.CreateOptions{})

		if err != nil {
			return err
		}
	}

	// if there are claims we own, that we haven't started yet, start them
	for _, item := range scalingPolicyStatuses {
		scalingPolicyStatus := item.(*unstructured.Unstructured)

		for _, or := range scalingPolicyStatus.GetOwnerReferences() {
			if or.Name != s.OwnerName || or.UID != s.OwnerUID {
				continue
			}

			namespace := scalingPolicyStatus.GetLabels()["policy-namespace"]
			name := scalingPolicyStatus.GetLabels()["policy-name"]

			if s.PolicyRegistry.Exists(namespace, name) {
				continue
			}
			scalingPolicy, exists, err := s.Store.ScalingPolicyCache.GetScalingPolicy(namespace, name)

			if !exists {
				fmt.Printf("%s/%s Scaling Policy DNE?\n", namespace, name)
				continue
			}

			if err != nil {
				return err
			}

			err = s.PolicyRegistry.Add(ctx, scalingPolicy, s.Store)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *ScalingPolicyReconciler) UpdateClaims(ctx context.Context) error {
	// if claims resource id has changed, cancel/update policy
	for _, item := range s.Store.ScalingPolicyCache.ListScalingPolicies() {
		scalingPolicy := item.(*unstructured.Unstructured)

		if !s.PolicyRegistry.NeedsUpdate(scalingPolicy) {
			continue
		}

		err := s.PolicyRegistry.Update(ctx, scalingPolicy, s.Store)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *ScalingPolicyReconciler) Cleanup(ctx context.Context) error {
	scalingPolicyStatuses := s.Store.ScalingPolicyStatusCache.ListScalingPolicyStatuses()
	scalingPolicies := s.Store.ScalingPolicyCache.ListScalingPolicies()

	scalingPolicySet := make(map[string]bool)

	for _, item := range scalingPolicies {
		scalingPolicy := item.(*unstructured.Unstructured)

		key := fmt.Sprintf("%s--%s", scalingPolicy.GetNamespace(), scalingPolicy.GetName())
		scalingPolicySet[key] = true
	}

	// determine if there exists a status without a policy
	for _, item := range scalingPolicyStatuses {
		scalingPolicyStatus := item.(*unstructured.Unstructured)
		if scalingPolicySet[scalingPolicyStatus.GetName()] {
			continue
		}

		// this is dangerious! we should look at determining if resource is safe to delete
		// since both caches could be out of sync
		err := s.Store.DynamicClientset.Resource(schema.GroupVersionResource{
			Group:    "agronomist.io",
			Version:  "v1",
			Resource: "scalingpolicystatuses",
		}).Namespace(s.OwnerNamespace).Delete(ctx, scalingPolicyStatus.GetName(), metav1.DeleteOptions{})

		if err != nil {
			return err
		}
	}

	// determine if we have a scaling policy which we don't own anymore
	ownedPolicyStatuses := make(map[string]bool)
	for _, item := range scalingPolicyStatuses {
		scalingPolicyStatus := item.(*unstructured.Unstructured)

		for _, or := range scalingPolicyStatus.GetOwnerReferences() {
			if or.Name != s.OwnerName || or.UID != s.OwnerUID {
				continue
			}
			ownedPolicyStatuses[scalingPolicyStatus.GetName()] = true
		}
	}

	for _, storedPolicy := range s.PolicyRegistry.Policies {

		if ownedPolicyStatuses[fmt.Sprintf("%s--%s", storedPolicy.Namespace, storedPolicy.Name)] {
			continue
		}

		s.PolicyRegistry.Remove(storedPolicy.Namespace, storedPolicy.Name)
	}

	return nil
}

func (s *ScalingPolicyReconciler) Start(ctx context.Context) {
	for {
		select {
		case <-time.After(time.Duration(s.Interval) * time.Second):
			err := s.AttemptClaims(ctx)
			if err != nil {
				fmt.Println(err)
			}

			err = s.UpdateClaims(ctx)
			if err != nil {
				fmt.Println(err)
			}

			err = s.Cleanup(ctx)

			if err != nil {
				fmt.Println(err)
			}
		case <-ctx.Done():
			return
		}
	}
}
