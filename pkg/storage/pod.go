package storage

import (
	"context"
	"fmt"
	"sync"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type PodCache struct {
	Informer   cache.SharedIndexInformer
	OwnerCache map[types.UID]map[string]bool
	Mutex      sync.RWMutex
}

func CreatePodCache(informer cache.SharedIndexInformer) *PodCache {
	podCache := &PodCache{
		Informer:   informer,
		OwnerCache: make(map[types.UID]map[string]bool),
	}

	informer.AddEventHandler(podCache)

	return podCache
}

func (p *PodCache) Start(ctx context.Context) {
	p.Informer.Run(ctx.Done())
}

func (p *PodCache) GetPodsByOwnerUID(uid types.UID) []string {
	p.Mutex.RLock()
	defer p.Mutex.RUnlock()

	var pods []string
	for pod := range p.OwnerCache[uid] {
		pods = append(pods, pod)
	}

	return pods
}

func (p *PodCache) GetPod(namespace, name string) (*coreV1.Pod, bool, error) {
	item, exists, err := p.Informer.GetStore().GetByKey(fmt.Sprintf("%s/%s", namespace, name))

	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	return item.(*coreV1.Pod), true, nil
}

func (p *PodCache) OnAdd(obj interface{}) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	pod := obj.(*coreV1.Pod)

	for _, ownerRef := range pod.OwnerReferences {
		if p.OwnerCache[ownerRef.UID] == nil {
			p.OwnerCache[ownerRef.UID] = make(map[string]bool)
		}
		p.OwnerCache[ownerRef.UID][pod.Name] = true
	}
}

func (p *PodCache) OnUpdate(oldObj, newObj interface{}) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	oldPod := oldObj.(*coreV1.Pod)

	for _, ownerRef := range oldPod.OwnerReferences {

		if p.OwnerCache[ownerRef.UID] == nil {
			continue
		}

		if !p.OwnerCache[ownerRef.UID][oldPod.Name] {
			continue
		}

		delete(p.OwnerCache[ownerRef.UID], oldPod.Name)
	}

	newPod := newObj.(*coreV1.Pod)

	for _, ownerRef := range newPod.OwnerReferences {
		if p.OwnerCache[ownerRef.UID] == nil {
			p.OwnerCache[ownerRef.UID] = make(map[string]bool)
		}
		p.OwnerCache[ownerRef.UID][newPod.Name] = true
	}
}

func (p *PodCache) OnDelete(obj interface{}) {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()

	pod := obj.(*coreV1.Pod)

	for _, ownerRef := range pod.OwnerReferences {

		if p.OwnerCache[ownerRef.UID] == nil {
			continue
		}

		if !p.OwnerCache[ownerRef.UID][pod.Name] {
			continue
		}

		delete(p.OwnerCache[ownerRef.UID], pod.Name)
	}

}
