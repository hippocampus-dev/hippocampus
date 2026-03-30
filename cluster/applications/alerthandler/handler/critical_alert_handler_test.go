package handler_test

import (
	"alerthandler/handler"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
			"return error when repository label is missing",
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
			"repository label is required",
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
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			gitHubClient := github.NewClient(nil)
			gitHubClient.BaseURL, _ = gitHubClient.BaseURL.Parse(server.URL + "/")
			h := handler.NewCriticalAlertHandler(gitHubClient)

			err := h.Call(tt.in.first)
			if err == nil {
				if tt.wantErrorString != "" {
					t.Errorf("expected error containing %q, got nil", tt.wantErrorString)
				}
			} else {
				if tt.wantErrorString == "" {
					t.Errorf("unexpected error: %v", err)
				} else if !strings.Contains(err.Error(), tt.wantErrorString) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrorString)
				}
			}
		})
	}
}
