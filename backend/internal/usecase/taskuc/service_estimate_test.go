package taskuc

import (
	"context"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// estimateTasks stores one task so UpdateTask can load and save it.
type estimateTasks struct {
	project.TaskRepository
	t *project.Task
}

func (f *estimateTasks) Get(_ context.Context, _, _ string) (*project.Task, error) {
	cp := *f.t
	return &cp, nil
}

func (f *estimateTasks) Update(_ context.Context, _ string, t *project.Task) error {
	cp := *t
	f.t = &cp
	return nil
}

type recordActivity struct {
	project.ActivityRepository
	fields []string
}

func (f *recordActivity) Append(_ context.Context, _ string, a *project.Activity) error {
	f.fields = append(f.fields, a.Field)
	return nil
}

func intp(n int) *int { return &n }

func TestUpdateTask_Estimate(t *testing.T) {
	ctx := context.Background()
	tasks := &estimateTasks{t: &project.Task{ID: "t1", ProjectID: "p1", Title: "x", Priority: project.PriorityNone}}
	act := &recordActivity{}
	s := New(Deps{
		Projects: fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  fakeMembers{role: role(project.RoleContributor)},
		Tasks:    tasks, Activity: act,
	})
	update := func(in UpdateTaskInput) (*project.Task, error) {
		in.Title = "x"
		return s.UpdateTask(ctx, "o", "u", "t1", in, tenant.RoleMember)
	}

	got, err := update(UpdateTaskInput{Estimate: intp(5), SetEstimate: true})
	if err != nil || got.Estimate == nil || *got.Estimate != 5 {
		t.Fatalf("set: %+v, %v", got.Estimate, err)
	}
	if len(act.fields) != 1 || act.fields[0] != "estimate" {
		t.Errorf("activity = %v, want [estimate]", act.fields)
	}

	// An update that does not send the estimate keeps it.
	if got, err = update(UpdateTaskInput{Priority: project.PriorityHigh}); err != nil || got.Estimate == nil || *got.Estimate != 5 {
		t.Fatalf("keep: %+v, %v", got.Estimate, err)
	}

	// An explicit nil clears it.
	if got, err = update(UpdateTaskInput{SetEstimate: true}); err != nil || got.Estimate != nil {
		t.Fatalf("clear: %+v, %v", got.Estimate, err)
	}

	for _, bad := range []int{-1, project.MaxEstimate + 1} {
		if _, err := update(UpdateTaskInput{Estimate: intp(bad), SetEstimate: true}); err != domain.ErrValidation {
			t.Errorf("estimate %d: err %v, want ErrValidation", bad, err)
		}
	}
}
