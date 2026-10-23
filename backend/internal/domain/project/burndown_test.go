package project

import (
	"testing"
	"time"
)

func TestComputeBurndown(t *testing.T) {
	at := func(day, hour int) *time.Time {
		v := time.Date(2026, 10, day, hour, 0, 0, 0, time.UTC)
		return &v
	}
	start := *at(5, 9)
	b := ComputeBurndown(start, at(9, 0), *at(8, 15), 13, 3, []BurndownTask{
		{Points: 3, CompletedAt: at(4, 12)}, // done before the sprint started
		{Points: 5, CompletedAt: at(7, 23)},
	})
	if b.TotalPoints != 13 || b.TotalTasks != 3 || !b.Start.Equal(*at(5, 0)) {
		t.Fatalf("header = %+v", b)
	}
	want := []struct{ points, tasks int }{{10, 2}, {10, 2}, {5, 1}, {5, 1}} // Oct 5..8
	if len(b.Days) != len(want) {
		t.Fatalf("days = %d, want %d", len(b.Days), len(want))
	}
	for i, w := range want {
		d := b.Days[i]
		if d.RemainingPoints != w.points || d.RemainingTasks != w.tasks {
			t.Errorf("day %s: %d pts / %d tasks, want %d / %d", d.Date.Format("Jan 2"), d.RemainingPoints, d.RemainingTasks, w.points, w.tasks)
		}
	}

	// last before start still yields the first day; a long sprint is capped.
	if got := ComputeBurndown(start, nil, *at(1, 0), 0, 0, nil); len(got.Days) != 1 {
		t.Errorf("last before start: %d days, want 1", len(got.Days))
	}
	if got := ComputeBurndown(start, nil, start.AddDate(2, 0, 0), 0, 0, nil); len(got.Days) != MaxBurndownDays {
		t.Errorf("long sprint: %d days, want %d", len(got.Days), MaxBurndownDays)
	}
}
