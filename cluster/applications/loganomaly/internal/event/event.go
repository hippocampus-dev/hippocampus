package event

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
	Grouping string `json:"grouping"`
	Level    string `json:"level"`
	Message  string `json:"message"`
}

type AlertmanagerAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}
