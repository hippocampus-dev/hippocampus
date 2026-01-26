package registry

type Registry struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels"`
	Logs      *LogsConfig       `yaml:"logs,omitempty"`
	Traces    *TracesConfig     `yaml:"traces,omitempty"`
	Profiling *ProfilingConfig  `yaml:"profiling,omitempty"`
	Metrics   *MetricsConfig    `yaml:"metrics,omitempty"`
}

type LogsConfig struct {
	Grouping string `yaml:"grouping"`
}

type TracesConfig struct {
	ServiceName string `yaml:"serviceName"`
}

type ProfilingConfig struct {
	ServiceName string `yaml:"serviceName"`
}

type MetricsConfig struct {
	LabelSets []map[string]string `yaml:"labelSets"`
}
