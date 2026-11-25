package controllers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"golang.org/x/xerrors"
	coreV1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
	policyV1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	containerName     = "relocatable-volume-controller"
	requeueInterval   = 2 * time.Second
	transferDirectory = "/volume"
	transferPort      = 9000
)

func annotation(name string) string {
	return fmt.Sprintf("%s/%s", apiGroup(), name)
}

type RelocationController struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	PodNamespace string
	PodName      string
}

func (r *RelocationController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	node := &coreV1.Node{}
	if err := r.Get(ctx, req.NamespacedName, node); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, xerrors.Errorf("failed to get node: %w", err)
	}

	var claimList coreV1.PersistentVolumeClaimList
	if err := r.List(ctx, &claimList); err != nil {
		return ctrl.Result{}, xerrors.Errorf("failed to list persistentvolumeclaims: %w", err)
	}
	var volumeList coreV1.PersistentVolumeList
	if err := r.List(ctx, &volumeList); err != nil {
		return ctrl.Result{}, xerrors.Errorf("failed to list persistentvolumes: %w", err)
	}
	var podList coreV1.PodList
	if err := r.List(ctx, &podList); err != nil {
		return ctrl.Result{}, xerrors.Errorf("failed to list pods: %w", err)
	}

	claims := make(map[string]*coreV1.PersistentVolumeClaim, len(claimList.Items))
	for i := range claimList.Items {
		claim := &claimList.Items[i]
		claims[claimKey(claim.Namespace, claim.Name)] = claim
	}
	volumes := make(map[string]*coreV1.PersistentVolume, len(volumeList.Items))
	committed := make(map[string]bool)
	for i := range volumeList.Items {
		volume := &volumeList.Items[i]
		volumes[volume.Name] = volume
		if key, ok := volume.Annotations[annotation("claim")]; ok {
			committed[key] = true
			if err := r.finishCommit(ctx, volume, key); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	target := node.Annotations[annotation("relocate-to")]
	active := target != "" && ready(node)

	var marked []*coreV1.PersistentVolumeClaim
	for i := range claimList.Items {
		claim := &claimList.Items[i]
		if claim.Annotations[annotation("relocating-from")] != req.Name || committed[claimKey(claim.Namespace, claim.Name)] {
			continue
		}
		if active {
			marked = append(marked, claim)
			continue
		}
		if err := r.deleteTransfer(ctx, claim); err != nil {
			return ctrl.Result{}, err
		}
		patch := client.MergeFrom(claim.DeepCopy())
		delete(claim.Annotations, annotation("relocating-from"))
		if err := r.Patch(ctx, claim, patch); err != nil {
			return ctrl.Result{}, xerrors.Errorf("failed to unmark persistentvolumeclaim: %w", err)
		}
		r.Recorder.Eventf(claim, coreV1.EventTypeWarning, "Aborted", "Relocation from node %s was aborted", req.Name)
	}

	gated := false
	for i := range podList.Items {
		pod := &podList.Items[i]
		if !hasGate(pod) {
			continue
		}
		gated = true
		if !slices.ContainsFunc(claimNames(pod), func(name string) bool {
			claim, ok := claims[claimKey(pod.Namespace, name)]
			if !ok || claim.Status.Phase != coreV1.ClaimBound {
				return true
			}
			_, relocating := claim.Annotations[annotation("relocating-from")]
			return relocating
		}) {
			patch := client.MergeFrom(pod.DeepCopy())
			pod.Spec.SchedulingGates = slices.DeleteFunc(pod.Spec.SchedulingGates, func(gate coreV1.PodSchedulingGate) bool {
				return gate.Name == annotation("relocating")
			})
			if err := r.Patch(ctx, pod, patch); err != nil {
				return ctrl.Result{}, xerrors.Errorf("failed to remove scheduling gate: %w", err)
			}
		}
	}

	if !active {
		if gated || len(committed) > 0 {
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		return ctrl.Result{}, nil
	}

	for _, claim := range marked {
		if err := r.advance(ctx, claim, podList.Items, target); err != nil {
			return ctrl.Result{}, err
		}
	}

	targetNode := &coreV1.Node{}
	if err := r.Get(ctx, types.NamespacedName{Name: target}, targetNode); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		return ctrl.Result{}, xerrors.Errorf("failed to get node: %w", err)
	}

	busy := make(map[string]bool)
	for i := range podList.Items {
		pod := &podList.Items[i]
		names := claimNames(pod)
		if names == nil {
			continue
		}
		if !podReady(pod) || slices.ContainsFunc(names, func(name string) bool {
			claim, ok := claims[claimKey(pod.Namespace, name)]
			if !ok {
				return false
			}
			_, relocating := claim.Annotations[annotation("relocating-from")]
			return relocating
		}) {
			busy[ownerKey(pod)] = true
		}
	}

	settled := len(marked) == 0 && !gated && len(committed) == 0 && len(busy) == 0
	for i := range podList.Items {
		pod := &podList.Items[i]
		located, err := claimsOn(pod, node, claims, volumes)
		if err != nil {
			return ctrl.Result{}, err
		}
		if len(located) == 0 {
			continue
		}
		settled = false
		if busy[ownerKey(pod)] || targetNode.Spec.Unschedulable || !ready(targetNode) {
			continue
		}
		busy[ownerKey(pod)] = true
		if pod.Annotations[annotation("method")] != "transfer" {
			for _, claim := range located {
				if err := r.Delete(ctx, claim, client.Preconditions{UID: &claim.UID}); client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, xerrors.Errorf("failed to delete persistentvolumeclaim: %w", err)
				}
			}
			if err := r.evictPod(ctx, pod); err != nil {
				return ctrl.Result{}, err
			}
			continue
		}
		for _, claim := range located {
			patch := client.MergeFrom(claim.DeepCopy())
			if claim.Annotations == nil {
				claim.Annotations = make(map[string]string)
			}
			claim.Annotations[annotation("relocating-from")] = req.Name
			if err := r.Patch(ctx, claim, patch); err != nil {
				return ctrl.Result{}, xerrors.Errorf("failed to mark persistentvolumeclaim: %w", err)
			}
		}
	}
	if !settled {
		return ctrl.Result{RequeueAfter: requeueInterval}, nil
	}

	patch := client.MergeFrom(node.DeepCopy())
	delete(node.Annotations, annotation("relocate-to"))
	if err := r.Patch(ctx, node, patch); err != nil {
		return ctrl.Result{}, xerrors.Errorf("failed to remove annotation from node: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *RelocationController) advance(ctx context.Context, claim *coreV1.PersistentVolumeClaim, pods []coreV1.Pod, target string) error {
	var workload *coreV1.Pod
	for i := range pods {
		pod := &pods[i]
		if pod.Namespace != claim.Namespace || !slices.Contains(claimNames(pod), claim.Name) {
			continue
		}
		if pod.Spec.NodeName != "" || !hasGate(pod) {
			if pod.DeletionTimestamp == nil {
				return r.evictPod(ctx, pod)
			}
			return nil
		}
		workload = pod
	}
	if workload == nil {
		return nil
	}

	receiver := &coreV1.Pod{}
	if err := r.Get(ctx, transferKey(claim, "receiver"), receiver); err != nil {
		if !apierrors.IsNotFound(err) {
			return xerrors.Errorf("failed to get receiver: %w", err)
		}
		return r.createReceiver(ctx, claim, workload, target)
	}
	if receiver.Status.PodIP == "" {
		return nil
	}

	sender := &coreV1.Pod{}
	if err := r.Get(ctx, transferKey(claim, "sender"), sender); err != nil {
		if !apierrors.IsNotFound(err) {
			return xerrors.Errorf("failed to get sender: %w", err)
		}
		pod, err := r.transferPod(ctx, claim, workload, claim.Name, "sender", "--send-address", fmt.Sprintf("%s:%d", receiver.Status.PodIP, transferPort))
		if err != nil {
			return err
		}
		pod.Spec.NodeSelector = map[string]string{coreV1.LabelHostname: claim.Annotations[annotation("relocating-from")]}
		if err := r.Create(ctx, pod); client.IgnoreAlreadyExists(err) != nil {
			return xerrors.Errorf("failed to create sender: %w", err)
		}
		return nil
	}

	if receiver.Status.Phase == coreV1.PodFailed || sender.Status.Phase == coreV1.PodFailed {
		r.Recorder.Eventf(claim, coreV1.EventTypeWarning, "Retrying", "Transfer to node %s failed, so it starts over", target)
		return r.deleteTransfer(ctx, claim)
	}
	if receiver.Status.Phase != coreV1.PodSucceeded || sender.Status.Phase != coreV1.PodSucceeded {
		return nil
	}

	staging := &coreV1.PersistentVolumeClaim{}
	if err := r.Get(ctx, transferKey(claim, ""), staging); err != nil {
		return xerrors.Errorf("failed to get staging persistentvolumeclaim: %w", err)
	}
	volume := &coreV1.PersistentVolume{}
	if err := r.Get(ctx, types.NamespacedName{Name: staging.Spec.VolumeName}, volume); err != nil {
		return xerrors.Errorf("failed to get staging persistentvolume: %w", err)
	}

	patch := client.MergeFrom(volume.DeepCopy())
	if volume.Annotations == nil {
		volume.Annotations = make(map[string]string)
	}
	volume.Annotations[annotation("claim")] = claimKey(claim.Namespace, claim.Name)
	volume.Annotations[annotation("reclaim-policy")] = string(volume.Spec.PersistentVolumeReclaimPolicy)
	volume.Spec.PersistentVolumeReclaimPolicy = coreV1.PersistentVolumeReclaimRetain
	if err := r.Patch(ctx, volume, patch); err != nil {
		return xerrors.Errorf("failed to commit persistentvolume: %w", err)
	}
	return nil
}

func (r *RelocationController) createReceiver(ctx context.Context, claim *coreV1.PersistentVolumeClaim, workload *coreV1.Pod, target string) error {
	protocol := coreV1.ProtocolTCP
	port := intstr.FromInt32(transferPort)
	policy := &networkingV1.NetworkPolicy{
		ObjectMeta: metaV1.ObjectMeta{Name: transferKey(claim, "").Name, Namespace: claim.Namespace},
		Spec: networkingV1.NetworkPolicySpec{
			PodSelector: metaV1.LabelSelector{MatchLabels: transferLabels(claim, "receiver")},
			PolicyTypes: []networkingV1.PolicyType{networkingV1.PolicyTypeIngress},
			Ingress: []networkingV1.NetworkPolicyIngressRule{{
				From:  []networkingV1.NetworkPolicyPeer{{PodSelector: &metaV1.LabelSelector{MatchLabels: transferLabels(claim, "sender")}}},
				Ports: []networkingV1.NetworkPolicyPort{{Protocol: &protocol, Port: &port}},
			}},
		},
	}
	if err := r.Create(ctx, policy); client.IgnoreAlreadyExists(err) != nil {
		return xerrors.Errorf("failed to create networkpolicy: %w", err)
	}

	staging := &coreV1.PersistentVolumeClaim{
		ObjectMeta: metaV1.ObjectMeta{Name: transferKey(claim, "").Name, Namespace: claim.Namespace},
		Spec: coreV1.PersistentVolumeClaimSpec{
			AccessModes: claim.Spec.AccessModes,
			Resources:   claim.Spec.Resources,
		},
	}
	if err := r.Create(ctx, staging); client.IgnoreAlreadyExists(err) != nil {
		return xerrors.Errorf("failed to create staging persistentvolumeclaim: %w", err)
	}

	pod, err := r.transferPod(ctx, claim, workload, staging.Name, "receiver", "--receive-address", fmt.Sprintf(":%d", transferPort))
	if err != nil {
		return err
	}
	pod.Spec.NodeSelector = map[string]string{coreV1.LabelHostname: target}
	if err := r.Create(ctx, pod); client.IgnoreAlreadyExists(err) != nil {
		return xerrors.Errorf("failed to create receiver: %w", err)
	}
	return nil
}

func (r *RelocationController) transferPod(ctx context.Context, claim *coreV1.PersistentVolumeClaim, workload *coreV1.Pod, claimName string, role string, flag string, address string) (*coreV1.Pod, error) {
	self := &coreV1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: r.PodNamespace, Name: r.PodName}, self); err != nil {
		return nil, xerrors.Errorf("failed to get own pod: %w", err)
	}
	selfContainer := slices.IndexFunc(self.Spec.Containers, func(container coreV1.Container) bool {
		return container.Name == containerName
	})
	if selfContainer < 0 {
		return nil, xerrors.Errorf("pod %s has no container %s", r.PodName, containerName)
	}

	// Files in the volume belong to the user of the container mounting it, and a restricted namespace rejects root
	volumeName := workload.Spec.Volumes[slices.IndexFunc(workload.Spec.Volumes, func(volume coreV1.Volume) bool {
		return volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == claim.Name
	})].Name
	var securityContext *coreV1.SecurityContext
	for _, container := range workload.Spec.Containers {
		if slices.ContainsFunc(container.VolumeMounts, func(mount coreV1.VolumeMount) bool { return mount.Name == volumeName }) {
			securityContext = container.SecurityContext
			break
		}
	}

	return &coreV1.Pod{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      transferKey(claim, role).Name,
			Namespace: claim.Namespace,
			Labels:    transferLabels(claim, role),
		},
		Spec: coreV1.PodSpec{
			RestartPolicy:                coreV1.RestartPolicyNever,
			AutomountServiceAccountToken: new(false),
			SecurityContext:              workload.Spec.SecurityContext,
			// The source node is cordoned
			Tolerations: []coreV1.Toleration{{Operator: coreV1.TolerationOpExists}},
			Containers: []coreV1.Container{{
				Name:            "transfer",
				Image:           self.Spec.Containers[selfContainer].Image,
				ImagePullPolicy: coreV1.PullIfNotPresent,
				Args:            []string{flag, address, "--transfer-directory", transferDirectory},
				SecurityContext: securityContext,
				VolumeMounts:    []coreV1.VolumeMount{{Name: "volume", MountPath: transferDirectory}},
			}},
			Volumes: []coreV1.Volume{{
				Name: "volume",
				VolumeSource: coreV1.VolumeSource{
					PersistentVolumeClaim: &coreV1.PersistentVolumeClaimVolumeSource{ClaimName: claimName},
				},
			}},
		},
	}, nil
}

