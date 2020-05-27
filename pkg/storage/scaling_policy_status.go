package storage

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/tools/cache"
)

type ScalingPolicyStatusCache struct {
	Informer cache.SharedIndexInformer
}

func CreateScalingPolicyStatusCache(informer cache.SharedIndexInformer) *ScalingPolicyStatusCache {
	scalingPolicyStatusCache := &ScalingPolicyStatusCache{
		Informer: informer,
	}

	informer.AddEventHandler(scalingPolicyStatusCache)
	return scalingPolicyStatusCache
}

func (s *ScalingPolicyStatusCache) Start(ctx context.Context) {
	s.Informer.Run(ctx.Done())
}

func (s *ScalingPolicyStatusCache) ListScalingPolicyStatuses() []interface{} {
	return s.Informer.GetStore().List()
}

func (s *ScalingPolicyStatusCache) GetScalingPolicy(namespace, name string) (*unstructured.Unstructured, bool, error) {
	item, exists, err := s.Informer.GetStore().GetByKey(fmt.Sprintf("%s/%s", namespace, name))

	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	return item.(*unstructured.Unstructured), true, nil
}

func (s *ScalingPolicyStatusCache) OnAdd(obj interface{}) {
}

func (s *ScalingPolicyStatusCache) OnUpdate(oldObj, newObj interface{}) {
}

func (s *ScalingPolicyStatusCache) OnDelete(obj interface{}) {
}
