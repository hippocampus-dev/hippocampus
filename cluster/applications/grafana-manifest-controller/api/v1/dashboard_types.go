package v1

import (
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DashboardSpec defines the desired state of Dashboard
// +kubebuilder:validation:XValidation:rule="has(self.jsonnet) || has(self.configMapRef)",message="one of jsonnet or configMapRef must be specified"
type DashboardSpec struct {
	// Jsonnet is the dashboard definition in Jsonnet format
	// +optional
	Jsonnet string `json:"jsonnet,omitempty"`
	// ConfigMapRef is a reference to a ConfigMap containing the dashboard Jsonnet
	// +optional
	ConfigMapRef *coreV1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
	// Folder is the Grafana folder name to place the dashboard in
	// +optional
	Folder string `json:"folder,omitempty"`
	// HomeDashboard sets this dashboard as the organization's home dashboard
	// +optional
	HomeDashboard bool `json:"homeDashboard,omitempty"`
}

// DashboardStatus defines the observed state of Dashboard
type DashboardStatus struct {
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

// Dashboard is the Schema for the dashboards API
type Dashboard struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DashboardSpec   `json:"spec,omitempty"`
	Status DashboardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DashboardList contains a list of Dashboard
type DashboardList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []Dashboard `json:"items"`
}
