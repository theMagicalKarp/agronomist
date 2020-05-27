package storage

import (
	"context"
	"fmt"

	appsV1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"
)

type DeploymentCache struct {
	Informer cache.SharedIndexInformer
}

func CreateDeploymentCache(informer cache.SharedIndexInformer) *DeploymentCache {
	deploymentCache := &DeploymentCache{
		Informer: informer,
	}

	informer.AddEventHandler(deploymentCache)
	return deploymentCache
}

func (d *DeploymentCache) Start(ctx context.Context) {
	d.Informer.Run(ctx.Done())
}

func (d *DeploymentCache) GetDeployment(namespace, name string) (*appsV1.Deployment, bool, error) {
	item, exists, err := d.Informer.GetStore().GetByKey(fmt.Sprintf("%s/%s", namespace, name))

	if err != nil {
		return nil, false, err
	}

	if !exists {
		return nil, false, nil
	}

	return item.(*appsV1.Deployment), true, nil
}

func (d *DeploymentCache) OnAdd(obj interface{}) {
}

func (d *DeploymentCache) OnUpdate(oldObj, newObj interface{}) {
}

func (d *DeploymentCache) OnDelete(obj interface{}) {
}
