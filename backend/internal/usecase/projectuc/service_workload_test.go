package projectuc

import (
	"context"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// fakeTimeLog records the window it was asked for and returns fixed sums.
type fakeTimeLog struct {
	project.TimeEntryRepository
	from, to time.Time
}

func (f *fakeTimeLog) SumByUser(_ context.Context, _, _ string, from, to time.Time) (map[string]int64, error) {
	f.from, f.to = from, to
	return map[string]int64{"ann": 5400}, nil
}

func TestWorkload(t *testing.T) {
	ctx := context.Background()
	ann := "ann"
	due := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	from := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

	viewer := project.RoleViewer
	tl := &fakeTimeLog{}
	s := New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: &viewer},
		Boards:   &fakeSprintBoards{},
		Columns:  &fakeSprintColumns{},
		Tasks: &fakeSprintTasks{tasks: []*project.Task{
			{ID: "t1", ProjectID: "p1", ColumnID: "c1", AssigneeID: &ann, DueDate: &due},
			{ID: "t2", ProjectID: "p1", ColumnID: "c2", AssigneeID: &ann, DueDate: &due}, // done column
		}},
		Time: tl,
	})

	rows, err := s.Workload(ctx, "o", "u", "p1", from, to, tenant.RoleMember)
	if err != nil || len(rows) != 1 {
		t.Fatalf("Workload = %+v, %v", rows, err)
	}
	if r := rows[0]; r.OpenTasks != 1 || r.ScheduledTasks != 1 || r.LoggedSeconds != 5400 {
		t.Errorf("row = %+v", r)
	}
	// Time is summed over whole days: to is inclusive.
	if !tl.from.Equal(from) || !tl.to.Equal(to.AddDate(0, 0, 1)) {
		t.Errorf("time window = %v..%v", tl.from, tl.to)
	}

	for name, rng := range map[string][2]time.Time{
		"reversed": {to, from},
		"too long": {from, from.AddDate(0, 0, project.MaxWorkloadDays)},
	} {
		if _, err := s.Workload(ctx, "o", "u", "p1", rng[0], rng[1], tenant.RoleMember); err != domain.ErrValidation {
			t.Errorf("%s: err %v, want ErrValidation", name, err)
		}
	}
	if _, err := s.Workload(ctx, "o", "u", "p1", from, from.AddDate(0, 0, project.MaxWorkloadDays-1), tenant.RoleMember); err != nil {
		t.Errorf("92-day range: %v", err)
	}
}
