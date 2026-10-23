package project

import (
	"strings"
	"testing"
	"time"
)

func TestDeriveGoalStatus(t *testing.T) {
	now := time.Date(2026, 6, 10, 15, 0, 0, 0, time.UTC)
	day := func(d int) *time.Time { v := time.Date(2026, 6, d, 0, 0, 0, 0, time.UTC); return &v }
	cases := []struct {
		name        string
		total, done int
		target      *time.Time
		want        GoalStatus
	}{
		{"no tasks no date", 0, 0, nil, GoalOnTrack},
		{"empty goal is never done", 0, 0, day(1), GoalAtRisk},
		{"partial before date", 4, 1, day(20), GoalOnTrack},
		{"due today is not late", 4, 1, day(10), GoalOnTrack},
		{"partial after date", 4, 1, day(9), GoalAtRisk},
		{"all done after date", 4, 4, day(1), GoalDone},
	}
	for _, c := range cases {
		if got := DeriveGoalStatus(c.total, c.done, c.target, now); got != c.want {
			t.Errorf("%s: got %s want %s", c.name, got, c.want)
		}
	}
}

func TestNormalizeGoal(t *testing.T) {
	if _, _, err := NormalizeGoal("  ", ""); err == nil {
		t.Error("blank title accepted")
	}
	if _, _, err := NormalizeGoal(strings.Repeat("x", MaxGoalTitleLen+1), ""); err == nil {
		t.Error("long title accepted")
	}
	if _, _, err := NormalizeGoal("ok", strings.Repeat("x", MaxGoalDescriptionLen+1)); err == nil {
		t.Error("long description accepted")
	}
	if title, _, err := NormalizeGoal(" Ship v2 ", ""); err != nil || title != "Ship v2" {
		t.Errorf("got %q %v", title, err)
	}
}
