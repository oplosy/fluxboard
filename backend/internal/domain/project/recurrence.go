package project

import (
	"context"
	"slices"
	"time"
	_ "time/tzdata" // rules name IANA zones; the runtime image may ship no zoneinfo

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// RecurrenceFreq is how often a recurring task repeats.
type RecurrenceFreq string

// Recurrence frequencies.
const (
	FreqDaily   RecurrenceFreq = "daily"
	FreqWeekly  RecurrenceFreq = "weekly"  // on Weekdays
	FreqMonthly RecurrenceFreq = "monthly" // on MonthDay, clamped to the month's end
)

// RecurrenceMode picks what the next occurrence counts from.
type RecurrenceMode string

// Recurrence modes.
const (
	// ModeSchedule follows the calendar whatever happens to earlier occurrences.
	ModeSchedule RecurrenceMode = "schedule"
	// ModeAfterCompletion waits for the latest occurrence to be completed (or
	// trashed) and repeats on the first matching date after that.
	ModeAfterCompletion RecurrenceMode = "after_completion"
)

// Recurrence is a repeat rule on a template task. Dates are civil dates in
// Timezone, held as UTC midnights, so DST changes never shift or repeat one.
type Recurrence struct {
	ID        string
	OrgID     string
	TaskID    string // the template task
	Freq      RecurrenceFreq
	Weekdays  []time.Weekday // weekly
	MonthDay  int            // monthly: 1–31
	Timezone  string         // IANA name
	Mode      RecurrenceMode
	Paused    bool
	LastOn    time.Time // newest date generated, set, resumed or edited; never generated again
	CreatedBy string
}

// Validate checks the rule's shape.
func (r *Recurrence) Validate() error {
	if _, err := time.LoadLocation(r.Timezone); err != nil || r.Timezone == "" {
		return domain.ErrValidation
	}
	if r.Mode != ModeSchedule && r.Mode != ModeAfterCompletion {
		return domain.ErrValidation
	}
	switch r.Freq {
	case FreqDaily:
	case FreqWeekly:
		if len(r.Weekdays) == 0 {
			return domain.ErrValidation
		}
		for _, d := range r.Weekdays {
			if d < time.Sunday || d > time.Saturday {
				return domain.ErrValidation
			}
		}
	case FreqMonthly:
		if r.MonthDay < 1 || r.MonthDay > 31 {
			return domain.ErrValidation
		}
	default:
		return domain.ErrValidation
	}
	return nil
}

// CivilDate is t's calendar date in loc, as a UTC midnight.
func CivilDate(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Today is now's civil date in the rule's timezone (UTC if the zone is unknown,
// which Validate rules out for stored rules).
func (r *Recurrence) Today(now time.Time) time.Time {
	loc, err := time.LoadLocation(r.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return CivilDate(now, loc)
}

// Next is the first occurrence date strictly after the civil date after.
func (r *Recurrence) Next(after time.Time) time.Time {
	switch r.Freq {
	case FreqWeekly:
		for i := 1; i <= 7; i++ {
			if d := after.AddDate(0, 0, i); slices.Contains(r.Weekdays, d.Weekday()) {
				return d
			}
		}
	case FreqMonthly:
		if d := r.monthDay(after.Year(), after.Month()); d.After(after) {
			return d
		}
		next := time.Date(after.Year(), after.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		return r.monthDay(next.Year(), next.Month())
	}
	return after.AddDate(0, 0, 1) // daily; also an unvalidated weekly rule
}

// monthDay is MonthDay in the given month, clamped to its last day (31 → Feb 28/29).
func (r *Recurrence) monthDay(y int, m time.Month) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(y, m, min(r.MonthDay, last), 0, 0, 0, 0, time.UTC)
}

// Due is the occurrence date to generate at now, if any. A date is due once its
// day has begun in the rule's timezone. Schedule mode returns the newest due
// date after LastOn, skipping older missed ones (a worker outage yields one
// task, not a backlog). After-completion mode needs the latest instance's
// completion and returns the first date after it (and after LastOn).
func (r *Recurrence) Due(now time.Time, latestDone *time.Time) (time.Time, bool) {
	loc, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return time.Time{}, false
	}
	today := CivilDate(now, loc)
	from := r.LastOn
	if r.Mode == ModeAfterCompletion {
		if latestDone == nil {
			return time.Time{}, false
		}
		if done := CivilDate(*latestDone, loc); done.After(from) {
			from = done
		}
	}
	d := r.Next(from)
	if d.After(today) {
		return time.Time{}, false
	}
	if r.Mode == ModeSchedule {
		for n := r.Next(d); !n.After(today); n = r.Next(n) {
			d = n
		}
	}
	return d, true
}

// Restart moves LastOn up to yesterday so an edit or a resume never back-fills
// dates that passed while the rule was paused or different; today stays due.
func (r *Recurrence) Restart(now time.Time) {
	if y := r.Today(now).AddDate(0, 0, -1); y.After(r.LastOn) {
		r.LastOn = y
	}
}

// RecurrenceSeries is an active rule with what the generator needs to copy the
// template into an occurrence.
type RecurrenceSeries struct {
	Rule        Recurrence
	ProjectID   string
	BoardID     string // the template's board
	Title       string
	Description string
	AssigneeID  *string
	Priority    Priority
	// LatestDone is when the series' latest instance was completed or trashed;
	// nil while it is open.
	LatestDone *time.Time
}

// RecurrenceRepository persists recurrence rules and their occurrences.
type RecurrenceRepository interface {
	// Get returns the rule on a template task. ErrNotFound if none.
	Get(ctx context.Context, orgID, taskID string) (*Recurrence, error)
	// Upsert creates the task's rule or replaces it in place (keeping its id).
	Upsert(ctx context.Context, orgID string, r *Recurrence) error
	// Delete removes the task's rule; its occurrences stay. ErrNotFound if none.
	Delete(ctx context.Context, orgID, taskID string) error
	// ListActive returns unpaused rules whose template is live.
	ListActive(ctx context.Context, orgID string) ([]RecurrenceSeries, error)
	// CreateOccurrence inserts t as the rule's occurrence on date and advances
	// LastOn, in one transaction under the rule's row lock. It reports false and
	// inserts nothing when the rule is paused or gone, the date is not after
	// LastOn, or that occurrence already exists. The repo assigns t.Number.
	CreateOccurrence(ctx context.Context, orgID, ruleID string, on time.Time, t *Task) (bool, error)
}
