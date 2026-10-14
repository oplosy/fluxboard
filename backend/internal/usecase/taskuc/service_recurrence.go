package taskuc

import (
	"context"
	"errors"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/rank"
)

// ---- Recurring tasks (research report P1) -----------------------------------
// A rule on a template task; the worker copies the template into the board's
// to-do column (else its first) on each occurrence date. Reads need VIEWER, writes CONTRIBUTOR.

// RecurrenceInput is the payload for SetRecurrence.
type RecurrenceInput struct {
	Freq     project.RecurrenceFreq
	Weekdays []time.Weekday
	MonthDay int
	Timezone string
	Mode     project.RecurrenceMode
	Paused   bool
}

// GetRecurrence returns the task's rule (VIEWER), or nil if it has none.
func (s *Service) GetRecurrence(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) (*project.Recurrence, error) {
	if s.recurrences == nil {
		return nil, domain.ErrForbidden // recurrence disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	r, err := s.recurrences.Get(ctx, orgID, taskID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return r, err
}

// SetRecurrence creates or edits the task's rule, or pauses / resumes it
// (CONTRIBUTOR). An edit keeps the rule's id and affects future occurrences only.
func (s *Service) SetRecurrence(ctx context.Context, orgID, userID, taskID string, in RecurrenceInput, orgRole tenant.OrgRole) (*project.Recurrence, error) {
	if s.recurrences == nil {
		return nil, domain.ErrForbidden // recurrence disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	r := &project.Recurrence{
		OrgID: orgID, TaskID: taskID, Freq: in.Freq, Weekdays: in.Weekdays, MonthDay: in.MonthDay,
		Timezone: in.Timezone, Mode: in.Mode, Paused: in.Paused,
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	now := s.now()
	existing, err := s.recurrences.Get(ctx, orgID, taskID)
	switch {
	case err == nil:
		r.ID, r.CreatedBy, r.LastOn = existing.ID, existing.CreatedBy, existing.LastOn
		r.Restart(now)
	case errors.Is(err, domain.ErrNotFound):
		// The template itself is today's instance.
		r.ID, r.CreatedBy, r.LastOn = newID(), userID, r.Today(now)
	default:
		return nil, err
	}
	if err := s.recurrences.Upsert(ctx, orgID, r); err != nil {
		return nil, err
	}
	return s.recurrences.Get(ctx, orgID, taskID)
}

// StopRecurrence removes the task's rule (CONTRIBUTOR). Occurrences already
// generated stay as ordinary tasks.
func (s *Service) StopRecurrence(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) error {
	if s.recurrences == nil {
		return domain.ErrForbidden // recurrence disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleContributor); err != nil {
		return err
	}
	return s.recurrences.Delete(ctx, orgID, taskID)
}

// GenerateOccurrences creates every occurrence due now in one org (worker job).
// Safe to repeat or run concurrently: each occurrence's (rule, date) identity is
// unique, so a retry inserts nothing. One rule's failure does not stop the rest;
// their errors are joined and returned after all rules ran.
func (s *Service) GenerateOccurrences(ctx context.Context, orgID string) (int, error) {
	if s.recurrences == nil {
		return 0, nil
	}
	series, err := s.recurrences.ListActive(ctx, orgID)
	if err != nil {
		return 0, err
	}
	now := s.now()
	created := 0
	var errs error
	for i := range series {
		sr := &series[i]
		on, ok := sr.Rule.Due(now, sr.LatestDone)
		if !ok {
			continue
		}
		cols, err := s.columns.ListByBoard(ctx, orgID, sr.BoardID)
		if err != nil || len(cols) == 0 {
			errs = errors.Join(errs, err)
			continue
		}
		col := project.ColumnForRole(cols, project.ColumnTodo)
		if col == nil {
			col = &cols[0]
		}
		existing, err := s.tasks.ListByColumn(ctx, orgID, col.ID)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		last := ""
		if n := len(existing); n > 0 {
			last = existing[n-1].Rank
		}
		due := on // date-only due dates are UTC midnights, as the web sends them
		t := &project.Task{
			ID: newID(), OrgID: orgID, ProjectID: sr.ProjectID, ColumnID: col.ID,
			Title: sr.Title, Description: sr.Description, AssigneeID: sr.AssigneeID, Priority: sr.Priority,
			DueDate: &due, Rank: rank.Append(last), CreatedBy: sr.Rule.CreatedBy,
		}
		ok, err = s.recurrences.CreateOccurrence(ctx, orgID, sr.Rule.ID, on, t)
		if err != nil {
			s.logger.Error("recurrence: occurrence failed", "org", orgID, "rule", sr.Rule.ID, "on", on, "err", err)
			errs = errors.Join(errs, err)
			continue
		}
		if !ok {
			continue
		}
		created++
		s.publish(ctx, orgID, notify.EventTaskCreated, t.CreatedBy, map[string]any{
			"task_id": t.ID, "project_id": t.ProjectID, "column_id": t.ColumnID, "title": t.Title,
		})
		if t.AssigneeID != nil && s.notifier != nil {
			s.notifier.NotifyAssigned(ctx, orgID, t.CreatedBy, t.ID, *t.AssigneeID, t.Title)
		}
	}
	return created, errs
}
