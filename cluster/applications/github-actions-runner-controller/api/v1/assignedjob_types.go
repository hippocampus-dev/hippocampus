package v1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AssignedJobSpec defines the desired state of AssignedJob
// The listener fills the job identity in when GitHub reports the job started, and the reconciler marks the interruption when the runner's pod is terminated, either of which may arrive first
type AssignedJobSpec struct {
	// Owner specifies the GitHub organization or user owning the repository the job belongs to, which an organization-level scale set takes from the job rather than from the ScaleSet
	// +optional
	Owner string `json:"owner,omitempty"`
	// Repo specifies the repository the job belongs to
	// +optional
	Repo string `json:"repo,omitempty"`
	// WorkflowRunID is the run holding the job, which the cancel and the rerun are addressed to
	// +optional
	WorkflowRunID int64 `json:"workflowRunID,omitempty"`
	// Interrupted marks that the runner container was terminated by SIGTERM while it was running the job
	// +optional
	Interrupted bool `json:"interrupted,omitempty"`
}

// +kubebuilder:object:root=true

// AssignedJob is the schema for the assignedjobs API, and is named after the runner the job was assigned to
type AssignedJob struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec AssignedJobSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// AssignedJobList contains a list of AssignedJob
type AssignedJobList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []AssignedJob `json:"items"`
}
