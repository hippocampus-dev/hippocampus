package ruby_test

import (
	"armyknife/internal/marshal/ruby"
	"math"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func encodeLong(n int) string {
	long := make([]byte, 4)
	long[0] = byte(n & 0xff)
	long[1] = byte((n >> 8) & 0xff)
	long[2] = byte((n >> 16) & 0xff)
	long[3] = byte((n >> 24) & 0xff)

	index := len(long)
	for i := len(long) - 1; i >= 0; i-- {
		if long[i] != 0 {
			index = i + 1
			break
		}
	}

	s := string(rune(index))
	for i := 0; i < index; i++ {
		s += string(rune(long[i]))
	}
	return s
}

func TestDump(t *testing.T) {
	header := string(rune(ruby.MajorVersion)) + string(rune(ruby.MinorVersion))
	typeFixnumCode := string(rune(ruby.TypeFixnum))
	typeStringCode := string(rune(ruby.TypeString))
	longString := strings.Repeat("test", 100)

	tests := []struct {
		name    string
		in      interface{}
		want    string
		wantErr bool
	}{
		{name: "fixnum zero", in: 0, want: header + typeFixnumCode + string(rune(0))},
		{name: "fixnum overflow", in: int(math.Pow(2, 31)), wantErr: true},
		{name: "fixnum short", in: 1, want: header + typeFixnumCode + string(rune(1+ruby.Offset))},
		{name: "fixnum long", in: math.MaxInt32, want: header + typeFixnumCode + encodeLong(math.MaxInt32)},
		{name: "string short", in: "test", want: header + typeStringCode + string(rune(len("test")+5)) + "test"},
		{name: "string long", in: longString, want: header + typeStringCode + encodeLong(len(longString)) + longString},
		{name: "string multibyte", in: "テスト", want: header + typeStringCode + string(rune(len("テスト")+5)) + "テスト"},
	}

	for _, tt := range tests {
		name := tt.name
		in := tt.in
		want := tt.want
		wantErr := tt.wantErr
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ruby.Dump(in)
			if wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	header := string(rune(ruby.MajorVersion)) + string(rune(ruby.MinorVersion))
	typeFixnumCode := string(rune(ruby.TypeFixnum))
	typeStringCode := string(rune(ruby.TypeString))
	longString := strings.Repeat("test", 100)

	tests := []struct {
		name string
		in   string
		want interface{}
	}{
		{name: "fixnum zero", in: header + typeFixnumCode + string(rune(0)), want: 0},
		{name: "fixnum short", in: header + typeFixnumCode + string(rune(1+ruby.Offset)), want: 1},
		{name: "fixnum long", in: header + typeFixnumCode + encodeLong(math.MaxInt32), want: math.MaxInt32},
		{name: "string short", in: header + typeStringCode + string(rune(len("test")+5)) + "test", want: "test"},
		{name: "string long", in: header + typeStringCode + encodeLong(len(longString)) + longString, want: longString},
		{name: "string multibyte", in: header + typeStringCode + string(rune(len("テスト")+5)) + "テスト", want: "テスト"},
	}

	for _, tt := range tests {
		name := tt.name
		in := tt.in
		want := tt.want
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := ruby.Load(in)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}
