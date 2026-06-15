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

func TestResolveRangeStrictRejectsReversedExplicitRange(t *testing.T) {
	_, _, err := ResolveRangeStrict("2026-07-01", "2026-06-30")
	if err == nil {
		t.Fatal("err = nil, want reversed range error")
	}
	if err.Error() != "start datetime must be before or equal to end datetime" {
		t.Fatalf("err = %q", err.Error())
	}
}

func TestResolveOptionalRangeStrictUsesZeroDefaults(t *testing.T) {
	start, end, err := ResolveOptionalRangeStrict("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !start.IsZero() {
		t.Fatalf("start = %v, want zero", start)
	}
	if !end.IsZero() {
		t.Fatalf("end = %v, want zero", end)
	}
}

func TestResolveOptionalRangeStrictTreatsDateEndAsFullDay(t *testing.T) {
	_, end, err := ResolveOptionalRangeStrict("", "2026-06-30")
	if err != nil {
		t.Fatal(err)
	}

	want := time.Date(2026, 6, 30, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}

func TestResolveOptionalRangeStrictTrimsDateEndBeforeNormalizing(t *testing.T) {
	_, end, err := ResolveOptionalRangeStrict("", " 2026-06-30 ")
	if err != nil {
		t.Fatal(err)
	}

	want := time.Date(2026, 6, 30, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if !end.Equal(want) {
		t.Fatalf("end = %v, want %v", end, want)
	}
}

func TestResolveOptionalRangeStrictRejectsInvalidAndReversedRanges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start string
		end   string
		want  string
	}{
		{name: "invalid start", start: "bad", want: "invalid start datetime"},
		{name: "invalid end", end: "bad", want: "invalid end datetime"},
		{name: "reversed", start: "2026-07-01", end: "2026-06-30", want: "start datetime must be before or equal to end datetime"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ResolveOptionalRangeStrict(tc.start, tc.end)
			if err == nil {
				t.Fatal("err = nil")
			}
			if err.Error() != tc.want {
				t.Fatalf("err = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}
