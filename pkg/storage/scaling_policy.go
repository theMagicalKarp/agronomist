package storage

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/tools/cache"
)

type ScalingPolicyCache struct {
	Informer cache.SharedIndexInformer
}

func CreateScalingPolicyCache(informer cache.SharedIndexInformer) *ScalingPolicyCache {
	scalingPolicyCache := &ScalingPolicyCache{
		Informer: informer,
	}

	informer.AddEventHandler(scalingPolicyCache)
	return scalingPolicyCache
}

func (s *ScalingPolicyCache) Start(ctx context.Context) {
	s.Informer.Run(ctx.Done())
}

func (s *ScalingPolicyCache) ListScalingPolicies() []interface{} {
	return s.Informer.GetStore().List()
}

func (s *ScalingPolicyCache) GetScalingPolicy(namespace, name string) (*unstructured.Unstructured, bool, error) {
	item, exists, err := s.Informer.GetStore().GetByKey(fmt.Sprintf("%s/%s", namespace, name))

	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	return item.(*unstructured.Unstructured), true, nil
}

func (s *ScalingPolicyCache) OnAdd(obj interface{}) {
}

func (s *ScalingPolicyCache) OnUpdate(oldObj, newObj interface{}) {
}

func (s *ScalingPolicyCache) OnDelete(obj interface{}) {
}
