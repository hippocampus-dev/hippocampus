package registry

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
)

type Parser struct {
	manifestPath string
}

func NewParser(manifestPath string) *Parser {
	return &Parser{manifestPath: manifestPath}
}

func (p *Parser) Parse() (*Registry, error) {
	name := filepath.Base(p.manifestPath)

	namespace, err := p.parseNamespace()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse namespace: %w", err)
	}

	labels, err := p.parseLabels()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse labels: %w", err)
	}

	hasTelemetry := p.hasTelemetry()
	appName := labels["app.kubernetes.io/name"]

	registry := &Registry{
		Name:      name,
		Namespace: namespace,
		Labels:    labels,
	}

	if appName != "" && namespace != "" {
		registry.Logs = &LogsConfig{
			Grouping: fmt.Sprintf("kubernetes.%s.%s", namespace, appName),
		}

		otelServiceName := p.parseEnvValue("OTEL_SERVICE_NAME", appName)
		registry.Traces = &TracesConfig{
			ServiceName: otelServiceName,
		}

		pyroscopeAppName := p.parseEnvValue("PYROSCOPE_APPLICATION_NAME", appName)
		registry.Profiling = &ProfilingConfig{
			ServiceName: pyroscopeAppName,
		}
	}

	if namespace != "" && appName != "" {
		labelSets := []map[string]string{}

		if hasTelemetry {
			labelSets = append(labelSets, map[string]string{
				"destination_service_namespace": namespace,
				"destination_service_name":      appName,
			})
			labelSets = append(labelSets, map[string]string{
				"source_workload_namespace": namespace,
				"source_workload":           appName,
			})
		}

		labelSets = append(labelSets, map[string]string{
			"namespace":              namespace,
			"app_kubernetes_io_name": appName,
		})

		registry.Metrics = &MetricsConfig{
			LabelSets: labelSets,
		}
	}

	return registry, nil
}

func (p *Parser) findFiles(filename string) []string {
	var matches []string
	_ = filepath.WalkDir(p.manifestPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == filename {
			matches = append(matches, path)
		}
		return nil
	})
	sort.Strings(matches)
	return matches
}

func (p *Parser) parseNamespace() (string, error) {
	paths := p.findFiles("kustomization.yaml")

	sort.Slice(paths, func(i, j int) bool {
		iOverlay := strings.Contains(paths[i], "overlays")
		jOverlay := strings.Contains(paths[j], "overlays")
		if iOverlay != jOverlay {
			return iOverlay
		}
		return paths[i] < paths[j]
	})

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var kustomization struct {
			Namespace string `yaml:"namespace"`
		}
		if err := yaml.Unmarshal(data, &kustomization); err != nil {
			continue
		}

		if kustomization.Namespace != "" {
			return kustomization.Namespace, nil
		}
	}

	return filepath.Base(p.manifestPath), nil
}

func (p *Parser) parseLabels() (map[string]string, error) {
	filenames := []string{
		"deployment.yaml",
		"stateful_set.yaml",
		"daemon_set.yaml",
	}

	for _, filename := range filenames {
		paths := p.findFiles(filename)

		sort.Slice(paths, func(i, j int) bool {
			iBase := strings.Contains(paths[i], "/base/")
			jBase := strings.Contains(paths[j], "/base/")
			if iBase != jBase {
				return iBase
			}
			return paths[i] < paths[j]
		})

		for _, path := range paths {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var deployment struct {
				Spec struct {
					Selector struct {
						MatchLabels map[string]string `yaml:"matchLabels"`
					} `yaml:"selector"`
				} `yaml:"spec"`
			}
			if err := yaml.Unmarshal(data, &deployment); err != nil {
				continue
			}

			if len(deployment.Spec.Selector.MatchLabels) > 0 {
				return deployment.Spec.Selector.MatchLabels, nil
			}
		}
	}

	return map[string]string{}, nil
}

func (p *Parser) parseEnvValue(envName string, fallback string) string {
	paths := p.findFiles("deployment.yaml")

	sort.Slice(paths, func(i, j int) bool {
		iBase := strings.Contains(paths[i], "/base/")
		jBase := strings.Contains(paths[j], "/base/")
		if iBase != jBase {
			return iBase
		}
		return paths[i] < paths[j]
	})

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		for {
			var doc map[string]interface{}
			if err := decoder.Decode(&doc); err != nil {
				break
			}

			if value := extractEnvValue(doc, envName); value != "" {
				return value
			}
		}
	}

	return fallback
}

func extractEnvValue(doc map[string]interface{}, envName string) string {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return ""
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return ""
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return ""
	}

	containers, ok := templateSpec["containers"].([]interface{})
	if !ok {
		return ""
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		envs, ok := container["env"].([]interface{})
		if !ok {
			continue
		}

		for _, e := range envs {
			env, ok := e.(map[string]interface{})
			if !ok {
				continue
			}

			if env["name"] == envName {
				if value, ok := env["value"].(string); ok {
					return value
				}
				return ""
			}
		}
	}

	return ""
}

func (p *Parser) hasTelemetry() bool {
	paths := p.findFiles("telemetry.yaml")
	return len(paths) > 0
}

func WriteRegistry(registry *Registry, outputPath string) error {
	data, err := yaml.Marshal(registry)
	if err != nil {
		return xerrors.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return xerrors.Errorf("failed to write registry file: %w", err)
	}

	return nil
}