func (r *RelocationController) finishCommit(ctx context.Context, volume *coreV1.PersistentVolume, key string) error {
	namespace, name, _ := strings.Cut(key, "/")

	if volume.Spec.ClaimRef != nil && volume.Spec.ClaimRef.Name != name {
		staging := &coreV1.PersistentVolumeClaim{ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: volume.Spec.ClaimRef.Name}}
		for _, role := range []string{"sender", "receiver"} {
			pod := &coreV1.Pod{ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: fmt.Sprintf("%s-%s", staging.Name, role)}}
			if err := r.Delete(ctx, pod); client.IgnoreNotFound(err) != nil {
				return xerrors.Errorf("failed to delete transfer pod: %w", err)
			}
		}
		policy := &networkingV1.NetworkPolicy{ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: staging.Name}}
		if err := r.Delete(ctx, policy); client.IgnoreNotFound(err) != nil {
			return xerrors.Errorf("failed to delete networkpolicy: %w", err)
		}
		if err := r.Delete(ctx, staging); err != nil {
			if !apierrors.IsNotFound(err) {
				return xerrors.Errorf("failed to delete staging persistentvolumeclaim: %w", err)
			}
			patch := client.MergeFrom(volume.DeepCopy())
			volume.Spec.ClaimRef = &coreV1.ObjectReference{Namespace: namespace, Name: name}
			if err := r.Patch(ctx, volume, patch); err != nil {
				return xerrors.Errorf("failed to reserve persistentvolume: %w", err)
			}
		}
		return nil
	}

	claim := &coreV1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, claim); err != nil {
		if !apierrors.IsNotFound(err) {
			return xerrors.Errorf("failed to get persistentvolumeclaim: %w", err)
		}
		claim = &coreV1.PersistentVolumeClaim{
			ObjectMeta: metaV1.ObjectMeta{Namespace: namespace, Name: name},
			Spec: coreV1.PersistentVolumeClaimSpec{
				AccessModes:      volume.Spec.AccessModes,
				Resources:        coreV1.VolumeResourceRequirements{Requests: volume.Spec.Capacity},
				StorageClassName: &volume.Spec.StorageClassName,
				VolumeName:       volume.Name,
			},
		}
		if err := r.Create(ctx, claim); client.IgnoreAlreadyExists(err) != nil {
			return xerrors.Errorf("failed to create persistentvolumeclaim: %w", err)
		}
		return nil
	}
	if _, ok := claim.Annotations[annotation("relocating-from")]; ok {
		if err := r.Delete(ctx, claim, client.Preconditions{UID: &claim.UID}); client.IgnoreNotFound(err) != nil {
			return xerrors.Errorf("failed to delete persistentvolumeclaim: %w", err)
		}
		return nil
	}
	if claim.Spec.VolumeName != volume.Name || claim.Status.Phase != coreV1.ClaimBound {
		return nil
	}

	patch := client.MergeFrom(volume.DeepCopy())
	volume.Spec.PersistentVolumeReclaimPolicy = coreV1.PersistentVolumeReclaimPolicy(volume.Annotations[annotation("reclaim-policy")])
	delete(volume.Annotations, annotation("claim"))
	delete(volume.Annotations, annotation("reclaim-policy"))
	if err := r.Patch(ctx, volume, patch); err != nil {
		return xerrors.Errorf("failed to finish persistentvolume: %w", err)
	}
	r.Recorder.Eventf(claim, coreV1.EventTypeNormal, "SuccessfulRelocated", "Relocated to persistentvolume %s", volume.Name)
	return nil
}

