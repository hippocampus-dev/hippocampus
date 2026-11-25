package handler_test

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	policyV1 "k8s.io/api/policy/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	typedPolicyV1 "k8s.io/client-go/kubernetes/typed/policy/v1"
)

type kubernetesClientsetMock struct {
	kubernetes.Interface
	fakePodList                 func(context.Context, metaV1.ListOptions) (*coreV1.PodList, error)
	fakePodDelete               func(context.Context, string, metaV1.DeleteOptions) error
	fakePodDisruptionBudgetGet  func(context.Context, string, metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error)
	fakePodDisruptionBudgetList func(context.Context, metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error)
}

func (k *kubernetesClientsetMock) CoreV1() typedCoreV1.CoreV1Interface {
	return &coreV1Mock{
		fakePodList:   k.fakePodList,
		fakePodDelete: k.fakePodDelete,
	}
}

type coreV1Mock struct {
	typedCoreV1.CoreV1Interface
	fakePodList   func(context.Context, metaV1.ListOptions) (*coreV1.PodList, error)
	fakePodDelete func(context.Context, string, metaV1.DeleteOptions) error
}

func (c *coreV1Mock) Pods(namespace string) typedCoreV1.PodInterface {
	return &podMock{
		fakeList:   c.fakePodList,
		fakeDelete: c.fakePodDelete,
	}
}

type podMock struct {
	typedCoreV1.PodInterface
	fakeList   func(context.Context, metaV1.ListOptions) (*coreV1.PodList, error)
	fakeDelete func(context.Context, string, metaV1.DeleteOptions) error
}

func (p *podMock) List(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
	return p.fakeList(ctx, options)
}
func (p *podMock) Delete(ctx context.Context, name string, options metaV1.DeleteOptions) error {
	return p.fakeDelete(ctx, name, options)
}
func (k *kubernetesClientsetMock) PolicyV1() typedPolicyV1.PolicyV1Interface {
	return &policyV1Mock{
		fakePodDisruptionBudgetGet:  k.fakePodDisruptionBudgetGet,
		fakePodDisruptionBudgetList: k.fakePodDisruptionBudgetList,
	}
}

type policyV1Mock struct {
	typedPolicyV1.PolicyV1Interface
	fakePodDisruptionBudgetGet  func(context.Context, string, metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error)
	fakePodDisruptionBudgetList func(context.Context, metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error)
}

func (p *policyV1Mock) PodDisruptionBudgets(namespace string) typedPolicyV1.PodDisruptionBudgetInterface {
	return &podDisruptionBudgetMock{
		fakeGet:  p.fakePodDisruptionBudgetGet,
		fakeList: p.fakePodDisruptionBudgetList,
	}
}

type podDisruptionBudgetMock struct {
	typedPolicyV1.PodDisruptionBudgetInterface
	fakeGet  func(context.Context, string, metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error)
	fakeList func(context.Context, metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error)
}

func (p *podDisruptionBudgetMock) Get(ctx context.Context, name string, options metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error) {
	return p.fakeGet(ctx, name, options)
}
func (p *podDisruptionBudgetMock) List(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
	return p.fakeList(ctx, options)
}
