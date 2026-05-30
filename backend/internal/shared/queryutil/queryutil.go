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
