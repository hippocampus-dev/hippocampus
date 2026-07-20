package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
)

type GrafanaConfig struct {
	GrafanaURL             string
	LokiDatasourceUID      string
	TempoDatasourceUID     string
	PyroscopeDatasourceUID string
}

type Parser struct {
	manifestPath  string
	grafanaConfig *GrafanaConfig
}

func NewParser(manifestPath string, grafanaConfig *GrafanaConfig) *Parser {
	return &Parser{manifestPath: manifestPath, grafanaConfig: grafanaConfig}
}

type workloadInfo struct {
	name         string
	workloadType string
	matchLabels  map[string]string
	envs         map[string]string
}

func (p *Parser) Parse() (*Registry, error) {
	name := filepath.Base(p.manifestPath)

	namespace, err := p.parseNamespace()
	if err != nil {
		return nil, xerrors.Errorf("failed to parse namespace: %w", err)
	}

	workloads := p.parseWorkloads()
	hasTelemetry := p.hasTelemetry()
	var components []Component
	for _, w := range workloads {
		nameLabel := w.matchLabels["app.kubernetes.io/name"]
		componentLabel := w.matchLabels["app.kubernetes.io/component"]
		identifier := nameLabel
		if identifier == "" {
			identifier = componentLabel
		}
		component := Component{
			Name:         w.name,
			WorkloadType: w.workloadType,
			Labels:       w.matchLabels,
		}

		if identifier != "" && namespace != "" {
			groupingName := logsGroupingName(w.matchLabels)
			if groupingName == "" {
				groupingName = w.name
			}
			component.Logs = &LogsConfig{
				Grouping: fmt.Sprintf("kubernetes.%s.%s", namespace, groupingName),
			}

			if otelServiceName := w.envs["OTEL_SERVICE_NAME"]; otelServiceName != "" {
				if _, ok := w.envs["OTEL_EXPORTER_OTLP_ENDPOINT"]; ok {
					component.Traces = &TracesConfig{
						ServiceName: otelServiceName,
					}

					if p.grafanaConfig != nil && p.grafanaConfig.GrafanaURL != "" && p.grafanaConfig.TempoDatasourceUID != "" {
						link, err := buildTracesExploreURL(
							p.grafanaConfig.GrafanaURL,
							p.grafanaConfig.TempoDatasourceUID,
							otelServiceName,
						)
						if err != nil {
							return nil, xerrors.Errorf("failed to build traces explore URL: %w", err)
						}
						component.Traces.Link = link
					}
				}
			}

			if _, ok := w.envs["PYROSCOPE_ENDPOINT"]; ok {
				pyroscopeAppName := w.envs["PYROSCOPE_APPLICATION_NAME"]
				if pyroscopeAppName == "" {
					pyroscopeAppName = identifier
				}
				component.Profiling = &ProfilingConfig{
					ServiceName: pyroscopeAppName,
				}

				if p.grafanaConfig != nil && p.grafanaConfig.GrafanaURL != "" && p.grafanaConfig.PyroscopeDatasourceUID != "" {
					link, err := buildProfilingExploreURL(
						p.grafanaConfig.GrafanaURL,
						p.grafanaConfig.PyroscopeDatasourceUID,
						pyroscopeAppName,
					)
					if err != nil {
						return nil, xerrors.Errorf("failed to build profiling explore URL: %w", err)
					}
					component.Profiling.Link = link
				}
			}

			if p.grafanaConfig != nil && p.grafanaConfig.GrafanaURL != "" && p.grafanaConfig.LokiDatasourceUID != "" {
				link, err := buildExploreURL(
					p.grafanaConfig.GrafanaURL,
					p.grafanaConfig.LokiDatasourceUID,
					fmt.Sprintf(`{grouping="kubernetes.%s.%s"}`, namespace, groupingName),
				)
				if err != nil {
					return nil, xerrors.Errorf("failed to build logs explore URL: %w", err)
				}
				component.Logs.Link = link
			}

			var labelSets []LabelSet

			if hasTelemetry {
				labelSets = append(labelSets, LabelSet{
					Labels: map[string]string{
						"destination_service_namespace": namespace,
						"destination_service_name":      w.name,
					},
					Queries: destinationServiceQueries(),
				})
				labelSets = append(labelSets, LabelSet{
					Labels: map[string]string{
						"source_workload_namespace": namespace,
						"source_workload":           w.name,
					},
					Queries: sourceWorkloadQueries(),
				})
			}

			if w.workloadType != "" {
				labelSets = append(labelSets, LabelSet{
					Labels: map[string]string{
						"namespace":     namespace,
						"workload":      w.name,
						"workload_type": w.workloadType,
					},
					Queries: containerResourceQueries(),
				})
			}
			kubernetesLabels := map[string]string{
				"namespace": namespace,
			}
			if nameLabel != "" {
				kubernetesLabels["app_kubernetes_io_name"] = nameLabel
			}
			if _, ok := w.matchLabels["app.kubernetes.io/component"]; ok {
				kubernetesLabels["app_kubernetes_io_component"] = componentLabel
			}
			labelSets = append(labelSets, LabelSet{
				Labels: kubernetesLabels,
			})

			var metricsLink string
			if w.workloadType != "" && p.grafanaConfig != nil && p.grafanaConfig.GrafanaURL != "" {
				metricsLink = buildWorkloadDashboardURL(p.grafanaConfig.GrafanaURL, namespace, w.workloadType, w.name)
			}

			component.Metrics = &MetricsConfig{
				Link:      metricsLink,
				LabelSets: labelSets,
			}
		}

		components = append(components, component)
	}

	return &Registry{
		Name:       name,
		Namespace:  namespace,
		Components: components,
	}, nil
}

