package v1

import (
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScaleSetSpec defines the desired state of ScaleSet
// Scope is inferred from repo field: if repo is set, scope is repository; otherwise, scope is organization.
type ScaleSetSpec struct {
	// Image using by self-hosted runner
	Image string `json:"image"`
	// Owner specifies the GitHub organization or user
	// +kubebuilder:validation:Pattern=`^[^/]+$`
	Owner string `json:"owner"`
	// Repo specifies the GitHub repository name. If set, the scale set is registered at repository level.
	// If not set, the scale set is registered at organization level.
	// +kubebuilder:validation:Pattern=`^[^/]+$`
	// +optional
	Repo string `json:"repo,omitempty"`
	// RunnerGroup specifies the GitHub runner group the scale set belongs to. The scale set name must be unique within it.
	// Changing it would register a second scale set and leave the first one behind, since GitHub looks a scale set up within one group.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="runnerGroup is immutable"
	// +optional
	RunnerGroup string `json:"runnerGroup,omitempty"`
	// Labels specifies additional targets for runs-on. self-hosted and github-actions-runner-controller are always registered as ones.
	// +kubebuilder:validation:items:Pattern=`^\S+$`
	// +optional
	Labels []string `json:"labels,omitempty"`
	// MinRunners specifies how many idle runners to keep in addition to the runners taken by assigned jobs.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MinRunners int `json:"minRunners,omitempty"`
	// MaxRunners specifies the upper bound of runners, and is reported to GitHub as the capacity of this scale set.
	// +kubebuilder:validation:Minimum=1
	MaxRunners int `json:"maxRunners"`
	// Selects a key of a GitHub Token secret in the scale set's namespace. If not set, the controller's own GitHub App credentials are used.
	TokenSecretKeyRef *v1.SecretKeySelector `json:"tokenSecretKeyRef,omitempty"`
	// +optional
	Template Template `json:"template,omitzero"`
	// +optional
	BuilderContainerSpec BuilderContainerSpec `json:"builderContainerSpec,omitzero"`
	// +optional
	RunnerContainerSpec RunnerContainerSpec `json:"runnerContainerSpec,omitzero"`
}

// ScaleSetStatus defines the observed state of ScaleSet
type ScaleSetStatus struct {
	// ScaleSetID is the identifier GitHub assigned to the registered scale set
	// +optional
	ScaleSetID int `json:"scaleSetID,omitempty"`
	// DesiredRunners is the number of runners to converge to, reported by the listener and reset to min(maxRunners, minRunners) when its session starts
	// +optional
	DesiredRunners int `json:"desiredRunners,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ScaleSet is the schema for the scalesets API
type ScaleSet struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScaleSetSpec   `json:"spec,omitempty"`
	Status ScaleSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ScaleSetList contains a list of ScaleSet
type ScaleSetList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []ScaleSet `json:"items"`
}
