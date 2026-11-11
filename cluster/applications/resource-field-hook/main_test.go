package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRender(t *testing.T) {
	t.Parallel()

	const limit = 3758096384 // 3584Mi

	type in struct {
		expression string
		value      int64
	}

	tests := []struct {
		name            string
		in              in
		want            int64
		wantErrorString string
	}{
		{
			"a fraction of the value",
			in{`{{ scale .Value 0.9 }}`, limit},
			3382286745, // floor(3758096384 * 0.9)
			"",
		},
		{
			"a fixed reserve held back",
			in{`{{ sub .Value (quantity "64Mi") }}`, limit},
			3690987520,
			"",
		},
		{
			"whichever of the two leaves more headroom",
			in{`{{ min (scale .Value 0.9) (sub .Value (quantity "64Mi")) }}`, limit},
			3382286745,
			"",
		},
		{
			"a reserve suits a small value where a fraction would not",
			in{`{{ sub .Value (quantity "4Mi") }}`, 10 * 1024 * 1024},
			6291456,
			"",
		},
		{
			"the value passed through unchanged",
			in{`{{ .Value }}`, limit},
			limit,
			"",
		},
		{
			"infix arithmetic is not evaluated",
			in{`{{ .Value }} * 0.9`, limit},
			0,
			`expression produced "3758096384 * 0.9", which is not a byte count`,
		},
		{
			"a malformed template",
			in{`{{ scale .Value 0.9 `, limit},
			0,
			"failed to parse expression",
		},
		{
			"an unknown field",
			in{`{{ .Limit }}`, limit},
			0,
			"failed to execute expression",
		},
		{
			"an unknown function",
			in{`{{ multiply .Value 0.9 }}`, limit},
			0,
			"failed to parse expression",
		},
		{
			"a scale factor above one",
			in{`{{ scale .Value 1.5 }}`, limit},
			0,
			"scale factor 1.5 is not above 0 and at most 1",
		},
		{
			"a scale factor of zero",
			in{`{{ scale .Value 0 }}`, limit},
			0,
			"scale factor 0 is not above 0 and at most 1",
		},
		{
			"a reserve larger than the value",
			in{`{{ sub .Value (quantity "64Mi") }}`, 10 * 1024 * 1024},
			0,
			"leaves nothing to set",
		},
		{
			"a quantity that does not parse",
			in{`{{ sub .Value (quantity "64Megabytes") }}`, limit},
			0,
			`failed to parse quantity "64Megabytes"`,
		},
		{
			"an empty expression",
			in{``, limit},
			0,
			`expression produced "", which is not a byte count`,
		},
	}

	for _, tt := range tests {
		name := tt.name
		in := tt.in
		want := tt.want
		wantErrorString := tt.wantErrorString
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := render(in.expression, in.value)
			if err == nil {
				if wantErrorString != "" {
					t.Fatalf("expected error containing %q, got nil", wantErrorString)
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("(-want +got):\n%s", diff)
				}
				return
			}
			if wantErrorString == "" {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(err.Error(), wantErrorString) {
				t.Errorf("error %q does not contain %q", err.Error(), wantErrorString)
			}
		})
	}
}
