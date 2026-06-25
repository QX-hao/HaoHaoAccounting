package stringutil

import (
	"strings"
	"testing"
)

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		maxLength int
		want      string
	}{
		{
			name:      "keeps short value",
			value:     "HaoHaoMobile/1.0",
			maxLength: 256,
			want:      "HaoHaoMobile/1.0",
		},
		{
			name:      "keeps value when disabled",
			value:     "abcdef",
			maxLength: 0,
			want:      "abcdef",
		},
		{
			name:      "truncates ascii with marker",
			value:     "abcdef",
			maxLength: 5,
			want:      "ab...",
		},
		{
			name:      "truncates multibyte characters on rune boundary",
			value:     strings.Repeat("好", 6),
			maxLength: 5,
			want:      strings.Repeat("好", 2) + "...",
		},
		{
			name:      "truncates tiny limit without marker",
			value:     "abcdef",
			maxLength: 3,
			want:      "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateRunes(tt.value, tt.maxLength); got != tt.want {
				t.Fatalf("TruncateRunes(%q, %d) = %q, want %q", tt.value, tt.maxLength, got, tt.want)
			}
		})
	}
}
