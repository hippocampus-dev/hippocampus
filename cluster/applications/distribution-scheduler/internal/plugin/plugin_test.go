package plugin

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestFilter(t *testing.T) {
	type in struct {
		state    *stateData
		nodeName string
	}

	tests := []struct {
		name     string
		in       in
		wantCode framework.Code
	}{
		{
			"allow empty node when occupied < N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3}, totalPods: 3},
				nodeName: "node-2",
			},
			framework.Success,
		},
		{
			"allow empty node when occupied >= N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3, "node-2": 3}, totalPods: 6},
				nodeName: "node-3",
			},
			framework.Success,
		},
		{
			"allow occupied node when occupied >= N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3, "node-2": 2}, totalPods: 5},
				nodeName: "node-2",
			},
			framework.Success,
		},
		{
			"reject node when per-node cap exceeded",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 4, "node-2": 3}, totalPods: 7},
				nodeName: "node-1",
			},
			framework.Unschedulable,
		},
		{
			"allow node when per-node cap not exceeded",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3, "node-2": 3}, totalPods: 6},
				nodeName: "node-1",
			},
			framework.Success,
		},
		{
			"allow first pod on any node",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{}, totalPods: 0},
				nodeName: "node-1",
			},
			framework.Success,
		},
		{
			"second pod goes to different node with N=2",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 1}, totalPods: 1},
				nodeName: "node-1",
			},
			framework.Unschedulable,
		},
	}

	for _, tt := range tests {
		name := tt.name
		in := tt.in
		wantCode := tt.wantCode
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cycleState := framework.NewCycleState()
			cycleState.Write(Name, in.state)

			node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: in.nodeName}}
			nodeInfo := framework.NewNodeInfo()
			nodeInfo.SetNode(node)

			d := &DistributionScheduler{}
			status := d.Filter(context.Background(), cycleState, &v1.Pod{}, nodeInfo)

			if diff := cmp.Diff(wantCode, status.Code()); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}

	t.Run("allow when state not set", func(t *testing.T) {
		t.Parallel()

		cycleState := framework.NewCycleState()

		node := &v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
		nodeInfo := framework.NewNodeInfo()
		nodeInfo.SetNode(node)

		d := &DistributionScheduler{}
		status := d.Filter(context.Background(), cycleState, &v1.Pod{}, nodeInfo)

		if diff := cmp.Diff(framework.Success, status.Code()); diff != "" {
			t.Errorf("(-want +got):\n%s", diff)
		}
	})
}

func TestScore(t *testing.T) {
	type in struct {
		state    *stateData
		nodeName string
	}

	tests := []struct {
		name      string
		in        in
		wantScore int64
		wantCode  framework.Code
	}{
		{
			"empty node gets max score when occupied < N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3}, totalPods: 3},
				nodeName: "node-2",
			},
			framework.MaxNodeScore,
			framework.Success,
		},
		{
			"occupied node gets lower score when occupied < N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3}, totalPods: 3},
				nodeName: "node-1",
			},
			framework.MaxNodeScore / 4,
			framework.Success,
		},
		{
			"empty node gets max score when occupied >= N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 3, "node-2": 3}, totalPods: 6},
				nodeName: "node-3",
			},
			framework.MaxNodeScore,
			framework.Success,
		},
		{
			"node with fewer pods gets higher score when occupied >= N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 4, "node-2": 3}, totalPods: 7},
				nodeName: "node-2",
			},
			framework.MaxNodeScore / 4,
			framework.Success,
		},
		{
			"node with more pods gets lower score when occupied >= N",
			in{
				state:    &stateData{distribution: 2, siblingsByNode: map[string]int{"node-1": 4, "node-2": 3}, totalPods: 7},
				nodeName: "node-1",
			},
			framework.MaxNodeScore / 5,
			framework.Success,
		},
	}

	for _, tt := range tests {
		name := tt.name
		in := tt.in
		wantScore := tt.wantScore
		wantCode := tt.wantCode
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cycleState := framework.NewCycleState()
			cycleState.Write(Name, in.state)

			d := &DistributionScheduler{}
			score, status := d.Score(context.Background(), cycleState, &v1.Pod{}, in.nodeName)

			if diff := cmp.Diff(wantCode, status.Code()); diff != "" {
				t.Errorf("status code (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(wantScore, score); diff != "" {
				t.Errorf("score (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("zero score when state not set", func(t *testing.T) {
		t.Parallel()

		cycleState := framework.NewCycleState()

		d := &DistributionScheduler{}
		score, status := d.Score(context.Background(), cycleState, &v1.Pod{}, "node-1")

		if diff := cmp.Diff(framework.Success, status.Code()); diff != "" {
			t.Errorf("status code (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(int64(0), score); diff != "" {
			t.Errorf("score (-want +got):\n%s", diff)
		}
	})
}
