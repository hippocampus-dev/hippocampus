package event

import (
	"encoding/json"
	"fmt"
)

const (
	DetectionModeImmediate = "immediate"
	DetectionModeWindowed  = "windowed"
)

type AnomalyEvent struct {
	Grouping      string  `json:"grouping"`
	ErrorHash     string  `json:"error_hash"`
	Count         int     `json:"count"`
	Window        string  `json:"window"`
	DetectionMode string  `json:"detection_mode"`
	ZScore        float64 `json:"z_score,omitempty"`
	Summary       string  `json:"summary"`
	Pod           string  `json:"pod,omitempty"`
}

type Kubernetes struct {
	ContainerName        string            `json:"container_name"`
	NamespaceName        string            `json:"namespace_name"`
	PodName              string            `json:"pod_name"`
	Annotations          map[string]string `json:"annotations"`
	NamespaceAnnotations map[string]string `json:"namespace_annotations"`
}

type LogRecord struct {
	Grouping          string          `json:"grouping"`
	Level             string          `json:"level"`
	Severity          string          `json:"severity"`
	Levelname         string          `json:"levelname"`
	Message           string          `json:"message"`
	StructuralMessage json.RawMessage `json:"structural_message"`
	Kubernetes        *Kubernetes     `json:"kubernetes,omitempty"`
}

func (r LogRecord) ResolvedPod() string {
	// A journald record carries no kubernetes object
	if r.Kubernetes == nil || r.Kubernetes.NamespaceName == "" || r.Kubernetes.PodName == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", r.Kubernetes.NamespaceName, r.Kubernetes.PodName)
}

func (r LogRecord) ResolvedContainer() string {
	if r.Kubernetes == nil {
		return ""
	}
	return r.Kubernetes.ContainerName
}

func (r LogRecord) ResolvedAnnotations() [2]map[string]string {
	if r.Kubernetes == nil {
		return [2]map[string]string{}
	}
	return [2]map[string]string{r.Kubernetes.Annotations, r.Kubernetes.NamespaceAnnotations}
}

func (r LogRecord) ResolvedMessage() string {
	// filter_structural_json in fluentd-aggregator drops message once the line parses as JSON
	if r.Message == "" && len(r.StructuralMessage) > 0 {
		return string(r.StructuralMessage)
	}
	return r.Message
}

func (r LogRecord) ResolvedLevel() string {
	if r.Level != "" {
		return r.Level
	}
	if r.Severity != "" {
		return r.Severity
	}
	if r.Levelname != "" {
		return r.Levelname
	}
	if len(r.StructuralMessage) > 0 {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(r.StructuralMessage, &nested); err == nil {
			for _, key := range []string{"level", "severity", "levelname"} {
				raw, ok := nested[key]
				if !ok {
					continue
				}
				var value string
				if err := json.Unmarshal(raw, &value); err != nil {
					continue
				}
				if value != "" {
					return value
				}
			}
		}
	}
	return ""
}

type AlertmanagerAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}
