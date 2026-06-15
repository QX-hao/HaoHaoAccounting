package queryutil

import (
	"strconv"
	"testing"
)

func TestParsePositiveUint(t *testing.T) {
	for _, value := range []string{"1", "42", " 7 "} {
		if got, ok := ParsePositiveUint(value); !ok || got == 0 {
			t.Fatalf("ParsePositiveUint(%q) = %d, %v", value, got, ok)
		}
	}

	for _, value := range []string{"", "0", "-1", "abc"} {
		if got, ok := ParsePositiveUint(value); ok || got != 0 {
			t.Fatalf("ParsePositiveUint(%q) = %d, %v, want 0 false", value, got, ok)
		}
	}

	overflow := strconv.FormatUint(uint64(maxUint())+1, 10)
	if got, ok := ParsePositiveUint(overflow); ok || got != 0 {
		t.Fatalf("ParsePositiveUint(%q) = %d, %v, want 0 false", overflow, got, ok)
	}
}

func maxUint() uint {
	return ^uint(0)
}
