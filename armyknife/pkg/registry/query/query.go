package query

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"armyknife/internal/bakery"
	"armyknife/internal/grafana"
	"armyknife/internal/query_template"
	"armyknife/internal/registry"

	"github.com/go-playground/validator/v10"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	"gopkg.in/yaml.v3"
)

type resultOutput struct {
	Registry   string            `yaml:"registry"`
	Namespace  string            `yaml:"namespace"`
	Components []componentOutput `yaml:"components"`
}

type componentOutput struct {
	Name      string           `yaml:"name"`
	Metrics   []metricOutput   `yaml:"metrics,omitempty"`
	Logs      *logOutput       `yaml:"logs,omitempty"`
	Traces    *traceOutput     `yaml:"traces,omitempty"`
	Profiling *profilingOutput `yaml:"profiling,omitempty"`
}

type metricOutput struct {
	Name   string               `yaml:"name"`
	Query  string               `yaml:"query"`
	Series []metricSeriesOutput `yaml:"series,omitempty"`
	Error  string               `yaml:"error,omitempty"`
}

type metricSeriesOutput struct {
	Labels map[string]string `yaml:"labels,omitempty"`
	Values [][2]float64      `yaml:"values"`
}

type logOutput struct {
	Query string          `yaml:"query"`
	Lines []logLineOutput `yaml:"lines,omitempty"`
	Error string          `yaml:"error,omitempty"`
}

type logLineOutput struct {
	Timestamp string `yaml:"timestamp"`
	Body      string `yaml:"body"`
}

type traceOutput struct {
	Query  string      `yaml:"query"`
	Traces []traceItem `yaml:"traces,omitempty"`
	Error  string      `yaml:"error,omitempty"`
}

type traceItem struct {
	TraceID  string `yaml:"traceId"`
	Name     string `yaml:"name"`
	Duration string `yaml:"duration"`
}

type profilingOutput struct {
	Query  string               `yaml:"query"`
	Series []metricSeriesOutput `yaml:"series,omitempty"`
	Error  string               `yaml:"error,omitempty"`
}

