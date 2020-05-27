package storage

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Store struct {
	ClientSet        *kubernetes.Clientset
	MetricsClientset *metricsv.Clientset
	DynamicClientset dynamic.Interface

	DeploymentCache *DeploymentCache
	ReplicaSetCache *ReplicaSetCache
	PodCache        *PodCache

	ScalingPolicyCache       *ScalingPolicyCache
	ScalingPolicyStatusCache *ScalingPolicyStatusCache
}

func NewStore(clientSet *kubernetes.Clientset, metricsClientset *metricsv.Clientset, dynamicClientset dynamic.Interface, factory informers.SharedInformerFactory, dynamicFactory dynamicinformer.DynamicSharedInformerFactory) *Store {
	scalerGVR := schema.GroupVersionResource{
		Group:    "agronomist.io",
		Version:  "v1",
		Resource: "scalingpolicies",
	}

	scalerStatusGVR := schema.GroupVersionResource{
		Group:    "agronomist.io",
		Version:  "v1",
		Resource: "scalingpolicystatuses",
	}

	return &Store{
		ClientSet:        clientSet,
		MetricsClientset: metricsClientset,
		DynamicClientset: dynamicClientset,

		DeploymentCache: CreateDeploymentCache(factory.Apps().V1().Deployments().Informer()),
		ReplicaSetCache: CreateReplicaSetCache(factory.Apps().V1().ReplicaSets().Informer()),
		PodCache:        CreatePodCache(factory.Core().V1().Pods().Informer()),

		ScalingPolicyCache:       CreateScalingPolicyCache(dynamicFactory.ForResource(scalerGVR).Informer()),
		ScalingPolicyStatusCache: CreateScalingPolicyStatusCache(dynamicFactory.ForResource(scalerStatusGVR).Informer()),
	}

}

func (s *Store) Start(ctx context.Context) {
	go s.DeploymentCache.Start(ctx)
	go s.ReplicaSetCache.Start(ctx)
	go s.PodCache.Start(ctx)

	go s.ScalingPolicyCache.Start(ctx)
	go s.ScalingPolicyStatusCache.Start(ctx)
}
