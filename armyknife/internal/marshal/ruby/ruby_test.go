package ruby_test

import (
	"armyknife/internal/marshal/ruby"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestDump(t *testing.T) {
	majorVersionCode := string(rune(ruby.MajorVersion))
	minorVersionCode := string(rune(ruby.MinorVersion))

	t.Run("fixnum", func(t *testing.T) {
		typeFixnumCode := string(ruby.TypeFixnum)

		t.Run("zero", func(t *testing.T) {
			n := 0

			expected := fmt.Sprintf("%s%s%s%s", majorVersionCode, minorVersionCode, typeFixnumCode, string(rune(0)))
			result, err := ruby.Dump(n)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})

		t.Run("overflow", func(t *testing.T) {
			n := int(math.Pow(2, 31))

			if _, err := ruby.Dump(n); err == nil {
				t.Error("Expected error, got nil")
			}
		})

		t.Run("short", func(t *testing.T) {
			n := 1

			expected := fmt.Sprintf("%s%s%s%s", majorVersionCode, minorVersionCode, typeFixnumCode, string(rune(n+ruby.Offset)))
			result, err := ruby.Dump(n)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})

		t.Run("long", func(t *testing.T) {
			n := math.MaxInt32

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

			var s string
			for i := 0; i < index; i++ {
				s += string(rune(long[i]))
			}

			expected := fmt.Sprintf("%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeFixnumCode,
				string(rune(index)),
				s,
			)
			result, err := ruby.Dump(n)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	})

	t.Run("string", func(t *testing.T) {
		typeStringCode := string(rune(ruby.TypeString))

		t.Run("short", func(t *testing.T) {
			s := "test"

			length := string(rune(len(s) + 5))

			expected := fmt.Sprintf("%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeStringCode,
				length,
				s,
			)
			result, err := ruby.Dump(s)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})

		t.Run("long", func(t *testing.T) {
			s := strings.Repeat("test", 100)

			long := make([]byte, 4)
			long[0] = byte(len(s) & 0xff)
			long[1] = byte((len(s) >> 8) & 0xff)
			long[2] = byte((len(s) >> 16) & 0xff)
			long[3] = byte((len(s) >> 24) & 0xff)

			index := len(long)
			for i := len(long) - 1; i >= 0; i-- {
				if long[i] != 0 {
					index = i + 1
					break
				}
			}

			var length string
			for i := 0; i < index; i++ {
				length += string(rune(long[i]))
			}

			expected := fmt.Sprintf("%s%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeStringCode,
				string(rune(index)),
				length,
				s,
			)
			result, err := ruby.Dump(s)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})

		t.Run("multibyte", func(t *testing.T) {
			s := "テスト"

			length := string(rune(len(s) + 5))

			expected := fmt.Sprintf("%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeStringCode,
				length,
				s,
			)
			result, err := ruby.Dump(s)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != expected {
				t.Errorf("Expected %q, got %q", expected, result)
			}
		})
	})
}

func TestLoad(t *testing.T) {
	majorVersionCode := string(rune(ruby.MajorVersion))
	minorVersionCode := string(rune(ruby.MinorVersion))

	t.Run("fixnum", func(t *testing.T) {
		typeFixnumCode := string(rune(ruby.TypeFixnum))

		t.Run("zero", func(t *testing.T) {
			n := 0

			result, err := ruby.Load(fmt.Sprintf("%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeFixnumCode,
				string(rune(0)),
			))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != n {
				t.Errorf("Expected 0, got %v", result)
			}
		})

		t.Run("short", func(t *testing.T) {
			n := 1

			result, err := ruby.Load(fmt.Sprintf("%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeFixnumCode,
				string(rune(n+ruby.Offset)),
			))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != n {
				t.Errorf("Expected %d, got %v", n, result)
			}
		})

		t.Run("long", func(t *testing.T) {
			n := math.MaxInt32

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

			var length string
			for i := 0; i < index; i++ {
				length += string(rune(long[i]))
			}

			result, err := ruby.Load(fmt.Sprintf("%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeFixnumCode,
				string(rune(index)),
				length,
			))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != n {
				t.Errorf("Expected %d, got %v", n, result)
			}
		})
	})

	t.Run("string", func(t *testing.T) {
		typeStringCode := string(rune(ruby.TypeString))

		t.Run("short", func(t *testing.T) {
			s := "test"

			result, err := ruby.Load(fmt.Sprintf("%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeStringCode,
				string(rune(len(s)+5)),
				s,
			))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != s {
				t.Errorf("Expected %q, got %q", s, result)
			}
		})

		t.Run("long", func(t *testing.T) {
			s := strings.Repeat("test", 100)

			long := make([]byte, 4)
			long[0] = byte(len(s) & 0xff)
			long[1] = byte((len(s) >> 8) & 0xff)
			long[2] = byte((len(s) >> 16) & 0xff)
			long[3] = byte((len(s) >> 24) & 0xff)

			index := len(long)
			for i := len(long) - 1; i >= 0; i-- {
				if long[i] != 0 {
					index = i + 1
					break
				}
			}

			var length string
			for i := 0; i < index; i++ {
				length += string(rune(long[i]))
			}

			result, err := ruby.Load(fmt.Sprintf("%s%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeStringCode,
				string(rune(index)),
				length,
				s,
			))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != s {
				t.Errorf("Expected %q, got %q", s, result)
			}
		})

		t.Run("multibyte", func(t *testing.T) {
			s := "テスト"

			result, err := ruby.Load(fmt.Sprintf("%s%s%s%s%s",
				majorVersionCode,
				minorVersionCode,
				typeStringCode,
				string(rune(len(s)+5)),
				s,
			))
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != s {
				t.Errorf("Expected %q, got %q", s, result)
			}
		})
	})
}
