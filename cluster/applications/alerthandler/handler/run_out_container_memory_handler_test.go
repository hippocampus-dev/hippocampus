package handler_test

import (
	"alerthandler/handler"
	"context"
	"errors"
	"testing"
	"time"

	coreV1 "k8s.io/api/core/v1"
	policyV1 "k8s.io/api/policy/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
)

func TestRunOutPodCapacityHandler_Call(t *testing.T) {
	type in struct {
		first *handler.AlertManagerRequest
	}

	tests := []struct {
		name            string
		receiver        *handler.RunOutContainerMemoryHandler
		in              in
		wantErrorString string
	}{
		{
			"do nothing when alerts is empty",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{},
				},
			},
			"",
		},
		{
			"return error when label is missing",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
							},
						},
					},
				},
			},
			"alert must have namespace,pod label",
		},
		{
			"return error when failed to list pdb",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return nil, errors.New("fake")
				},
			}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"failed to list pdb in foo: fake",
		},
		{
			"return error when failed to list pod",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					return nil, errors.New("fake")
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"failed to list pod in foo: fake",
		},
		{
			"convert metaV1.LabelSelector to string like app=fake",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					if diff := cmp.Diff("app=fake", options.LabelSelector); diff != "" {
						t.Errorf("(-want +got):\n%s", diff)
					}
					return &coreV1.PodList{
						Items: []coreV1.Pod{},
					}, nil
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"",
		},
		{
			"do nothing when return unmatched pod",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					return &coreV1.PodList{
						Items: []coreV1.Pod{
							{
								ObjectMeta: metaV1.ObjectMeta{
									Namespace: "fake",
									Name:      "fake",
								},
							},
						},
					}, nil
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"",
		},
		{
			"return error when failed to get pdb",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					return &coreV1.PodList{
						Items: []coreV1.Pod{
							{
								ObjectMeta: metaV1.ObjectMeta{
									Namespace: "foo",
									Name:      "bar",
								},
							},
						},
					}, nil
				},
				fakePodDisruptionBudgetGet: func(ctx context.Context, name string, options metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error) {
					return nil, errors.New("fake")
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"failed to waitForDelete bar/foo: failed to wait PodDisruptionBudget: failed to get pdb in /: fake",
		},
		{
			"timeout when PodDisruptionBudget DisruptionAllowed is 0",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					return &coreV1.PodList{
						Items: []coreV1.Pod{
							{
								ObjectMeta: metaV1.ObjectMeta{
									Namespace: "foo",
									Name:      "bar",
								},
							},
						},
					}, nil
				},
				fakePodDisruptionBudgetGet: func(ctx context.Context, name string, options metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error) {
					return &policyV1.PodDisruptionBudget{
						Status: policyV1.PodDisruptionBudgetStatus{
							DisruptionsAllowed: 0,
						},
					}, nil
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Second),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"failed to waitForDelete bar/foo: failed to wait PodDisruptionBudget: context deadline exceeded",
		},
		{
			"return error when failed to delete pod",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					return &coreV1.PodList{
						Items: []coreV1.Pod{
							{
								ObjectMeta: metaV1.ObjectMeta{
									Namespace: "foo",
									Name:      "bar",
								},
							},
						},
					}, nil
				},
				fakePodDelete: func(ctx context.Context, name string, options metaV1.DeleteOptions) error {
					return errors.New("fake")
				},
				fakePodDisruptionBudgetGet: func(ctx context.Context, name string, options metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error) {
					return &policyV1.PodDisruptionBudget{
						Status: policyV1.PodDisruptionBudgetStatus{
							DisruptionsAllowed: 1,
						},
					}, nil
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Minute*5),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"failed to delete pod bar/foo: fake",
		},
		{
			"success",
			handler.NewRunOutContainerMemoryHandler(&kubernetesClientsetMock{
				fakePodList: func(ctx context.Context, options metaV1.ListOptions) (*coreV1.PodList, error) {
					return &coreV1.PodList{
						Items: []coreV1.Pod{
							{
								ObjectMeta: metaV1.ObjectMeta{
									Namespace: "foo",
									Name:      "bar",
								},
							},
						},
					}, nil
				},
				fakePodDelete: func(ctx context.Context, name string, options metaV1.DeleteOptions) error {
					return nil
				},
				fakePodDisruptionBudgetGet: func(ctx context.Context, name string, options metaV1.GetOptions) (*policyV1.PodDisruptionBudget, error) {
					return &policyV1.PodDisruptionBudget{
						Status: policyV1.PodDisruptionBudgetStatus{
							DisruptionsAllowed: 1,
						},
					}, nil
				},
				fakePodDisruptionBudgetList: func(ctx context.Context, options metaV1.ListOptions) (*policyV1.PodDisruptionBudgetList, error) {
					return &policyV1.PodDisruptionBudgetList{
						Items: []policyV1.PodDisruptionBudget{
							{
								Spec: policyV1.PodDisruptionBudgetSpec{
									Selector: &metaV1.LabelSelector{
										MatchLabels: map[string]string{
											"app": "fake",
										},
									},
								},
							},
						},
					}, nil
				},
			}, time.Millisecond*100, time.Second),
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{
						{
							Labels: map[string]string{
								"namespace": "foo",
								"pod":       "bar",
							},
						},
					},
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		name := tt.name
		receiver := tt.receiver
		in := tt.in
		wantErrorString := tt.wantErrorString
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := receiver.Call(in.first)
			if err == nil {
				if diff := cmp.Diff(wantErrorString, ""); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
		})
	}
}
