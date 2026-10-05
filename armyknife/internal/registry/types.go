package registry

type Registry struct {
	Name       string      `yaml:"name"`
	Namespace  string      `yaml:"namespace"`
	Components []Component `yaml:"components"`
}

type Component struct {
	Name         string            `yaml:"name"`
	WorkloadType string            `yaml:"workloadType,omitempty"`
	Labels       map[string]string `yaml:"labels"`
	Logs         *LogsConfig       `yaml:"logs,omitempty"`
	Traces       *TracesConfig     `yaml:"traces,omitempty"`
	Profiling    *ProfilingConfig  `yaml:"profiling,omitempty"`
	Metrics      *MetricsConfig    `yaml:"metrics,omitempty"`
}

type LogsConfig struct {
	Grouping string `yaml:"grouping"`
	Link     string `yaml:"link,omitempty"`
}

type TracesConfig struct {
	ServiceName string `yaml:"serviceName"`
	Link        string `yaml:"link,omitempty"`
}

type ProfilingConfig struct {
	ServiceName string `yaml:"serviceName"`
	Link        string `yaml:"link,omitempty"`
}

type MetricsConfig struct {
	Link      string     `yaml:"link,omitempty"`
	LabelSets []LabelSet `yaml:"labelSets"`
}

type LabelSet struct {
	Labels  map[string]string `yaml:"labels"`
	Queries []QueryTemplate   `yaml:"queries,omitempty"`
}

type QueryTemplate struct {
	Name     string `yaml:"name"`
	Template string `yaml:"template"`
}
