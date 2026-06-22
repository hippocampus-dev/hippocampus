package handler_test

import (
	"alerthandler/handler"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/v68/github"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
)

func TestHandle(t *testing.T) {
	fakeClient := &kubernetesClientsetMock{}

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	type in struct {
		kubernetes    kubernetes.Interface
		dynamicClient dynamic.Interface
		gitHubClient  *github.Client
		request       *handler.AlertManagerRequest
	}

	tests := []struct {
		name            string
		in              in
		wantErrorString string
	}{
		{
			"do nothing when receive an empty request",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{},
			},
			"",
		},
		{
			"do nothing when receive a resolved alert",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "resolved",
				},
			},
			"",
		},
		{
			"return error when request does not have alertname",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
				},
			},
			"alertname label is not found",
		},
		{
			"success",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "RunOutContainerMemory at nc-application-sp in homes",
					},
				},
			},
			"",
		},
	}
	for _, tt := range tests {
		name := tt.name
		in := tt.in
		wantErrorString := tt.wantErrorString
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dispatcher := handler.NewDispatcher(in.kubernetes, in.dynamicClient, in.gitHubClient)
			err := dispatcher.Handle(in.request)
			if err == nil {
				if diff := cmp.Diff(wantErrorString, ""); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
		})
	}
}

func TestDispatch(t *testing.T) {
	fakeClient := &kubernetesClientsetMock{}
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &github.Issue{
			Number:  github.Ptr(1),
			HTMLURL: github.Ptr("https://github.com/test/repo/issues/1"),
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(issue)
	}))
	defer server.Close()

	fakeGitHubClient := github.NewClient(nil)
	fakeGitHubClient.BaseURL, _ = fakeGitHubClient.BaseURL.Parse(server.URL + "/")

	type in struct {
		kubernetes    kubernetes.Interface
		dynamicClient dynamic.Interface
		gitHubClient  *github.Client
		request       *handler.AlertManagerRequest
	}

	tests := []struct {
		name            string
		in              in
		wantHandlerType string
		wantErrorString string
	}{
		{
			"do nothing when receive an empty request",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{},
			},
			"",
			"handler is not found",
		},
		{
			"do nothing when receive a resolved alert",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "resolved",
				},
			},
			"",
			"handler is not found",
		},
		{
			"do nothing when request does not have alertname",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
				},
			},
			"",
			"alertname label is not found",
		},
		{
			"do nothing when receive NotFound alertname",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "NotFound",
					},
				},
			},
			"",
			"handler is not found",
		},
		{
			"return RunOutContainerMemoryHandler",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "RunOutContainerMemory",
					},
				},
			},
			"*handler.RunOutContainerMemoryHandler",
			"",
		},
		{
			"return CustomResourceStateStuckHandler for KafkaChannelStuck",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "KafkaChannelStuck",
					},
				},
			},
			"*handler.CustomResourceStateStuckHandler",
			"",
		},
		{
			"return CustomResourceStateStuckHandler for ConsumerGroupStuck",
			in{
				fakeClient,
				fakeDynamicClient,
				nil,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "ConsumerGroupStuck",
					},
				},
			},
			"*handler.CustomResourceStateStuckHandler",
			"",
		},
		{
			"return CriticalAlertHandler for critical severity",
			in{
				fakeClient,
				fakeDynamicClient,
				fakeGitHubClient,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "AnyAlert",
						"severity":  "critical",
					},
				},
			},
			"*handler.CriticalAlertHandler",
			"",
		},
	}
	for _, tt := range tests {
		name := tt.name
		in := tt.in
		wantHandlerType := tt.wantHandlerType
		wantErrorString := tt.wantErrorString
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dispatcher := handler.NewDispatcher(in.kubernetes, in.dynamicClient, in.gitHubClient)
			got, err := dispatcher.Dispatch(in.request)
			if wantHandlerType != "" {
				if diff := cmp.Diff(wantHandlerType, fmt.Sprintf("%T", got)); diff != "" {
					t.Errorf("handler type (-want +got):\n%s", diff)
				}
			} else {
				if got != nil {
					t.Errorf("expected nil handler, got %T", got)
				}
			}
			if err == nil {
				if diff := cmp.Diff(wantErrorString, ""); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
		})
	}
}
