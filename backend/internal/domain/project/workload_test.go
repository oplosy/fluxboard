package project

import (
	"testing"
	"time"
)

func TestComputeWorkload(t *testing.T) {
	day := func(d int) *time.Time {
		v := time.Date(2026, 10, d, 15, 0, 0, 0, time.UTC)
		return &v
	}
	pts := func(n int) *int { return &n }
	ann, bob := "ann", "bob"
	from, to := *day(20), *day(26) // Oct 20..26

	rows := ComputeWorkload([]Task{
		{ID: "due-in", AssigneeID: &ann, ColumnID: "c1", DueDate: day(22), Estimate: pts(3)},
		{ID: "spans", AssigneeID: &ann, ColumnID: "c1", StartDate: day(10), DueDate: day(30)},  // overlaps, unestimated
		{ID: "overdue", AssigneeID: &ann, ColumnID: "c1", DueDate: day(15), Estimate: pts(2)},  // before range
		{ID: "later", AssigneeID: &ann, ColumnID: "c1", StartDate: day(27), Estimate: pts(8)},  // after range
		{ID: "done", AssigneeID: &ann, ColumnID: "done", DueDate: day(22), Estimate: pts(5)},   // ignored
		{ID: "undated", ColumnID: "c1", Estimate: pts(1)},                                      // unassigned, open only
		{ID: "last-day", AssigneeID: &bob, ColumnID: "c1", DueDate: day(26), Estimate: pts(5)}, // to is inclusive
	}, "done", from, to, map[string]int64{"ann": 3600, "cat": 600})

	want := []WorkloadRow{
		{UserID: "ann", OpenTasks: 4, OpenPoints: 13, ScheduledTasks: 2, ScheduledPoints: 3, ScheduledUnestimated: 1, OverdueTasks: 1, LoggedSeconds: 3600},
		{UserID: "bob", OpenTasks: 1, OpenPoints: 5, ScheduledTasks: 1, ScheduledPoints: 5},
		{UserID: "cat", LoggedSeconds: 600},
		{UserID: "", OpenTasks: 1, OpenPoints: 1},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v", rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}
