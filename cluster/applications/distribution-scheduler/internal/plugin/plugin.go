package plugin

import (
	"context"
	"fmt"
	"math"
	"strconv"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

const Name = "DistributionScheduler"

type DistributionScheduler struct {
	handle framework.Handle
}

type stateData struct {
	distribution   int
	siblingsByNode map[string]int
	totalPods      int
}

func (s *stateData) Clone() framework.StateData {
	m := make(map[string]int, len(s.siblingsByNode))
	for k, v := range s.siblingsByNode {
		m[k] = v
	}
	return &stateData{
		distribution:   s.distribution,
		siblingsByNode: m,
		totalPods:      s.totalPods,
	}
}

var (
	_ framework.PreFilterPlugin = &DistributionScheduler{}
	_ framework.FilterPlugin    = &DistributionScheduler{}
	_ framework.ScorePlugin     = &DistributionScheduler{}
)

func New(_ context.Context, _ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &DistributionScheduler{handle: h}, nil
}

func (d *DistributionScheduler) Name() string {
	return Name
}

func (d *DistributionScheduler) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	value, ok := pod.Annotations["distribution-scheduler.kaidotio.github.io/min"]
	if !ok {
		return nil, framework.NewStatus(framework.Skip)
	}

	distribution, err := strconv.Atoi(value)
	if err != nil || distribution <= 0 {
		return nil, framework.NewStatus(framework.Error, fmt.Sprintf("invalid distribution value: %s", value))
	}

	var ownerReference *metav1.OwnerReference
	for i := range pod.OwnerReferences {
		if pod.OwnerReferences[i].Controller != nil && *pod.OwnerReferences[i].Controller {
			ownerReference = &pod.OwnerReferences[i]
			break
		}
	}
	if ownerReference == nil {
		return nil, framework.NewStatus(framework.Error, "pod has no controller owner reference")
	}

	nodeInfos, err := d.handle.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return nil, framework.NewStatus(framework.Error, fmt.Sprintf("failed to list node infos: %v", err))
	}

	siblingsByNode := make(map[string]int)
	totalPods := 0
	for _, ni := range nodeInfos {
		for _, pi := range ni.Pods {
			p := pi.Pod
			if p.UID == pod.UID {
				continue
			}
			if p.DeletionTimestamp != nil {
				continue
			}
			sibling := false
			for _, r := range p.OwnerReferences {
				if r.UID == ownerReference.UID {
					sibling = true
					break
				}
			}
			if !sibling {
				continue
			}
			siblingsByNode[ni.Node().Name]++
			totalPods++
		}
	}

	state.Write(Name, &stateData{
		distribution:   distribution,
		siblingsByNode: siblingsByNode,
		totalPods:      totalPods,
	})

	return nil, framework.NewStatus(framework.Success)
}

func (d *DistributionScheduler) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (d *DistributionScheduler) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	data, err := readStateData(state)
	if err != nil {
		return framework.NewStatus(framework.Success)
	}

	nodeName := nodeInfo.Node().Name
	siblings := data.siblingsByNode[nodeName]

	totalAfterScheduling := data.totalPods + 1
	maxPerNode := int(math.Ceil(float64(totalAfterScheduling) / float64(data.distribution)))
	if siblings >= maxPerNode {
		return framework.NewStatus(framework.Unschedulable,
			fmt.Sprintf("node %s has %d pods, max allowed %d", nodeName, siblings, maxPerNode))
	}

	return framework.NewStatus(framework.Success)
}

func (d *DistributionScheduler) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	data, err := readStateData(state)
	if err != nil {
		return 0, framework.NewStatus(framework.Success)
	}

	return framework.MaxNodeScore / int64(data.siblingsByNode[nodeName]+1), framework.NewStatus(framework.Success)
}

func (d *DistributionScheduler) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

func readStateData(state *framework.CycleState) (*stateData, error) {
	raw, err := state.Read(Name)
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %v", err)
	}
	s, ok := raw.(*stateData)
	if !ok {
		return nil, fmt.Errorf("invalid state data type")
	}
	return s, nil
}
