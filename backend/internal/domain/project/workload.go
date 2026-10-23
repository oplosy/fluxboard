package project

import (
	"sort"
	"time"
)

// MaxWorkloadDays caps the date range of a workload view (FR-AN-004).
const MaxWorkloadDays = 92

// WorkloadRow is one person's load in a project; UserID "" is unassigned work.
type WorkloadRow struct {
	UserID string
	// Open*: every task not in the done column, whatever its dates.
	OpenTasks  int
	OpenPoints int
	// Scheduled*: open tasks whose start-to-due window overlaps the range.
	ScheduledTasks       int
	ScheduledPoints      int
	ScheduledUnestimated int
	// OverdueTasks: open tasks due before the range starts.
	OverdueTasks  int
	LoggedSeconds int64 // time logged on the project's tasks in the range
}

// ComputeWorkload sums each assignee's open work for the days from through to
// (both inclusive, UTC). A task with only a start or only a due date is
// scheduled on that day; one with neither is open but never scheduled.
// logged maps user id to seconds; a user who only logged time still gets a row.
// Rows come sorted by user id, unassigned last.
func ComputeWorkload(tasks []Task, doneColumn string, from, to time.Time, logged map[string]int64) []WorkloadRow {
	rangeStart := utcDay(from)
	rangeEnd := utcDay(to).AddDate(0, 0, 1) // exclusive
	rows := map[string]*WorkloadRow{}
	row := func(userID string) *WorkloadRow {
		if r, ok := rows[userID]; ok {
			return r
		}
		r := &WorkloadRow{UserID: userID}
		rows[userID] = r
		return r
	}
	for i := range tasks {
		t := &tasks[i]
		if t.ColumnID == doneColumn {
			continue
		}
		userID := ""
		if t.AssigneeID != nil {
			userID = *t.AssigneeID
		}
		r := row(userID)
		pts := 0
		if t.Estimate != nil {
			pts = *t.Estimate
		}
		r.OpenTasks++
		r.OpenPoints += pts
		if t.DueDate != nil && t.DueDate.Before(rangeStart) {
			r.OverdueTasks++
		}
		start, end := t.StartDate, t.DueDate
		if start == nil {
			start = end
		}
		if end == nil {
			end = start
		}
		if start != nil && start.Before(rangeEnd) && !utcDay(*end).Before(rangeStart) {
			r.ScheduledTasks++
			r.ScheduledPoints += pts
			if t.Estimate == nil {
				r.ScheduledUnestimated++
			}
		}
	}
	for userID, secs := range logged {
		row(userID).LoggedSeconds += secs
	}
	out := make([]WorkloadRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].UserID == "") != (out[j].UserID == "") {
			return out[j].UserID == ""
		}
		return out[i].UserID < out[j].UserID
	})
	return out
}