func (r *RelocationController) deleteTransfer(ctx context.Context, claim *coreV1.PersistentVolumeClaim) error {
	for _, role := range []string{"sender", "receiver"} {
		key := transferKey(claim, role)
		pod := &coreV1.Pod{ObjectMeta: metaV1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
		// The sender never reports back from a node that left
		if err := r.Delete(ctx, pod, client.GracePeriodSeconds(0)); client.IgnoreNotFound(err) != nil {
			return xerrors.Errorf("failed to delete transfer pod: %w", err)
		}
	}

	key := transferKey(claim, "")
	policy := &networkingV1.NetworkPolicy{ObjectMeta: metaV1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
	if err := r.Delete(ctx, policy); client.IgnoreNotFound(err) != nil {
		return xerrors.Errorf("failed to delete networkpolicy: %w", err)
	}
	staging := &coreV1.PersistentVolumeClaim{ObjectMeta: metaV1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
	if err := r.Delete(ctx, staging); client.IgnoreNotFound(err) != nil {
		return xerrors.Errorf("failed to delete staging persistentvolumeclaim: %w", err)
	}
	return nil
}

func (r *RelocationController) evictPod(ctx context.Context, pod *coreV1.Pod) error {
	eviction := &policyV1.Eviction{ObjectMeta: metaV1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name}}
	if err := r.SubResource("eviction").Create(ctx, pod, eviction); client.IgnoreNotFound(err) != nil && !apierrors.IsTooManyRequests(err) {
		return xerrors.Errorf("failed to evict pod: %w", err)
	}
	return nil
}

func claimKey(namespace string, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func transferKey(claim *coreV1.PersistentVolumeClaim, role string) types.NamespacedName {
	name := fmt.Sprintf("relocation-%s", claim.UID)
	if role != "" {
		name = fmt.Sprintf("%s-%s", name, role)
	}
	return types.NamespacedName{Namespace: claim.Namespace, Name: name}
}

func transferLabels(claim *coreV1.PersistentVolumeClaim, role string) map[string]string {
	return map[string]string{
		annotation("transfer"): string(claim.UID),
		annotation("role"):     role,
	}
}

func ownerKey(pod *coreV1.Pod) string {
	if owner := metaV1.GetControllerOf(pod); owner != nil {
		return string(owner.UID)
	}
	return string(pod.UID)
}

func hasGate(pod *coreV1.Pod) bool {
	return slices.ContainsFunc(pod.Spec.SchedulingGates, func(gate coreV1.PodSchedulingGate) bool {
		return gate.Name == annotation("relocating")
	})
}

func podReady(pod *coreV1.Pod) bool {
	return slices.ContainsFunc(pod.Status.Conditions, func(condition coreV1.PodCondition) bool {
		return condition.Type == coreV1.PodReady && condition.Status == coreV1.ConditionTrue
	})
}

func claimNames(pod *coreV1.Pod) []string {
	value, ok := pod.Annotations[annotation("volumes")]
	if !ok {
		return nil
	}

	relocatableVolumes := strings.Split(value, ",")
	var names []string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && slices.Contains(relocatableVolumes, volume.Name) {
			names = append(names, volume.PersistentVolumeClaim.ClaimName)
		}
	}
	return names
}

func claimsOn(pod *coreV1.Pod, node *coreV1.Node, claims map[string]*coreV1.PersistentVolumeClaim, volumes map[string]*coreV1.PersistentVolume) ([]*coreV1.PersistentVolumeClaim, error) {
	var located []*coreV1.PersistentVolumeClaim
	for _, name := range claimNames(pod) {
		claim, ok := claims[claimKey(pod.Namespace, name)]
		if !ok {
			continue
		}
		volume, ok := volumes[claim.Spec.VolumeName]
		if !ok {
			continue
		}
		if volume.Spec.NodeAffinity == nil || volume.Spec.NodeAffinity.Required == nil {
			if pod.Annotations[annotation("method")] == "transfer" && pod.Spec.NodeName == node.Name {
				located = append(located, claim)
			}
			continue
		}
		matched, err := corev1helpers.MatchNodeSelectorTerms(node, volume.Spec.NodeAffinity.Required)
		if err != nil {
			return nil, xerrors.Errorf("failed to match node selector terms: %w", err)
		}
		if matched {
			located = append(located, claim)
		}
	}
	return located, nil
}

func (r *RelocationController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("relocation").
		For(&coreV1.Node{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
