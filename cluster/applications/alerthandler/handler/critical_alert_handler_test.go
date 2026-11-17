package handler_test

import (
	"alerthandler/handler"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v68/github"
)

func TestCriticalAlertHandler_Call(t *testing.T) {
	type in struct {
		first *handler.AlertManagerRequest
	}

	tests := []struct {
		name            string
		serverHandler   http.HandlerFunc
		in              in
		wantErrorString string
	}{
		{
			"do nothing when alerts is empty",
			func(w http.ResponseWriter, r *http.Request) {
				t.Error("CreateIssue should not be called")
			},
			in{
				&handler.AlertManagerRequest{
					Alerts: []handler.Alert{},
				},
			},
			"",
		},
		{
			"skip resolved alerts",
			func(w http.ResponseWriter, r *http.Request) {
				t.Error("CreateIssue should not be called for resolved alerts")
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "TestAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "resolved",
							Labels: map[string]string{
								"namespace": "default",
								"pod":       "test-pod",
							},
						},
					},
				},
			},
			"",
		},
		{
			"skip alerts without a repository label instead of failing the batch",
			func(w http.ResponseWriter, r *http.Request) {
				t.Error("CreateIssue should not be called when repository is missing")
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "TestAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "firing",
							Labels: map[string]string{
								"namespace": "default",
								"pod":       "test-pod",
							},
						},
					},
				},
			},
			"",
		},
		{
			"return error when CreateIssue fails",
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "TestAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "firing",
							Labels: map[string]string{
								"namespace":  "default",
								"pod":        "test-pod",
								"repository": "test/repo",
							},
						},
					},
				},
			},
			"alert TestAlert: failed to create issue: POST",
		},
		{
			"success with repository label",
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/test/repo/issues" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				issue := &github.Issue{
					Number:  github.Ptr(123),
					HTMLURL: github.Ptr("https://github.com/test/repo/issues/123"),
				}
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issue)
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "TestAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "firing",
							Labels: map[string]string{
								"namespace":  "default",
								"pod":        "test-pod",
								"repository": "test/repo",
							},
							StartsAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			"",
		},
		{
			"success with custom repository",
			func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/custom-org/custom-repo/issues" {
					t.Errorf("unexpected path: %s, want /repos/custom-org/custom-repo/issues", r.URL.Path)
				}
				issue := &github.Issue{
					Number:  github.Ptr(456),
					HTMLURL: github.Ptr("https://github.com/custom-org/custom-repo/issues/456"),
				}
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issue)
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "TestAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "firing",
							Labels: map[string]string{
								"namespace":  "default",
								"pod":        "test-pod",
								"repository": "custom-org/custom-repo",
							},
							StartsAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			"",
		},
		{
			"build title with only namespace",
			func(w http.ResponseWriter, r *http.Request) {
				var req github.IssueRequest
				json.NewDecoder(r.Body).Decode(&req)
				if diff := cmp.Diff("[CRITICAL] TestAlert: production", *req.Title); diff != "" {
					t.Errorf("title mismatch (-want +got):\n%s", diff)
				}
				issue := &github.Issue{
					Number:  github.Ptr(789),
					HTMLURL: github.Ptr("https://github.com/test/repo/issues/789"),
				}
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issue)
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "TestAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "firing",
							Labels: map[string]string{
								"namespace":  "production",
								"repository": "test/repo",
							},
							StartsAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			"",
		},
		{
			"build title without namespace and pod",
			func(w http.ResponseWriter, r *http.Request) {
				var req github.IssueRequest
				json.NewDecoder(r.Body).Decode(&req)
				if diff := cmp.Diff("[CRITICAL] ClusterAlert", *req.Title); diff != "" {
					t.Errorf("title mismatch (-want +got):\n%s", diff)
				}
				issue := &github.Issue{
					Number:  github.Ptr(999),
					HTMLURL: github.Ptr("https://github.com/test/repo/issues/999"),
				}
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(issue)
			},
			in{
				&handler.AlertManagerRequest{
					CommonLabels: map[string]string{
						"alertname": "ClusterAlert",
						"severity":  "critical",
					},
					Alerts: []handler.Alert{
						{
							Status: "firing",
							Labels: map[string]string{
								"repository": "test/repo",
							},
							StartsAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		name := tt.name
		serverHandler := tt.serverHandler
		in := tt.in
		wantErrorString := tt.wantErrorString
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte("[]"))
					return
				}
				serverHandler(w, r)
			}))
			defer server.Close()

			gitHubClient := github.NewClient(nil)
			gitHubClient.BaseURL, _ = gitHubClient.BaseURL.Parse(server.URL + "/")
			h := handler.NewCriticalAlertHandler(gitHubClient, time.Second*30, "")

			err := h.Call(in.first)
			if err == nil {
				if wantErrorString != "" {
					t.Errorf("expected error containing %q, got nil", wantErrorString)
				}
			} else {
				if wantErrorString == "" {
					t.Errorf("unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), wantErrorString) {
					t.Errorf("error %q does not contain %q", err.Error(), wantErrorString)
				}
			}
		})
	}
}

func TestCriticalAlertHandler_Call_Links(t *testing.T) {
	type in struct {
		grafanaAlertsURL string
		generatorURL     string
	}

	tests := []struct {
		name      string
		in        in
		wantLinks string
	}{
		{
			"link to the alert rule list filtered by alertname",
			in{"https://grafana.example.com/alerting/list", "/graph?g0.expr=up&g0.tab=1"},
			"\n## Links\n\n- [View in Grafana](https://grafana.example.com/alerting/list?search=rule%3A%22TestAlert%22)\n",
		},
		{
			"omit links when grafana alerts url is empty",
			in{"", "/graph?g0.expr=up&g0.tab=1"},
			"",
		},
	}
	for _, tt := range tests {
		name := tt.name
		in := tt.in
		wantLinks := tt.wantLinks
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			created := &atomic.Bool{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					_, _ = w.Write([]byte("[]"))
					return
				}
				var req github.IssueRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				gotLinks := ""
				if index := strings.Index(req.GetBody(), "\n## Links"); index >= 0 {
					gotLinks = req.GetBody()[index:]
				}
				if diff := cmp.Diff(wantLinks, gotLinks); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
				created.Store(true)
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(&github.Issue{Number: github.Ptr(1)})
			}))
			defer server.Close()

			gitHubClient := github.NewClient(nil)
			gitHubClient.BaseURL, _ = gitHubClient.BaseURL.Parse(server.URL + "/")
			h := handler.NewCriticalAlertHandler(gitHubClient, time.Second*30, in.grafanaAlertsURL)

			err := h.Call(&handler.AlertManagerRequest{
				CommonLabels: map[string]string{
					"alertname": "TestAlert",
					"severity":  "critical",
				},
				Alerts: []handler.Alert{
					{
						Status: "firing",
						Labels: map[string]string{
							"alertname":  "TestAlert",
							"severity":   "critical",
							"repository": "test/repo",
						},
						StartsAt:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						GeneratorURL: in.generatorURL,
					},
				},
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !created.Load() {
				t.Error("CreateIssue should be called")
			}
		})
	}
}
