package storage

import (
	"context"
	"fmt"
	"sync"

	appsV1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type ReplicaSetCache struct {
	Informer   cache.SharedIndexInformer
	OwnerCache map[types.UID]map[string]bool
	Mutex      sync.RWMutex
}

func CreateReplicaSetCache(informer cache.SharedIndexInformer) *ReplicaSetCache {
	rsCache := &ReplicaSetCache{
		Informer:   informer,
		OwnerCache: make(map[types.UID]map[string]bool),
	}

	informer.AddEventHandler(rsCache)

	return rsCache
}

func (r *ReplicaSetCache) Start(ctx context.Context) {
	r.Informer.Run(ctx.Done())
}

func (r *ReplicaSetCache) GetReplicaSetsByOwnerUID(uid types.UID) []string {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	var replicaSets []string
	for replicaSet := range r.OwnerCache[uid] {
		replicaSets = append(replicaSets, replicaSet)
	}

	return replicaSets
}

func (r *ReplicaSetCache) GetReplicaSet(namespace, name string) (*appsV1.ReplicaSet, bool, error) {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	item, exists, err := r.Informer.GetStore().GetByKey(fmt.Sprintf("%s/%s", namespace, name))

	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	return item.(*appsV1.ReplicaSet), true, nil
}

func (r *ReplicaSetCache) OnAdd(obj interface{}) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	replicaSet := obj.(*appsV1.ReplicaSet)

	for _, ownerRef := range replicaSet.OwnerReferences {
		if r.OwnerCache[ownerRef.UID] == nil {
			r.OwnerCache[ownerRef.UID] = make(map[string]bool)
		}
		r.OwnerCache[ownerRef.UID][replicaSet.Name] = true
	}
}

func (r *ReplicaSetCache) OnUpdate(oldObj, newObj interface{}) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	oldReplicaSet := oldObj.(*appsV1.ReplicaSet)

	for _, ownerRef := range oldReplicaSet.OwnerReferences {

		if r.OwnerCache[ownerRef.UID] == nil {
			continue
		}

		if !r.OwnerCache[ownerRef.UID][oldReplicaSet.Name] {
			continue
		}

		delete(r.OwnerCache[ownerRef.UID], oldReplicaSet.Name)
	}

	newReplicaSet := newObj.(*appsV1.ReplicaSet)

	for _, ownerRef := range newReplicaSet.OwnerReferences {
		if r.OwnerCache[ownerRef.UID] == nil {
			r.OwnerCache[ownerRef.UID] = make(map[string]bool)
		}
		r.OwnerCache[ownerRef.UID][newReplicaSet.Name] = true
	}

}

func (r *ReplicaSetCache) OnDelete(obj interface{}) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	replicaSet := obj.(*appsV1.ReplicaSet)

	for _, ownerRef := range replicaSet.OwnerReferences {

		if r.OwnerCache[ownerRef.UID] == nil {
			continue
		}

		if !r.OwnerCache[ownerRef.UID][replicaSet.Name] {
			continue
		}

		delete(r.OwnerCache[ownerRef.UID], replicaSet.Name)
	}
}
