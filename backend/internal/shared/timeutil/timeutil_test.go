package timeutil

import (
	"testing"
	"time"
)

func TestResolveRangeTreatsDateEndAsFullDay(t *testing.T) {
	_, end := ResolveRange("2026-06-01", "2026-06-30")

	want := time.Date(2026, 6, 30, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}

func TestResolveRangeKeepsPreciseEndTime(t *testing.T) {
	_, end := ResolveRange("2026-06-01", "2026-06-30T10:30:00Z")

	want := time.Date(2026, 6, 30, 10, 30, 0, 0, time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}
