package projectuc

import (
	"context"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// burndownSprints adds the sprint's burndown rows to fakeSprints.
type burndownSprints struct {
	*fakeSprints
	rows []project.BurndownTask
}

func (f *burndownSprints) BurndownTasks(_ context.Context, _, _ string) ([]project.BurndownTask, error) {
	return f.rows, nil
}

func TestCompleteSprint_SnapshotsPoints(t *testing.T) {
	sid := "s1"
	five, three := 5, 3
	sprints := newFakeSprints(&project.Sprint{ID: sid, ProjectID: "p1", Status: project.SprintActive})
	s := sprintTestService(sprints,
		&project.Task{ID: "t-done", ProjectID: "p1", ColumnID: "c2", SprintID: &sid, Estimate: &five},
		&project.Task{ID: "t-open", ProjectID: "p1", ColumnID: "c1", SprintID: &sid, Estimate: &three},
		&project.Task{ID: "t-unest", ProjectID: "p1", ColumnID: "c1", SprintID: &sid},
	)
	got, err := s.CompleteSprint(context.Background(), "o", "u", "p1", sid, tenant.RoleMember)
	if err != nil {
		t.Fatalf("CompleteSprint: %v", err)
	}
	if got.CompletedPointsTotal != 8 || got.CompletedPointsDone != 5 {
		t.Errorf("points = %d/%d, want 5/8", got.CompletedPointsDone, got.CompletedPointsTotal)
	}
}

func TestSprintBurndown(t *testing.T) {
	ctx := context.Background()
	day := func(d int) *time.Time {
		v := time.Date(2026, 10, d, 10, 0, 0, 0, time.UTC)
		return &v
	}
	rows := []project.BurndownTask{{Points: 5, CompletedAt: day(6)}, {Points: 3}}
	svc := func(sp *project.Sprint) *Service {
		s := sprintTestService(nil)
		s.sprints = &burndownSprints{fakeSprints: newFakeSprints(sp), rows: rows}
		s.now = func() time.Time { return *day(7) }
		return s
	}

	planned := &project.Sprint{ID: "s1", ProjectID: "p1", Status: project.SprintPlanned}
	if _, err := svc(planned).SprintBurndown(ctx, "o", "u", "p1", "s1", tenant.RoleMember); err != domain.ErrConflict {
		t.Fatalf("planned: err %v, want ErrConflict", err)
	}

	// Active: scope is the current tasks, the series runs to today.
	active := &project.Sprint{ID: "s1", ProjectID: "p1", Status: project.SprintActive, StartedAt: day(5)}
	b, err := svc(active).SprintBurndown(ctx, "o", "u", "p1", "s1", tenant.RoleMember)
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if b.TotalPoints != 8 || b.TotalTasks != 2 || len(b.Days) != 3 {
		t.Fatalf("active = %d pts, %d tasks, %d days; want 8, 2, 3", b.TotalPoints, b.TotalTasks, len(b.Days))
	}
	if last := b.Days[2]; last.RemainingPoints != 3 || last.RemainingTasks != 1 {
		t.Errorf("active last day = %+v, want 3 pts / 1 task", last)
	}

	// Completed: scope is the snapshot, the series runs to the end date.
	done := &project.Sprint{ID: "s1", ProjectID: "p1", Status: project.SprintCompleted, StartedAt: day(5), EndedAt: day(6),
		CompletedTotal: 4, CompletedPointsTotal: 13}
	b, err = svc(done).SprintBurndown(ctx, "o", "u", "p1", "s1", tenant.RoleMember)
	if err != nil {
		t.Fatalf("completed: %v", err)
	}
	if b.TotalPoints != 13 || b.TotalTasks != 4 || len(b.Days) != 2 || b.Days[1].RemainingPoints != 8 {
		t.Errorf("completed = %+v", b)
	}
}