func (p *Parser) findFiles(filename string) []string {
	return findFilesInDirectory(p.manifestPath, filename)
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

func (p *Parser) parseWorkloads() []workloadInfo {
	filenames := []string{
		"deployment.yaml",
		"stateful_set.yaml",
		"daemon_set.yaml",
	}

	resolved := p.resolveResourceDirectories()
	directories := []resolvedDirectory{{path: p.manifestPath}}
	directories = append(directories, resolved...)

	var workloads []workloadInfo

	var allPatchDocs []map[string]interface{}
	for _, filename := range filenames {
		for _, directory := range directories {
			for _, path := range findFilesInDirectory(directory.path, filename) {
				if !strings.Contains(path, "/patches/") {
					continue
				}
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				decoder := yaml.NewDecoder(bytes.NewReader(data))
				for {
					var doc map[string]interface{}
					if err := decoder.Decode(&doc); err != nil {
						break
					}
					allPatchDocs = append(allPatchDocs, doc)
				}
			}
		}
	}

	for _, filename := range filenames {
		for _, directory := range directories {
			for _, path := range findFilesInDirectory(directory.path, filename) {
				if strings.Contains(path, "/patches/") {
					continue
				}

				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}

				decoder := yaml.NewDecoder(bytes.NewReader(data))
				for {
					var doc map[string]interface{}
					if err := decoder.Decode(&doc); err != nil {
						break
					}

					if info := extractWorkloadInfo(doc); info != nil {
						info.workloadType = workloadTypeFromKind(doc)
						for _, patch := range allPatchDocs {
							mergeEnvsFromPatch(info, patch, directory.labels)
						}
						applyKustomizationMetadata(info, directory)
						workloads = append(workloads, *info)
					}
				}
			}
		}
	}

	for _, directory := range directories {
		paths := findFilesInDirectory(directory.path, "service.yaml")

		for _, path := range paths {
			if strings.Contains(path, "/patches/") {
				continue
			}

			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			decoder := yaml.NewDecoder(bytes.NewReader(data))
			for {
				var doc map[string]interface{}
				if err := decoder.Decode(&doc); err != nil {
					break
				}

				if info := extractKnativeServiceInfo(doc); info != nil {
					applyKustomizationMetadata(info, directory)
					workloads = append(workloads, *info)
				}
			}
		}
	}

	return workloads
}

func applyKustomizationMetadata(info *workloadInfo, directory resolvedDirectory) {
	if directory.namePrefix != "" {
		info.name = directory.namePrefix + info.name
	}
	for k, v := range directory.labels {
		info.matchLabels[k] = v
	}
}

