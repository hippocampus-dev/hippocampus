package retry_test

import (
	"fmt"
	"math"
	"runtime"
	"snapshot-controller/internal/retry"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestRetrySleep(t *testing.T) {
	type in struct {
		first uint
	}

	type want struct {
		first  time.Duration
		second bool
	}

	tests := []struct {
		name     string
		receiver retry.Strategy
		in       in
		want     want
	}{
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewNever(),
			in{
				0,
			},
			want{
				0,
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(0, math.MaxInt64, 0, nil),
			in{
				0,
			},
			want{
				0,
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(0, math.MaxInt64, 1, func(i int64) int64 {
				return i
			}),
			in{
				0,
			},
			want{
				0,
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(0, math.MaxInt64, 1, func(i int64) int64 {
				return i
			}),
			in{
				1,
			},
			want{
				0,
				true,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(1*time.Second, math.MaxInt64, 2, func(i int64) int64 {
				return i
			}),
			in{
				0,
			},
			want{
				1 * time.Second,
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(1*time.Second, math.MaxInt64, 2, func(i int64) int64 {
				return i
			}),
			in{
				1,
			},
			want{
				2 * time.Second,
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(1*time.Second, 1*time.Second, 2, func(i int64) int64 {
				return i
			}),
			in{
				1,
			},
			want{
				1 * time.Second,
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(1*time.Second, math.MaxInt64, 64, func(i int64) int64 {
				return i
			}),
			in{
				63,
			},
			want{
				time.Duration(math.MaxInt64),
				false,
			},
		},
		{
			func() string {
				_, _, line, _ := runtime.Caller(1)
				return fmt.Sprintf("L%d", line)
			}(),
			retry.NewExponentialBackOff(100*time.Second, math.MaxInt64, 32, func(i int64) int64 {
				return i
			}),
			in{
				31,
			},
			want{
				time.Duration(math.MaxInt64),
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

			gotFirst, gotSecond := receiver.Sleep(in.first)
			if diff := cmp.Diff(want.first, gotFirst); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(want.second, gotSecond); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
