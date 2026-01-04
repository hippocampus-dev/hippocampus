// Package v1 contains API Schema definitions for the grafana-manifest v1 API group
// +kubebuilder:object:generate=true
// +groupName=grafana-manifest.kaidotio.github.io
package v1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion is a group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "grafana-manifest.kaidotio.github.io", Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(&GrafanaDashboard{}, &GrafanaDashboardList{})
}
