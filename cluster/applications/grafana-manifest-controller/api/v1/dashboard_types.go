package v1

import (
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GrafanaDashboardSpec defines the desired state of GrafanaDashboard
// +kubebuilder:validation:XValidation:rule="has(self.jsonnet) || has(self.configMapRef)",message="one of jsonnet or configMapRef must be specified"
type GrafanaDashboardSpec struct {
	// Jsonnet is the dashboard definition in Jsonnet format
	// +optional
	Jsonnet string `json:"jsonnet,omitempty"`
	// ConfigMapRef is a reference to a ConfigMap containing the dashboard Jsonnet
	// +optional
	ConfigMapRef *coreV1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
	// Folder is the Grafana folder name to place the dashboard in
	// +optional
	Folder string `json:"folder,omitempty"`
}

// GrafanaDashboardStatus defines the observed state of GrafanaDashboard
type GrafanaDashboardStatus struct {
	// UID is the Grafana dashboard UID
	// +optional
	UID string `json:"uid,omitempty"`
	// URL is the Grafana dashboard URL
	// +optional
	URL string `json:"url,omitempty"`
	// Version is the Grafana dashboard version
	// +optional
	Version int `json:"version,omitempty"`
	// LastSyncedAt is the last time the dashboard was synced to Grafana
	// +optional
	LastSyncedAt *metaV1.Time `json:"lastSyncedAt,omitempty"`
	// Conditions represent the latest available observations of the dashboard's state
	// +optional
	Conditions []metaV1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="UID",type=string,JSONPath=`.status.uid`
// +kubebuilder:printcolumn:name="Version",type=integer,JSONPath=`.status.version`
// +kubebuilder:printcolumn:name="LastSynced",type=date,JSONPath=`.status.lastSyncedAt`

// GrafanaDashboard is the Schema for the grafanadashboards API
type GrafanaDashboard struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrafanaDashboardSpec   `json:"spec,omitempty"`
	Status GrafanaDashboardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GrafanaDashboardList contains a list of GrafanaDashboard
type GrafanaDashboardList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []GrafanaDashboard `json:"items"`
}