func Run(a *Args) error {
	absPath, err := filepath.Abs(a.Directory)
	if err != nil {
		return xerrors.Errorf("failed to resolve path: %w", err)
	}
	a.Directory = absPath

	if err := validator.New().Struct(a); err != nil {
		return xerrors.Errorf("validation error: %w", err)
	}

	registryPath := filepath.Join(a.Directory, ".registry.yaml")
	r, err := registry.ReadRegistry(registryPath)
	if err != nil {
		return xerrors.Errorf("failed to read registry %s: %w", registryPath, err)
	}

	bakeryClient := bakery.NewClient("https://bakery.kaidotio.dev/callback", a.AuthorizationListenPort)
	client := grafana.NewClient(a.GrafanaURL, bakeryClient)

	now := time.Now()
	to := now.Add(-a.To)
	from := to.Add(-a.From)
	ctx := context.Background()

	output := resultOutput{
		Registry:  r.Name,
		Namespace: r.Namespace,
	}

	for _, component := range r.Components {
		co, err := processComponent(ctx, component, a, client, from, to)
		if err != nil {
			return err
		}
		output.Components = append(output.Components, co)
	}

	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	if err := encoder.Encode(output); err != nil {
		return xerrors.Errorf("failed to encode output: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return xerrors.Errorf("failed to close encoder: %w", err)
	}

	return nil
}

func signalEnabled(signals []string, name string) bool {
	if len(signals) == 0 {
		return true
	}
	for _, s := range signals {
		if s == name {
			return true
		}
	}
	return false
}

func processComponent(ctx context.Context, component registry.Component, a *Args, client *grafana.Client, from time.Time, to time.Time) (componentOutput, error) {
	co := componentOutput{
		Name: component.Name,
	}

	eg, ctx := errgroup.WithContext(ctx)

	if component.Metrics != nil && signalEnabled(a.Signals, "metrics") {
		eg.Go(func() error {
			for _, labelSet := range component.Metrics.LabelSets {
				for _, qt := range labelSet.Queries {
					labelMatchers := buildLabelMatchers(labelSet.Labels)
					expr, err := query_template.Process(qt.Template, query_template.TemplateData{
						LabelMatchers: labelMatchers,
					})
					if err != nil {
						co.Metrics = append(co.Metrics, metricOutput{
							Name:  qt.Name,
							Query: qt.Template,
							Error: err.Error(),
						})
						continue
					}

					q, err := grafana.NewPrometheusQueryBuilder().
						WithDatasourceUID(a.PrometheusDatasourceUID).
						WithExpression(expr).
						WithInterval(a.Step.String()).
						Build()
					if err != nil {
						co.Metrics = append(co.Metrics, metricOutput{
							Name:  qt.Name,
							Query: expr,
							Error: err.Error(),
						})
						continue
					}

					result, err := client.QueryPrometheus(ctx, []grafana.Query{q}, from, to)
					m := metricOutput{
						Name:  qt.Name,
						Query: expr,
					}
					if err != nil {
						m.Error = err.Error()
					} else {
						m.Series = extractMetricSeries(result)
					}
					co.Metrics = append(co.Metrics, m)
				}
			}
			return nil
		})
	}

	if component.Logs != nil && component.Logs.Grouping != "" && signalEnabled(a.Signals, "logs") {
		eg.Go(func() error {
			expr := fmt.Sprintf(`{grouping="%s"}`, component.Logs.Grouping)
			q, err := grafana.NewLokiQueryBuilder().
				WithDatasourceUID(a.LokiDatasourceUID).
				WithExpression(expr).
				Build()
			if err != nil {
				co.Logs = &logOutput{
					Query: expr,
					Error: err.Error(),
				}
				return nil
			}
			result, err := client.QueryLoki(ctx, []grafana.LokiQuery{q}, from, to)
			o := &logOutput{Query: expr}
			if err != nil {
				o.Error = err.Error()
			} else {
				o.Lines = extractLogLines(result)
			}
			co.Logs = o
			return nil
		})
	}

	if component.Traces != nil && component.Traces.ServiceName != "" && signalEnabled(a.Signals, "traces") {
		eg.Go(func() error {
			traceQL := fmt.Sprintf(`{resource.service.name="%s"}`, component.Traces.ServiceName)
			q, err := grafana.NewTempoQueryBuilder().
				WithDatasourceUID(a.TempoDatasourceUID).
				WithQuery(traceQL).
				Build()
			if err != nil {
				co.Traces = &traceOutput{
					Query: traceQL,
					Error: err.Error(),
				}
				return nil
			}
			result, err := client.QueryTempo(ctx, []grafana.TempoQuery{q}, from, to)
			o := &traceOutput{Query: traceQL}
			if err != nil {
				o.Error = err.Error()
			} else {
				o.Traces = extractTraceItems(result)
			}
			co.Traces = o
			return nil
		})
	}

	if component.Profiling != nil && component.Profiling.ServiceName != "" && signalEnabled(a.Signals, "profiling") {
		eg.Go(func() error {
			labelSelector := fmt.Sprintf(`{service_name="%s"}`, component.Profiling.ServiceName)
			q, err := grafana.NewPyroscopeQueryBuilder().
				WithDatasourceUID(a.PyroscopeDatasourceUID).
				WithLabelSelector(labelSelector).
				Build()
			if err != nil {
				co.Profiling = &profilingOutput{
					Query: labelSelector,
					Error: err.Error(),
				}
				return nil
			}
			result, err := client.QueryPyroscope(ctx, []grafana.PyroscopeQuery{q}, from, to)
			o := &profilingOutput{Query: labelSelector}
			if err != nil {
				o.Error = err.Error()
			} else {
				o.Series = extractMetricSeries(result)
			}
			co.Profiling = o
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return co, err
	}

	return co, nil
}

func buildLabelMatchers(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	matchers := make([]string, 0, len(labels))
	for _, k := range keys {
		matchers = append(matchers, fmt.Sprintf(`%s="%s"`, k, labels[k]))
	}
	return strings.Join(matchers, ",")
}

func extractMetricSeries(response *grafana.QueryResponse) []metricSeriesOutput {
	var series []metricSeriesOutput
	for _, result := range response.Results {
		for _, frame := range result.Frames {
			if len(frame.Schema.Fields) < 2 || len(frame.Data.Values) < 2 {
				continue
			}

			timeIndex := -1
			for i, field := range frame.Schema.Fields {
				if strings.ToLower(field.Name) == "time" {
					timeIndex = i
					break
				}
			}

			for i, field := range frame.Schema.Fields {
				if strings.ToLower(field.Name) == "time" || i >= len(frame.Data.Values) {
					continue
				}

				values := frame.Data.Values[i]
				s := metricSeriesOutput{Labels: field.Labels}
				for j, v := range values {
					f, ok := v.(float64)
					if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
						continue
					}
					var ts float64
					if timeIndex >= 0 && timeIndex < len(frame.Data.Values) && j < len(frame.Data.Values[timeIndex]) {
						if t, ok := frame.Data.Values[timeIndex][j].(float64); ok {
							ts = t
						}
					}
					s.Values = append(s.Values, [2]float64{ts, f})
				}
				series = append(series, s)
			}
		}
	}
	return series
}

func extractLogLines(response *grafana.QueryResponse) []logLineOutput {
	var lines []logLineOutput

	for _, result := range response.Results {
		for _, frame := range result.Frames {
			bodyIndex := -1
			timeIndex := -1
			for i, field := range frame.Schema.Fields {
				name := strings.ToLower(field.Name)
				if name == "line" || name == "body" {
					bodyIndex = i
				}
				if name == "time" || name == "timestamp" || name == "ts" {
					timeIndex = i
				}
			}

			if bodyIndex < 0 || bodyIndex >= len(frame.Data.Values) {
				continue
			}

			bodyValues := frame.Data.Values[bodyIndex]

			var timeValues []interface{}
			if timeIndex >= 0 && timeIndex < len(frame.Data.Values) {
				timeValues = frame.Data.Values[timeIndex]
			}

			for j, v := range bodyValues {
				body, ok := v.(string)
				if !ok {
					continue
				}
				line := logLineOutput{Body: body}
				if timeValues != nil && j < len(timeValues) {
					if ts, ok := timeValues[j].(float64); ok {
						t := time.UnixMilli(int64(ts))
						line.Timestamp = t.UTC().Format(time.RFC3339)
					}
				}
				lines = append(lines, line)
			}
		}
	}

	return lines
}

func extractTraceItems(response *grafana.QueryResponse) []traceItem {
	var items []traceItem

	for _, result := range response.Results {
		for _, frame := range result.Frames {
			fieldIndices := map[string]int{}
			for i, field := range frame.Schema.Fields {
				fieldIndices[field.Name] = i
			}

			rowCount := 0
			for _, values := range frame.Data.Values {
				rowCount = len(values)
				break
			}

			for j := 0; j < rowCount; j++ {
				item := traceItem{}
				if idx, ok := fieldIndices["traceID"]; ok && idx < len(frame.Data.Values) && j < len(frame.Data.Values[idx]) {
					if v, ok := frame.Data.Values[idx][j].(string); ok {
						item.TraceID = v
					}
				}
				if idx, ok := fieldIndices["traceName"]; ok && idx < len(frame.Data.Values) && j < len(frame.Data.Values[idx]) {
					if v, ok := frame.Data.Values[idx][j].(string); ok {
						item.Name = v
					}
				}
				if idx, ok := fieldIndices["duration"]; ok && idx < len(frame.Data.Values) && j < len(frame.Data.Values[idx]) {
					if v, ok := frame.Data.Values[idx][j].(float64); ok {
						item.Duration = formatDuration(v)
					}
				}
				items = append(items, item)
			}
		}
	}

	return items
}

func formatDuration(nanoseconds float64) string {
	if math.IsNaN(nanoseconds) || math.IsInf(nanoseconds, 0) || nanoseconds < 0 {
		return "N/A"
	}
	d := time.Duration(int64(nanoseconds))
	if d >= time.Second {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d >= time.Millisecond {
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fµs", float64(d)/float64(time.Microsecond))
}
