package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DedicatedContainerIngressSpec defines the desired state of DedicatedContainerIngress
type DedicatedContainerIngressSpec struct {
	// Host is the hostname that triggers dedicated container provisioning
	Host string `json:"host"`
	// Template describes the pod template for creating dedicated containers
	Template corev1.PodTemplateSpec `json:"template"`
}

// DedicatedContainerIngressStatus defines the observed state of DedicatedContainerIngress
type DedicatedContainerIngressStatus struct{}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DedicatedContainerIngress is the schema for the dedicatedcontaineringresses API
type DedicatedContainerIngress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DedicatedContainerIngressSpec   `json:"spec,omitempty"`
	Status DedicatedContainerIngressStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DedicatedContainerIngressList contains a list of DedicatedContainerIngress
type DedicatedContainerIngressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DedicatedContainerIngress `json:"items"`
}
