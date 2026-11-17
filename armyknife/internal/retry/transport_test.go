package retry_test

import (
	"armyknife/internal/retry"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"runtime"
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

func (te *temporaryError) Error() string {
	return te.s
}

func (te *temporaryError) Temporary() bool {
	return true
}

func TestHTTPClientDo(t *testing.T) {
	fakeReader := strings.NewReader("fake")

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
								Body:       ioutil.NopCloser(fakeReader),
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
					request, err := http.NewRequest("GET", "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(fakeReader),
				},
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *http.Response:
					return cmp.AllowUnexported(*fakeReader)
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
					request, err := http.NewRequest("GET", "/", nil)
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
									Body:       ioutil.NopCloser(fakeReader),
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
					request, err := http.NewRequest("GET", "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(fakeReader),
				},
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *http.Response:
					return cmp.AllowUnexported(*fakeReader)
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
										Body:       ioutil.NopCloser(fakeReader),
									}, nil
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       ioutil.NopCloser(fakeReader),
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
					request, err := http.NewRequest("GET", "/", nil)
					if err != nil {
						t.Fatal()
					}
					return request
				}(),
			},
			want{
				&http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(fakeReader),
				},
			},
			"",
			func(got interface{}) cmp.Option {
				switch got.(type) {
				case *http.Response:
					return cmp.AllowUnexported(*fakeReader)
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
									Body:       ioutil.NopCloser(fakeReader),
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
					request, err := http.NewRequest("GET", "/", nil)
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
									Body:       ioutil.NopCloser(fakeReader),
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
					request, err := http.NewRequest("GET", "/", nil)
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
										Body:       ioutil.NopCloser(fakeReader),
									}, nil
								}
								return &http.Response{
									StatusCode: http.StatusOK,
									Body:       ioutil.NopCloser(fakeReader),
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
					request, err := http.NewRequest("GET", "/", nil)
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
				defer got.Body.Close()
			} else {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
		})
	}
}

type unrewindableReader struct {
	io.Reader
}

func TestHTTPClientDoPOST(t *testing.T) {
	type want struct {
		statusCode int
		attempts   int
		bodies     []string
	}

	tests := []struct {
		name            string
		newRequest      func() *http.Request
		fakeRoundTrip   func(*int, *[]string) func(*http.Request) (*http.Response, error)
		want            want
		wantErrorString string
	}{
		{
			name: "rewinds body from strings.NewReader on retriable error",
			newRequest: func() *http.Request {
				request, err := http.NewRequest("POST", "/", strings.NewReader("payload"))
				if err != nil {
					t.Fatal(err)
				}
				return request
			},
			fakeRoundTrip: func(attempts *int, bodies *[]string) func(*http.Request) (*http.Response, error) {
				return func(request *http.Request) (*http.Response, error) {
					*attempts++
					b, err := io.ReadAll(request.Body)
					if err != nil {
						return nil, err
					}
					*bodies = append(*bodies, string(b))
					if *attempts == 1 {
						return nil, &temporaryError{"fake"}
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(strings.NewReader("")),
					}, nil
				}
			},
			want: want{
				statusCode: http.StatusOK,
				attempts:   2,
				bodies:     []string{"payload", "payload"},
			},
		},
		{
			name: "rewinds body from strings.NewReader on retriable response",
			newRequest: func() *http.Request {
				request, err := http.NewRequest("POST", "/", strings.NewReader("payload"))
				if err != nil {
					t.Fatal(err)
				}
				return request
			},
			fakeRoundTrip: func(attempts *int, bodies *[]string) func(*http.Request) (*http.Response, error) {
				return func(request *http.Request) (*http.Response, error) {
					*attempts++
					b, err := io.ReadAll(request.Body)
					if err != nil {
						return nil, err
					}
					*bodies = append(*bodies, string(b))
					if *attempts == 1 {
						return &http.Response{
							StatusCode: http.StatusServiceUnavailable,
							Body:       ioutil.NopCloser(strings.NewReader("err")),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(strings.NewReader("")),
					}, nil
				}
			},
			want: want{
				statusCode: http.StatusOK,
				attempts:   2,
				bodies:     []string{"payload", "payload"},
			},
		},
		{
			name: "buffers and rewinds body when GetBody is missing",
			newRequest: func() *http.Request {
				request, err := http.NewRequest("POST", "/", &unrewindableReader{strings.NewReader("payload")})
				if err != nil {
					t.Fatal(err)
				}
				request.GetBody = nil
				return request
			},
			fakeRoundTrip: func(attempts *int, bodies *[]string) func(*http.Request) (*http.Response, error) {
				return func(request *http.Request) (*http.Response, error) {
					*attempts++
					b, err := io.ReadAll(request.Body)
					if err != nil {
						return nil, err
					}
					*bodies = append(*bodies, string(b))
					if *attempts == 1 {
						return &http.Response{
							StatusCode: http.StatusServiceUnavailable,
							Body:       ioutil.NopCloser(strings.NewReader("err")),
						}, nil
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(strings.NewReader("")),
					}, nil
				}
			},
			want: want{
				statusCode: http.StatusOK,
				attempts:   2,
				bodies:     []string{"payload", "payload"},
			},
		},
	}

	for _, tt := range tests {
		name := tt.name
		newRequest := tt.newRequest
		fakeRoundTrip := tt.fakeRoundTrip
		want := tt.want
		wantErrorString := tt.wantErrorString
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			attempts := 0
			bodies := []string{}
			client := &http.Client{
				Transport: &retry.Transport{
					Base: &transportMock{
						fakeRoundTrip: fakeRoundTrip(&attempts, &bodies),
					},
					RetryStrategy: retry.NewExponentialBackOff(1*time.Millisecond, 10*time.Second, 5, nil),
					RetryOn: func() *retry.On {
						retryOn, _ := retry.NewRetryOnFromString("gateway-error,retriable-4xx,connect-failure")
						return retryOn
					}(),
				},
			}

			got, err := client.Do(newRequest())
			if err == nil {
				defer got.Body.Close()
				if diff := cmp.Diff(wantErrorString, ""); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(want.statusCode, got.StatusCode); diff != "" {
					t.Errorf("status (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(wantErrorString, err.Error()); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
			}
			if diff := cmp.Diff(want.attempts, attempts); diff != "" {
				t.Errorf("attempts (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(want.bodies, bodies); diff != "" {
				t.Errorf("bodies (-want +got):\n%s", diff)
			}
		})
	}
}
