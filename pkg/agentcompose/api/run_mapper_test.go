package api

import (
	"testing"
	"time"
)

func TestFormatProjectTimePreservesInstantAndNanosecondPrecision(t *testing.T) {
	t.Parallel()
	zone := time.FixedZone("UTC+08:30", 8*60*60+30*60)
	input := time.Date(2026, 7, 23, 12, 34, 56, 123456789, zone)

	got := FormatProjectTime(input)
	if err := got.CheckValid(); err != nil {
		t.Fatalf("FormatProjectTime() returned invalid timestamp: %v", err)
	}
	if !got.AsTime().Equal(input) {
		t.Fatalf("FormatProjectTime() = %s, want instant %s", got.AsTime(), input)
	}
	if got.AsTime().Nanosecond() != input.Nanosecond() {
		t.Fatalf("nanoseconds = %d, want %d", got.AsTime().Nanosecond(), input.Nanosecond())
	}
}

func TestFormatProjectTimeOmitsZeroValue(t *testing.T) {
	t.Parallel()
	if got := FormatProjectTime(time.Time{}); got != nil {
		t.Fatalf("FormatProjectTime(zero) = %v, want nil", got)
	}
}
