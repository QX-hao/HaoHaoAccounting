package timeutil

import (
	"errors"
	"strings"
	"time"
)

// ParseDateTime centralizes all date formats accepted by API query strings
// and import files so modules do not drift over time.
func ParseDateTime(raw string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006/01/02"}
	for _, format := range formats {
		if t, err := time.Parse(format, strings.TrimSpace(raw)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid datetime")
}

func ResolveRange(startRaw, endRaw string) (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := now

	if t, err := ParseDateTime(startRaw); strings.TrimSpace(startRaw) != "" && err == nil {
		start = t
	}
	if t, err := ParseDateTime(endRaw); strings.TrimSpace(endRaw) != "" && err == nil {
		end = normalizeRangeEnd(strings.TrimSpace(endRaw), t)
	}
	return start, end
}

func normalizeRangeEnd(raw string, value time.Time) time.Time {
	if isDateOnly(raw) {
		return value.AddDate(0, 0, 1).Add(-time.Nanosecond)
	}
	return value
}

func isDateOnly(raw string) bool {
	if _, err := time.Parse("2006-01-02", raw); err == nil {
		return true
	}
	if _, err := time.Parse("2006/01/02", raw); err == nil {
		return true
	}
	return false
}
