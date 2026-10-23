package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// DashboardRepo is the Postgres-backed analytics.DashboardRepository ([T], RLS).
type DashboardRepo struct{ tp *TenantPool }

// NewDashboardRepo builds a DashboardRepo over the tenant pool.
func NewDashboardRepo(tp *TenantPool) *DashboardRepo { return &DashboardRepo{tp: tp} }

var _ analytics.DashboardRepository = (*DashboardRepo)(nil)

func (r *DashboardRepo) Counts(ctx context.Context, orgID string, projectIDs []string, now, weekFrom time.Time) ([]analytics.ProjectCounts, []analytics.AssigneeCount, []analytics.WeeklyCompleted, error) {
	ids := make([]uuid.UUID, 0, len(projectIDs))
	for _, s := range projectIDs {
		id, err := parseUUID(s)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("dashboard counts: project: %w", err)
		}
		ids = append(ids, id)
	}
	projects := make([]analytics.ProjectCounts, 0, len(ids))
	assignees := make([]analytics.AssigneeCount, 0)
	weekly := make([]analytics.WeeklyCompleted, 0)
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		nowTS := pgtype.Timestamptz{Time: now, Valid: true}
		pRows, err := q.DashboardProjectCounts(ctx, gen.DashboardProjectCountsParams{Now: nowTS, OrgID: oid, ProjectIds: ids})
		if err != nil {
			return err
		}
		for _, row := range pRows {
			projects = append(projects, analytics.ProjectCounts{
				ProjectID: row.ProjectID.String(), Open: int(row.OpenCount), Done: int(row.DoneCount), Overdue: int(row.OverdueCount),
			})
		}
		aRows, err := q.DashboardAssigneeCounts(ctx, gen.DashboardAssigneeCountsParams{Now: nowTS, OrgID: oid, ProjectIds: ids})
		if err != nil {
			return err
		}
		for _, row := range aRows {
			a := analytics.AssigneeCount{ProjectID: row.ProjectID.String(), Open: int(row.OpenCount), Overdue: int(row.OverdueCount)}
			if row.AssigneeID.Valid {
				a.UserID = uuid.UUID(row.AssigneeID.Bytes).String()
			}
			assignees = append(assignees, a)
		}
		wRows, err := q.DashboardWeeklyCompleted(ctx, gen.DashboardWeeklyCompletedParams{OrgID: oid, ProjectIds: ids, FromDay: weekFrom})
		if err != nil {
			return err
		}
		for _, row := range wRows {
			weekly = append(weekly, analytics.WeeklyCompleted{ProjectID: row.ProjectID.String(), Week: row.Week, Completed: int(row.Completed)})
		}
		return nil
	})
	return projects, assignees, weekly, err
}

func (r *DashboardRepo) GetLayout(ctx context.Context, orgID, userID string) ([]analytics.Widget, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("%w: dashboard layout needs a user principal", domain.ErrForbidden)
	}
	var out []analytics.Widget
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		raw, err := q.GetDashboardLayout(ctx, gen.GetDashboardLayoutParams{OrgID: oid, UserID: uid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return fmt.Errorf("dashboard layout %s: %w", userID, err)
		}
		return nil
	})
	return out, err
}

func (r *DashboardRepo) SaveLayout(ctx context.Context, orgID, userID string, widgets []analytics.Widget) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return fmt.Errorf("%w: dashboard layout needs a user principal", domain.ErrForbidden)
	}
	raw, err := json.Marshal(widgets)
	if err != nil {
		return fmt.Errorf("dashboard layout save: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.UpsertDashboardLayout(ctx, gen.UpsertDashboardLayoutParams{OrgID: oid, UserID: uid, Widgets: raw})
	})
}
