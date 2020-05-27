package policy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/theMagicalKarp/agronomist/pkg/storage"
)

type PolicyRegistry struct {
	Policies  map[string]*ScalingPolicy
	CancelMap map[string]context.CancelFunc
}

func CreatePolicyRegistry() *PolicyRegistry {
	return &PolicyRegistry{
		Policies:  make(map[string]*ScalingPolicy),
		CancelMap: make(map[string]context.CancelFunc),
	}
}

func (p *PolicyRegistry) Exists(policyNamespace, policyName string) bool {
	return p.Policies[fmt.Sprintf("%s:%s", policyNamespace, policyName)] != nil
}

func (p *PolicyRegistry) NeedsUpdate(obj *unstructured.Unstructured) bool {
	index := fmt.Sprintf("%s:%s", obj.GetNamespace(), obj.GetName())

	storedPolicy := p.Policies[index]
	if storedPolicy == nil {
		return false
	}

	return storedPolicy.ResourceVersion != obj.GetResourceVersion()
}

func (p *PolicyRegistry) Update(ctx context.Context, obj *unstructured.Unstructured, store *storage.Store) error {
	index := fmt.Sprintf("%s:%s", obj.GetNamespace(), obj.GetName())
	p.CancelMap[index]()
	return p.Add(ctx, obj, store)
}

func (p *PolicyRegistry) Remove(policyNamespace, policyName string) {
	index := fmt.Sprintf("%s:%s", policyNamespace, policyName)
	delete(p.Policies, index)
	p.CancelMap[index]()
	delete(p.CancelMap, index)
}

func (p *PolicyRegistry) Add(ctx context.Context, obj *unstructured.Unstructured, store *storage.Store) error {
	index := fmt.Sprintf("%s:%s", obj.GetNamespace(), obj.GetName())

	sp, err := CreateScalingPolicy(obj)
	if err != nil {
		return err
	}
	childCtx, cancel := context.WithCancel(ctx)
	p.Policies[index] = sp
	p.CancelMap[index] = cancel
	go sp.Run(childCtx, store)

	return nil
}
