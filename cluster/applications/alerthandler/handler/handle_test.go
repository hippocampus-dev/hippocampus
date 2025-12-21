package handler_test

import (
	"alerthandler/handler"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/client-go/kubernetes"
)

func TestHandle(t *testing.T) {
	fakeClient := &kubernetesClientsetMock{}

	type in struct {
		first  kubernetes.Interface
		second *handler.AlertManagerRequest
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
				&handler.AlertManagerRequest{},
			},
			"",
		},
		{
			"do nothing when receive a resolved alert",
			in{
				fakeClient,
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
			err := handler.Handle(in.first, in.second)
			if err != nil {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
		})
	}
}
func TestDispatch(t *testing.T) {
	fakeClient := &kubernetesClientsetMock{}

	type in struct {
		first  kubernetes.Interface
		second *handler.AlertManagerRequest
	}
	type want struct {
		first handler.Handler
	}

	tests := []struct {
		name            string
		in              in
		want            want
		wantErrorString string
		optsFunction    func(interface{}) cmp.Option
	}{
		{
			"do nothing when receive an empty request",
			in{
				fakeClient,
				&handler.AlertManagerRequest{},
			},
			want{
				nil,
			},
			"handler is not found",
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			"do nothing when receive a resolved alert",
			in{
				fakeClient,
				&handler.AlertManagerRequest{
					Status: "resolved",
				},
			},
			want{
				nil,
			},
			"handler is not found",
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			"do nothing when request does not have alertname",
			in{
				fakeClient,
				&handler.AlertManagerRequest{
					Status: "firing",
				},
			},
			want{
				nil,
			},
			"alertname label is not found",
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			"do nothing when receive NotFound alertname",
			in{
				fakeClient,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "NotFound",
					},
				},
			},
			want{
				nil,
			},
			"handler is not found",
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			"return RunOutContainerMemoryHandler",
			in{
				fakeClient,
				&handler.AlertManagerRequest{
					Status: "firing",
					CommonLabels: map[string]string{
						"alertname": "RunOutContainerMemory",
					},
				},
			},
			want{
				handler.NewRunOutContainerMemoryHandler(fakeClient, time.Millisecond*100, time.Minute*5),
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *handler.RunOutContainerMemoryHandler:
					deref := func(v interface{}) interface{} {
						return reflect.ValueOf(v).Elem().Interface()
					}
					var opts cmp.Options
					opts = []cmp.Option{
						cmp.AllowUnexported(deref(got)),
						cmp.AllowUnexported(deref(fakeClient)),
					}
					return opts
				default:
					return nil
				}
			},
		},
	}
	for _, tt := range tests {
		name := tt.name
		in := tt.in
		want := tt.want
		wantErrorString := tt.wantErrorString
		optsFunction := tt.optsFunction
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := handler.Dispatch(in.first, in.second)
			if diff := cmp.Diff(want.first, got, optsFunction(got)); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
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