func logsGroupingName(labels map[string]string) string {
	name := labels["app.kubernetes.io/name"]
	if name == "" {
		name = labels["k8s-app"]
	}
	if name == "" {
		name = labels["app"]
	}
	if name == "" {
		name = labels["component"]
	}
	if name != "" {
		if component := labels["app.kubernetes.io/component"]; component != "" {
			name = name + "-" + component
		}
	}
	return name
}

func findFilesInDirectory(directory string, filename string) []string {
	var matches []string
	_ = filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
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

type resolvedDirectory struct {
	path       string
	namePrefix string
	labels     map[string]string
}

func (p *Parser) resolveResourceDirectories() []resolvedDirectory {
	kustomizationPaths := p.findFiles("kustomization.yaml")

	var directories []resolvedDirectory
	for _, path := range kustomizationPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var kustomization struct {
			Resources  []string `yaml:"resources"`
			NamePrefix string   `yaml:"namePrefix"`
			Labels     []struct {
				Pairs map[string]string `yaml:"pairs"`
			} `yaml:"labels"`
		}
		if err := yaml.Unmarshal(data, &kustomization); err != nil {
			continue
		}

		labels := make(map[string]string)
		for _, entry := range kustomization.Labels {
			for k, v := range entry.Pairs {
				labels[k] = v
			}
		}

		dir := filepath.Dir(path)
		for _, resource := range kustomization.Resources {
			if strings.HasSuffix(resource, ".yaml") || strings.HasSuffix(resource, ".yml") {
				continue
			}

			resolved := filepath.Join(dir, resource)
			resolved = filepath.Clean(resolved)

			info, err := os.Stat(resolved)
			if err != nil || !info.IsDir() {
				continue
			}

			if resolved == p.manifestPath || strings.HasPrefix(resolved, p.manifestPath+string(filepath.Separator)) {
				continue
			}

			directories = append(directories, resolvedDirectory{
				path:       resolved,
				namePrefix: kustomization.NamePrefix,
				labels:     labels,
			})
		}
	}

	return directories
}

func extractWorkloadInfo(doc map[string]interface{}) *workloadInfo {
	metadata, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return nil
	}

	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	selector, ok := spec["selector"].(map[string]interface{})
	if !ok {
		return nil
	}

	matchLabelsRaw, ok := selector["matchLabels"].(map[string]interface{})
	if !ok || len(matchLabelsRaw) == 0 {
		return nil
	}

	matchLabels := make(map[string]string, len(matchLabelsRaw))
	for k, v := range matchLabelsRaw {
		if s, ok := v.(string); ok {
			matchLabels[k] = s
		}
	}

	envs := make(map[string]string)
	for _, envName := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_NAME", "PYROSCOPE_ENDPOINT", "PYROSCOPE_APPLICATION_NAME"} {
		if value, found := extractEnvValue(doc, envName, matchLabels); found {
			envs[envName] = value
		}
	}

	return &workloadInfo{
		name:        name,
		matchLabels: matchLabels,
		envs:        envs,
	}
}

func extractKnativeServiceInfo(doc map[string]interface{}) *workloadInfo {
	apiVersion, _ := doc["apiVersion"].(string)
	if apiVersion != "serving.knative.dev/v1" {
		return nil
	}

	kind, _ := doc["kind"].(string)
	if kind != "Service" {
		return nil
	}

	metadata, ok := doc["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return nil
	}

	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return nil
	}

	templateMetadata, ok := template["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}

	labelsRaw, ok := templateMetadata["labels"].(map[string]interface{})
	if !ok || len(labelsRaw) == 0 {
		return nil
	}

	labels := make(map[string]string, len(labelsRaw))
	for k, v := range labelsRaw {
		if s, ok := v.(string); ok {
			labels[k] = s
		}
	}

	envs := make(map[string]string)
	for _, envName := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_NAME", "PYROSCOPE_ENDPOINT", "PYROSCOPE_APPLICATION_NAME"} {
		if value, found := extractEnvValue(doc, envName, labels); found {
			envs[envName] = value
		}
	}

	return &workloadInfo{
		name:        name,
		matchLabels: labels,
		envs:        envs,
	}
}

