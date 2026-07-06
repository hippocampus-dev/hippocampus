package query_template_test

import (
	"armyknife/internal/query_template"
	"testing"
)

func TestProcess(t *testing.T) {
	tests := []struct {
		name          string
		queryTemplate string
		data          query_template.TemplateData
		want          string
		wantErr       bool
	}{
		{
			name:          "basic series replacement",
			queryTemplate: "<<.Series>>{<<.LabelMatchers>>}",
			data: query_template.TemplateData{
				Series:        "container_cpu_usage_seconds_total",
				LabelMatchers: `container!=""`,
			},
			want: `container_cpu_usage_seconds_total{container!=""}`,
		},
		{
			name:          "complex query with group_left",
			queryTemplate: `<<.Series>>{<<.LabelMatchers>>} * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="default",workload_type="deployment",workload="foo"}`,
			data: query_template.TemplateData{
				Series:        "container_cpu_usage_seconds_total",
				LabelMatchers: `container!=""`,
			},
			want: `container_cpu_usage_seconds_total{container!=""} * on(namespace,pod) group_left(workload,workload_type) mixin_pod_workload{namespace="default",workload_type="deployment",workload="foo"}`,
		},
		{
			name:          "with groupby",
			queryTemplate: "sum(rate(<<.Series>>{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>)",
			data: query_template.TemplateData{
				Series:        "http_requests_total",
				LabelMatchers: `namespace="production",job="api"`,
				GroupBy:       "pod,container",
			},
			want: `sum(rate(http_requests_total{namespace="production",job="api"}[2m])) by (pod,container)`,
		},
		{
			name:          "multiple label matchers",
			queryTemplate: "<<.Series>>{<<.LabelMatchers>>}",
			data: query_template.TemplateData{
				Series:        "memory_usage_bytes",
				LabelMatchers: `namespace="default",pod=~"myapp-.*",container!="POD"`,
			},
			want: `memory_usage_bytes{namespace="default",pod=~"myapp-.*",container!="POD"}`,
		},
		{
			name:          "empty label matchers",
			queryTemplate: "<<.Series>>{<<.LabelMatchers>>}",
			data: query_template.TemplateData{
				Series:        "up",
				LabelMatchers: "",
			},
			want: "up{}",
		},
		{
			name:          "template without placeholders",
			queryTemplate: "up{job='prometheus'}",
			data:          query_template.TemplateData{},
			want:          "up{job='prometheus'}",
		},
		{
			name:          "invalid template syntax",
			queryTemplate: "<<.Series>{<<.LabelMatchers>>}",
			data: query_template.TemplateData{
				Series:        "test",
				LabelMatchers: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		name := tt.name
		queryTemplate := tt.queryTemplate
		data := tt.data
		want := tt.want
		wantErr := tt.wantErr
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := query_template.Process(queryTemplate, data)
			if err != nil {
				if !wantErr {
					t.Errorf("Process() error = %+v", err)
				}
				return
			}
			if got != want {
				t.Errorf("Process() = %v, want %v", got, want)
			}
		})
	}
}

func TestExtractTemplateDataFromQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    *query_template.TemplateData
		wantErr bool
	}{
		{
			name:  "simple query with labels",
			query: `container_cpu_usage_seconds_total{container!=""}`,
			want: &query_template.TemplateData{
				Series:        "container_cpu_usage_seconds_total",
				LabelMatchers: `container!=""`,
				LabelValuesByName: map[string]string{
					"container": "",
				},
			},
		},
		{
			name:  "multiple labels",
			query: `http_requests_total{method="GET",status="200",job="api"}`,
			want: &query_template.TemplateData{
				Series:        "http_requests_total",
				LabelMatchers: `method="GET",status="200",job="api"`,
				LabelValuesByName: map[string]string{
					"method": "GET",
					"status": "200",
					"job":    "api",
				},
			},
		},
		{
			name:  "regex matcher",
			query: `up{job=~"prometheus.*",instance!="localhost:9090"}`,
			want: &query_template.TemplateData{
				Series:        "up",
				LabelMatchers: `job=~"prometheus.*",instance!="localhost:9090"`,
				LabelValuesByName: map[string]string{
					"job":      "prometheus.*",
					"instance": "localhost:9090",
				},
			},
		},
		{
			name:  "no labels",
			query: `up`,
			want: &query_template.TemplateData{
				Series:            "up",
				LabelMatchers:     "",
				LabelValuesByName: map[string]string{},
			},
		},
		{
			name:  "empty brackets",
			query: `metric{}`,
			want: &query_template.TemplateData{
				Series:            "metric",
				LabelMatchers:     "",
				LabelValuesByName: map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		name := tt.name
		query := tt.query
		want := tt.want
		wantErr := tt.wantErr
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := query_template.ExtractTemplateDataFromQuery(query)
			if err != nil {
				if !wantErr {
					t.Errorf("ExtractTemplateDataFromQuery() error = %+v", err)
				}
				return
			}
			if wantErr {
				t.Errorf("ExtractTemplateDataFromQuery() expected error but got none")
				return
			}
			if got.Series != want.Series {
				t.Errorf("Series = %v, want %v", got.Series, want.Series)
			}
			if got.LabelMatchers != want.LabelMatchers {
				t.Errorf("LabelMatchers = %v, want %v", got.LabelMatchers, want.LabelMatchers)
			}
			if len(got.LabelValuesByName) != len(want.LabelValuesByName) {
				t.Errorf("LabelValuesByName length = %v, want %v", len(got.LabelValuesByName), len(want.LabelValuesByName))
			}
			for k, v := range want.LabelValuesByName {
				if got.LabelValuesByName[k] != v {
					t.Errorf("LabelValuesByName[%s] = %v, want %v", k, got.LabelValuesByName[k], v)
				}
			}
		})
	}
}
