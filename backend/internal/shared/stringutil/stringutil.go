package stringutil

import "strings"

func FallbackName(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// TruncateRunes 按 Unicode 字符数量截断字符串，避免把中文或 emoji 之类的多字节字符切坏。
func TruncateRunes(value string, maxLength int) string {
	if maxLength <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	if maxLength <= 3 {
		return string(runes[:maxLength])
	}
	return string(runes[:maxLength-3]) + "..."
}
