package retry_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"snapshot-controller/internal/retry"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type transportMock struct {
	http.RoundTripper
	fakeRoundTrip func(*http.Request) (*http.Response, error)
}

func (m *transportMock) RoundTrip(request *http.Request) (*http.Response, error) {
	return m.fakeRoundTrip(request)
}

type temporaryError struct {
	s string
}

func (e *temporaryError) Error() string {
	return e.s
}

func (e *temporaryError) Temporary() bool {
	return true
}

func TestHTTPClientDo(t *testing.T) {
	type in struct {
		first *http.Request
	}

	type want struct {
		first *http.Response
	}

	tests := []struct {
		name            string
		receiver        *http.Client
		in              in
		want            want
		wantErrorString string
		optsFunction    func(interface{}) cmp.Option
	}{
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func(request *http.Request) (*http.Response, error) {
							return &http.Response{
								StatusCode: http.StatusOK,
								Body:       io.NopCloser(strings.NewReader("fake")),
							}, nil
						},
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("fake")),
				},
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *http.Response:
					return cmp.AllowUnexported(strings.Reader{})
				default:
					return nil
				}
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func(request *http.Request) (*http.Response, error) {
							return nil, errors.New("fake")
						},
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				nil,
			},
			`Get "/": fake`,
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func() func(request *http.Request) (*http.Response, error) {
							i := 0
							return func(request *http.Request) (*http.Response, error) {
								i++
								if i == 1 {
									return nil, &temporaryError{
										"fake",
									}
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       io.NopCloser(strings.NewReader("fake")),
								}, nil
							}
						}(),
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("fake")),
				},
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *http.Response:
					return cmp.AllowUnexported(strings.Reader{})
				default:
					return nil
				}
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func() func(request *http.Request) (*http.Response, error) {
							i := 0
							return func(request *http.Request) (*http.Response, error) {
								i++
								if i == 1 {
									return &http.Response{
										StatusCode: http.StatusServiceUnavailable,
										Body:       io.NopCloser(strings.NewReader("fake")),
									}, nil
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       io.NopCloser(strings.NewReader("fake")),
								}, nil
							}
						}(),
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("fake")),
				},
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *http.Response:
					return cmp.AllowUnexported(strings.Reader{})
				default:
					return nil
				}
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func() func(request *http.Request) (*http.Response, error) {
							i := 0
							return func(request *http.Request) (*http.Response, error) {
								i++
								if i == 1 {
									return nil, &temporaryError{
										"fake",
									}
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       io.NopCloser(strings.NewReader("fake")),
								}, nil
							}
						}(),
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				nil,
			},
			`Get "/": fake`,
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func() func(request *http.Request) (*http.Response, error) {
							i := 0
							return func(request *http.Request) (*http.Response, error) {
								i++
								if i == 1 {
									return nil, &temporaryError{
										"fake",
									}
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       io.NopCloser(strings.NewReader("fake")),
								}, nil
							}
						}(),
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return request.WithContext(ctx)
				}(),
			},
			want{
				nil,
			},
			`Get "/": context canceled`,
			func(got interface{}) cmp.Option {
				return nil
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			&http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: func() func(request *http.Request) (*http.Response, error) {
							i := 0
							return func(request *http.Request) (*http.Response, error) {
								i++
								if i == 1 {
									return &http.Response{
										StatusCode: http.StatusServiceUnavailable,
										Body:       io.NopCloser(strings.NewReader("fake")),
									}, nil
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       io.NopCloser(strings.NewReader("fake")),
								}, nil
							}
						}(),
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			},
			in{
				func() *http.Request {
					request, err := http.NewRequest(http.MethodGet, "/", nil)
					if err != nil {
						t.Fatal()
					}
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return request.WithContext(ctx)
				}(),
			},
			want{
				nil,
			},
			`Get "/": context canceled`,
			func(got interface{}) cmp.Option {
				return nil
			},
		},
	}

	for _, tt := range tests {
		name := tt.name
		receiver := tt.receiver
		in := tt.in
		want := tt.want
		wantErrorString := tt.wantErrorString
		optsFunction := tt.optsFunction
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := receiver.Do(in.first)
			if diff := cmp.Diff(want.first, got, optsFunction(got)); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}

			if err == nil {
				if diff := cmp.Diff(wantErrorString, ""); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
				defer func() {
					_, _ = io.Copy(io.Discard, got.Body)
					_ = got.Body.Close()
				}()
			} else {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
		})
	}
}
