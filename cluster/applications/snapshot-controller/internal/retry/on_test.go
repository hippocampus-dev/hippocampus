package retry_test

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"snapshot-controller/internal/retry"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCheckResponse(t *testing.T) {
	type in struct {
		first *http.Response
	}

	type want struct {
		first bool
	}

	tests := []struct {
		name     string
		receiver *retry.On
		in       in
		want     want
	}{
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("5xx")
				return o
			}(),
			in{
				&http.Response{StatusCode: 500},
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("5xx")
				return o
			}(),
			in{
				&http.Response{StatusCode: 404},
			},
			want{
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("gateway-error")
				return o
			}(),
			in{
				&http.Response{StatusCode: 502},
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("gateway-error")
				return o
			}(),
			in{
				&http.Response{StatusCode: 500},
			},
			want{
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("retriable-4xx")
				return o
			}(),
			in{
				&http.Response{StatusCode: 409},
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("retriable-4xx")
				return o
			}(),
			in{
				&http.Response{StatusCode: 404},
			},
			want{
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("500")
				return o
			}(),
			in{
				&http.Response{StatusCode: 500},
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("500")
				return o
			}(),
			in{
				&http.Response{StatusCode: 404},
			},
			want{
				false,
			},
		},
	}
	for _, tt := range tests {
		name := tt.name
		receiver := tt.receiver
		in := tt.in
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := receiver.CheckResponse(in.first)
			if diff := cmp.Diff(want.first, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckError(t *testing.T) {
	type in struct {
		first error
	}

	type want struct {
		first bool
	}

	tests := []struct {
		name     string
		receiver *retry.On
		in       in
		want     want
	}{
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("5xx")
				return o
			}(),
			in{
				io.EOF,
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("5xx")
				return o
			}(),
			in{
				&net.DNSError{IsTemporary: true},
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("5xx")
				return o
			}(),
			in{
				errors.New(""),
			},
			want{
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("connect-failure")
				return o
			}(),
			in{
				io.EOF,
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("connect-failure")
				return o
			}(),
			in{
				&net.DNSError{IsTemporary: true},
			},
			want{
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("connect-failure")
				return o
			}(),
			in{
				errors.New(""),
			},
			want{
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("gateway-error")
				return o
			}(),
			in{
				io.EOF,
			},
			want{
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			func() *retry.On {
				o, _ := retry.NewRetryOnFromString("gateway-error")
				return o
			}(),
			in{
				&net.DNSError{IsTemporary: true},
			},
			want{
				false,
			},
		},
	}
	for _, tt := range tests {
		name := tt.name
		receiver := tt.receiver
		in := tt.in
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := receiver.CheckError(in.first)
			if diff := cmp.Diff(want.first, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