func mergeEnvsFromPatch(info *workloadInfo, patch map[string]interface{}, directoryLabels map[string]string) {
	metadata, _ := patch["metadata"].(map[string]interface{})
	if metadata == nil {
		return
	}
	name, _ := metadata["name"].(string)
	if name != info.name {
		return
	}

	mergedLabels := make(map[string]string, len(info.matchLabels)+len(directoryLabels))
	for k, v := range info.matchLabels {
		mergedLabels[k] = v
	}
	for k, v := range directoryLabels {
		mergedLabels[k] = v
	}

	targetEnvNames := []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_NAME", "PYROSCOPE_ENDPOINT", "PYROSCOPE_APPLICATION_NAME"}
	for _, envName := range targetEnvNames {
		if _, exists := info.envs[envName]; exists {
			continue
		}
		if value, found := extractEnvValue(patch, envName, mergedLabels); found {
			info.envs[envName] = value
		}
	}
}

func extractEnvValue(doc map[string]interface{}, envName string, labels map[string]string) (string, bool) {
	spec, ok := doc["spec"].(map[string]interface{})
	if !ok {
		return "", false
	}

	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return "", false
	}

	templateSpec, ok := template["spec"].(map[string]interface{})
	if !ok {
		return "", false
	}

	containers, ok := templateSpec["containers"].([]interface{})
	if !ok {
		return "", false
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
					return value, true
				}
				if valueFrom, ok := env["valueFrom"].(map[string]interface{}); ok {
					if fieldRef, ok := valueFrom["fieldRef"].(map[string]interface{}); ok {
						if fieldPath, ok := fieldRef["fieldPath"].(string); ok {
							if strings.HasPrefix(fieldPath, "metadata.labels['") && strings.HasSuffix(fieldPath, "']") {
								labelKey := fieldPath[len("metadata.labels['") : len(fieldPath)-len("']")]
								if v, ok := labels[labelKey]; ok {
									return v, true
								}
							}
						}
					}
				}
				return "", true
			}
		}
	}

	return "", false
}

func buildExploreURL(baseURL string, datasourceUID string, query string) (string, error) {
	panes := map[string]interface{}{
		"default": map[string]interface{}{
			"datasource": datasourceUID,
			"queries": []map[string]interface{}{
				{"expr": query},
			},
		},
	}
	panesJSON, err := json.Marshal(panes)
	if err != nil {
		return "", xerrors.Errorf("failed to marshal panes: %w", err)
	}
	return fmt.Sprintf("%s/explore?schemaVersion=1&panes=%s", baseURL, url.QueryEscape(string(panesJSON))), nil
}

func buildTracesExploreURL(baseURL string, datasourceUID string, serviceName string) (string, error) {
	panes := map[string]interface{}{
		"default": map[string]interface{}{
			"datasource": datasourceUID,
			"queries": []map[string]interface{}{
				{
					"query":     fmt.Sprintf(`{resource.service.name="%s"}`, serviceName),
					"queryType": "traceql",
					"refId":     "A",
					"limit":     20,
				},
			},
		},
	}
	panesJSON, err := json.Marshal(panes)
	if err != nil {
		return "", xerrors.Errorf("failed to marshal panes: %w", err)
	}
	return fmt.Sprintf("%s/explore?schemaVersion=1&panes=%s", baseURL, url.QueryEscape(string(panesJSON))), nil
}

func buildProfilingExploreURL(baseURL string, datasourceUID string, serviceName string) (string, error) {
	panes := map[string]interface{}{
		"default": map[string]interface{}{
			"datasource": datasourceUID,
			"queries": []map[string]interface{}{
				{
					"labelSelector": fmt.Sprintf(`{service_name="%s"}`, serviceName),
					"profileTypeId": "process_cpu:cpu:nanoseconds:cpu:nanoseconds",
					"queryType":     "profile",
					"refId":         "A",
					"spanSelector":  []string{},
					"groupBy":       []string{},
				},
			},
		},
	}
	panesJSON, err := json.Marshal(panes)
	if err != nil {
		return "", xerrors.Errorf("failed to marshal panes: %w", err)
	}
	return fmt.Sprintf("%s/explore?schemaVersion=1&panes=%s", baseURL, url.QueryEscape(string(panesJSON))), nil
}

