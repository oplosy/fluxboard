package main

import (
	"testing"
	"time"
)

func TestParseClock(t *testing.T) {
	now := time.Date(2026, 10, 12, 15, 4, 5, 0, time.FixedZone("TRT", 3*3600))
	got, err := parseClock("", now)
	if err != nil || !got.Equal(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("default clock = %v, %v", got, err)
	}
	if got, err := parseClock("2026-01-15", now); err != nil || got.Format(time.DateOnly) != "2026-01-15" {
		t.Fatalf("explicit clock = %v, %v", got, err)
	}
	if _, err := parseClock("15/01/2026", now); err == nil {
		t.Fatal("bad clock format accepted")
	}
}
