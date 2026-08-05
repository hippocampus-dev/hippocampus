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
	Grouping             string  `json:"grouping"`
	ErrorHash            string  `json:"error_hash"`
	Count                int     `json:"count"`
	Window               string  `json:"window"`
	DetectionMode        string  `json:"detection_mode"`
	ZScore               float64 `json:"z_score,omitempty"`
	ActiveErrorGroupings int     `json:"active_error_groupings"`
	Summary              string  `json:"summary"`
	Pod                  string  `json:"pod,omitempty"`
}

type Kubernetes struct {
	NamespaceName string `json:"namespace_name"`
	PodName       string `json:"pod_name"`
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
	// A journald record carries no kubernetes object, so the pod is only known for container logs.
	if r.Kubernetes == nil || r.Kubernetes.NamespaceName == "" || r.Kubernetes.PodName == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", r.Kubernetes.NamespaceName, r.Kubernetes.PodName)
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
