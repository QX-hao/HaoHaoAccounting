package queryutil

import (
	"strconv"
	"strings"
)

func ParseInt(value string, fallback int) int {
	i, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return i
}

func ParseUint(value string) uint {
	u, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return uint(u)
}

func ParsePositiveUint(value string) (uint, bool) {
	u, err := strconv.ParseUint(strings.TrimSpace(value), 10, strconv.IntSize)
	if err != nil || u == 0 {
		return 0, false
	}
	return uint(u), true
}
