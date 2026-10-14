package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// RecurrenceRepo is the Postgres-backed project.RecurrenceRepository ([T], RLS).
type RecurrenceRepo struct{ tp *TenantPool }

// NewRecurrenceRepo builds a RecurrenceRepo over the tenant pool.
func NewRecurrenceRepo(tp *TenantPool) *RecurrenceRepo { return &RecurrenceRepo{tp: tp} }

var _ project.RecurrenceRepository = (*RecurrenceRepo)(nil)

func weekdaysFrom(ds []int16) []time.Weekday {
	out := make([]time.Weekday, 0, len(ds))
	for _, d := range ds {
		out = append(out, time.Weekday(d))
	}
	return out
}

func monthDayFrom(d *int16) int {
	if d == nil {
		return 0
	}
	return int(*d)
}

func (r *RecurrenceRepo) Get(ctx context.Context, orgID, taskID string) (*project.Recurrence, error) {
	tid, err := parseUUID(taskID)
	if err != nil {
		return nil, fmt.Errorf("recurrence get: %w", err)
	}
	var out *project.Recurrence
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetTaskRecurrence(ctx, gen.GetTaskRecurrenceParams{OrgID: oid, TaskID: tid})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = &project.Recurrence{
			ID: row.ID.String(), OrgID: row.OrgID.String(), TaskID: row.TaskID.String(),
			Freq: project.RecurrenceFreq(row.Freq), Weekdays: weekdaysFrom(row.Weekdays),
			MonthDay: monthDayFrom(row.MonthDay), Timezone: row.Timezone,
			Mode: project.RecurrenceMode(row.Mode), Paused: row.Paused, LastOn: row.LastOn,
			CreatedBy: row.CreatedBy.String(),
		}
		return nil
	})
	return out, err
}

func (r *RecurrenceRepo) Upsert(ctx context.Context, orgID string, rec *project.Recurrence) error {
	id, err := parseUUID(rec.ID)
	if err != nil {
		return fmt.Errorf("recurrence upsert: id: %w", err)
	}
	tid, err := parseUUID(rec.TaskID)
	if err != nil {
		return fmt.Errorf("recurrence upsert: task: %w", err)
	}
	createdBy, err := parseUUID(rec.CreatedBy)
	if err != nil {
		return fmt.Errorf("recurrence upsert: created_by: %w", err)
	}
	days := make([]int16, 0, len(rec.Weekdays))
	for _, d := range rec.Weekdays {
		days = append(days, int16(d)) //nolint:gosec // Validate bounds it to 0–6
	}
	var monthDay *int16
	if rec.Freq == project.FreqMonthly {
		md := int16(rec.MonthDay) //nolint:gosec // Validate bounds it to 1–31
		monthDay = &md
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.UpsertTaskRecurrence(ctx, gen.UpsertTaskRecurrenceParams{
			ID: id, OrgID: oid, TaskID: tid, Freq: string(rec.Freq), Weekdays: days, MonthDay: monthDay,
			Timezone: rec.Timezone, Mode: string(rec.Mode), Paused: rec.Paused, LastOn: rec.LastOn,
			CreatedBy: createdBy,
		})
	})
}

func (r *RecurrenceRepo) Delete(ctx context.Context, orgID, taskID string) error {
	tid, err := parseUUID(taskID)
	if err != nil {
		return fmt.Errorf("recurrence delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteTaskRecurrence(ctx, gen.DeleteTaskRecurrenceParams{OrgID: oid, TaskID: tid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *RecurrenceRepo) ListActive(ctx context.Context, orgID string) ([]project.RecurrenceSeries, error) {
	var out []project.RecurrenceSeries
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListActiveRecurrences(ctx, oid)
		if err != nil {
			return err
		}
		out = make([]project.RecurrenceSeries, 0, len(rows))
		for i := range rows {
			row := &rows[i]
			done := tsPtr(row.LatestCompletedAt)
			if done == nil {
				done = tsPtr(row.LatestDeletedAt)
			}
			out = append(out, project.RecurrenceSeries{
				Rule: project.Recurrence{
					ID: row.ID.String(), OrgID: orgID, TaskID: row.TaskID.String(),
					Freq: project.RecurrenceFreq(row.Freq), Weekdays: weekdaysFrom(row.Weekdays),
					MonthDay: monthDayFrom(row.MonthDay), Timezone: row.Timezone,
					Mode: project.RecurrenceMode(row.Mode), LastOn: row.LastOn, CreatedBy: row.CreatedBy.String(),
				},
				ProjectID: row.ProjectID.String(), BoardID: row.BoardID.String(),
				Title: row.Title, Description: row.Description, AssigneeID: uuidStrPtr(row.AssigneeID),
				Priority: project.Priority(row.Priority), LatestDone: done,
			})
		}
		return nil
	})
	return out, err
}

func (r *RecurrenceRepo) CreateOccurrence(ctx context.Context, orgID, ruleID string, on time.Time, t *project.Task) (bool, error) {
	rid, err := parseUUID(ruleID)
	if err != nil {
		return false, fmt.Errorf("occurrence create: rule: %w", err)
	}
	id, err := parseUUID(t.ID)
	if err != nil {
		return false, fmt.Errorf("occurrence create: id: %w", err)
	}
	pid, err := parseUUID(t.ProjectID)
	if err != nil {
		return false, fmt.Errorf("occurrence create: project: %w", err)
	}
	cid, err := parseUUID(t.ColumnID)
	if err != nil {
		return false, fmt.Errorf("occurrence create: column: %w", err)
	}
	createdBy, err := parseUUID(t.CreatedBy)
	if err != nil {
		return false, fmt.Errorf("occurrence create: created_by: %w", err)
	}
	assignee, err := nullableUUID(t.AssigneeID)
	if err != nil {
		return false, fmt.Errorf("occurrence create: assignee: %w", err)
	}
	created := false
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		lock, err := q.LockRecurrence(ctx, gen.LockRecurrenceParams{OrgID: oid, ID: rid})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // stopped meanwhile
		}
		if err != nil {
			return err
		}
		if lock.Paused || !on.After(lock.LastOn) {
			return nil
		}
		num, err := q.NextTaskNumber(ctx, gen.NextTaskNumberParams{OrgID: oid, ID: pid})
		if err != nil {
			return err
		}
		n, err := q.CreateOccurrenceTask(ctx, gen.CreateOccurrenceTaskParams{
			ID: id, OrgID: oid, ProjectID: pid, ColumnID: cid, Number: num,
			Title: t.Title, Description: t.Description, AssigneeID: assignee,
			Priority: string(t.Priority), DueDate: nullTS(t.DueDate), Rank: t.Rank, CreatedBy: createdBy,
			RecurrenceID: pgtype.UUID{Bytes: rid, Valid: true}, OccurrenceOn: on,
		})
		if err != nil {
			return err
		}
		created, t.Number = n == 1, int(num)
		return q.AdvanceRecurrence(ctx, gen.AdvanceRecurrenceParams{OrgID: oid, ID: rid, LastOn: on})
	})
	if isUnique(err) {
		return false, domain.ErrConflict
	}
	return created, err
}