func (p *Parser) hasTelemetry() bool {
	paths := p.findFiles("telemetry.yaml")
	return len(paths) > 0
}

func workloadTypeFromKind(doc map[string]interface{}) string {
	kind, _ := doc["kind"].(string)
	switch kind {
	case "Deployment":
		return "deployment"
	case "StatefulSet":
		return "statefulset"
	case "DaemonSet":
		return "daemonset"
	default:
		return ""
	}
}

func buildWorkloadDashboardURL(baseURL string, namespace string, workloadType string, workload string) string {
	return fmt.Sprintf(
		"%s/d/workload/workload?orgId=1&var-namespace=%s&var-workload_type=%s&var-workload=%s",
		baseURL,
		url.QueryEscape(namespace),
		url.QueryEscape(workloadType),
		url.QueryEscape(workload),
	)
}

func destinationServiceQueries() []QueryTemplate {
	return []QueryTemplate{
		{
			Name:     "Request Rate",
			Template: `sum(rate(istio_requests_total{<<.LabelMatchers>>}[5m])) by (response_code)`,
		},
		{
			Name:     "Error Rate",
			Template: `sum(rate(istio_requests_total{<<.LabelMatchers>>,response_code=~"5.*"}[5m]))`,
		},
		{
			Name:     "P99 Latency",
			Template: `histogram_quantile(0.99, sum(rate(istio_request_duration_milliseconds_bucket{<<.LabelMatchers>>}[5m])) by (le))`,
		},
	}
}

func sourceWorkloadQueries() []QueryTemplate {
	return []QueryTemplate{
		{
			Name:     "Outbound Request Rate",
			Template: `sum(rate(istio_requests_total{<<.LabelMatchers>>}[5m])) by (destination_service_name)`,
		},
		{
			Name:     "P99 Latency",
			Template: `histogram_quantile(0.99, sum(rate(istio_request_duration_milliseconds_bucket{<<.LabelMatchers>>}[5m])) by (le))`,
		},
	}
}

func containerResourceQueries() []QueryTemplate {
	return []QueryTemplate{
		{
			Name:     "CPU Usage",
			Template: `sum by (pod) (rate(container_cpu_usage_seconds_total{container!=""}[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "CPU Throttled",
			Template: `sum by (pod) (rate(container_cpu_cfs_throttled_periods_total{container!=""}[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>}) / sum by (pod) (rate(container_cpu_cfs_periods_total{container!=""}[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "CPU Request Utilization",
			Template: `sum by (pod) (rate(container_cpu_usage_seconds_total{container!=""}[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>}) / sum by (pod) (kube_pod_all_container_resource_requests{resource="cpu",unit="core"} * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Memory Working Set",
			Template: `sum by (pod) (container_memory_working_set_bytes{container!=""} * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Memory Limit Utilization",
			Template: `sum by (pod) (container_memory_working_set_bytes{container!=""} * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>}) / sum by (pod) (kube_pod_all_container_resource_limits{resource="memory",unit="byte"} * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Container Restarts",
			Template: `sum by (pod) (increase(kube_pod_all_container_status_restarts_total[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Network Receive",
			Template: `sum by (pod) (rate(container_network_receive_bytes_total[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Network Transmit",
			Template: `sum by (pod) (rate(container_network_transmit_bytes_total[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Network Receive Dropped",
			Template: `sum by (pod) (rate(container_network_receive_packets_dropped_total[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
		{
			Name:     "Network Transmit Dropped",
			Template: `sum by (pod) (rate(container_network_transmit_packets_dropped_total[5m]) * on(namespace,pod) group_left() mixin_pod_workload{<<.LabelMatchers>>})`,
		},
	}
}

func WriteRegistry(registry *Registry, outputPath string) error {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(registry); err != nil {
		return xerrors.Errorf("failed to marshal registry: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return xerrors.Errorf("failed to close encoder: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return xerrors.Errorf("failed to write registry file: %w", err)
	}

	return nil
}
