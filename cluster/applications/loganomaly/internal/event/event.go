package event

import "encoding/json"

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
	BlastRadius   int     `json:"blast_radius"`
	Summary       string  `json:"summary"`
}

type LogRecord struct {
	Grouping          string          `json:"grouping"`
	Level             string          `json:"level"`
	Severity          string          `json:"severity"`
	Levelname         string          `json:"levelname"`
	Message           string          `json:"message"`
	StructuralMessage json.RawMessage `json:"structural_message"`
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
