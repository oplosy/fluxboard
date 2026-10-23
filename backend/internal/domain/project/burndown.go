package project

import "time"

// BurndownTask is one sprint task as the burndown sees it (FR-AN-003).
type BurndownTask struct {
	Points      int        // 0 when not estimated
	CompletedAt *time.Time // set while the task sits in the done column
}

// BurndownDay is the work still open at the end of one UTC day.
type BurndownDay struct {
	Date            time.Time // midnight UTC
	RemainingPoints int
	RemainingTasks  int
}

// Burndown is a sprint's remaining work per day. Start/End bound the ideal
// line; End is nil when the sprint has no planned end.
type Burndown struct {
	Start       time.Time
	End         *time.Time
	TotalPoints int
	TotalTasks  int
	Days        []BurndownDay
}

// MaxBurndownDays caps the series so a sprint left open for months stays small.
const MaxBurndownDays = 180

func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ComputeBurndown lists remaining work for each day from start through last
// (both inclusive). A task counts as done from the day it was completed; one
// completed before start counts as done from the first day. Scope is today's:
// tasks added or removed mid-sprint are not replayed.
func ComputeBurndown(start time.Time, end *time.Time, last time.Time, totalPoints, totalTasks int, done []BurndownTask) Burndown {
	b := Burndown{Start: utcDay(start), End: end, TotalPoints: totalPoints, TotalTasks: totalTasks}
	lastDay := utcDay(last)
	if lastDay.Before(b.Start) {
		lastDay = b.Start
	}
	if limit := b.Start.AddDate(0, 0, MaxBurndownDays-1); lastDay.After(limit) {
		lastDay = limit
	}
	for d := b.Start; !d.After(lastDay); d = d.AddDate(0, 0, 1) {
		next := d.AddDate(0, 0, 1)
		day := BurndownDay{Date: d, RemainingPoints: totalPoints, RemainingTasks: totalTasks}
		for _, t := range done {
			if t.CompletedAt != nil && t.CompletedAt.Before(next) {
				day.RemainingPoints -= t.Points
				day.RemainingTasks--
			}
		}
		b.Days = append(b.Days, day)
	}
	return b
}
