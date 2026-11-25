package v1

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RateLimitSpec defines the desired state of RateLimit
type RateLimitSpec struct {
	// WorkloadSelector is a set of labels to match the target workload
	WorkloadSelector map[string]string `json:"workloadSelector"`
	// Filter is the envoy.extensions.filters.http.ratelimit.v3.RateLimit filter config. domain and rate_limit_service are set by the controller, and timeout defaults to 1s.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Filter *runtime.RawExtension `json:"filter,omitempty"`
	// RateLimits is the list of config.route.v3.RateLimit applied to the virtual host
	RateLimits []runtime.RawExtension `json:"rateLimits"`
	// Descriptors is the list of envoyproxy/ratelimit descriptors served under the generated domain
	Descriptors []runtime.RawExtension `json:"descriptors"`
}

// RateLimitStatus defines the observed state of RateLimit
type RateLimitStatus struct {
	// Domain is the generated rate limit domain name
	// +optional
	Domain string `json:"domain,omitempty"`
	// LastSyncedAt is the last time the config was synced
	// +optional
	LastSyncedAt *metaV1.Time `json:"lastSyncedAt,omitempty"`
	// Conditions represent the latest available observations of the RateLimit's state
	// +optional
	Conditions []metaV1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Domain",type=string,JSONPath=`.status.domain`
// +kubebuilder:printcolumn:name="LastSynced",type=date,JSONPath=`.status.lastSyncedAt`

// RateLimit is the Schema for the ratelimits API
type RateLimit struct {
	metaV1.TypeMeta   `json:",inline"`
	metaV1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RateLimitSpec   `json:"spec,omitempty"`
	Status RateLimitStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RateLimitList contains a list of RateLimit
type RateLimitList struct {
	metaV1.TypeMeta `json:",inline"`
	metaV1.ListMeta `json:"metadata,omitempty"`
	Items           []RateLimit `json:"items"`
}
