package projectuc

import (
	"context"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeReleases struct {
	project.ReleaseRepository
	rels map[string]*project.Release
	set  map[string]*string
}

func newFakeReleases(rs ...*project.Release) *fakeReleases {
	m := make(map[string]*project.Release)
	for _, r := range rs {
		m[r.ID] = r
	}
	return &fakeReleases{rels: m, set: make(map[string]*string)}
}

func (f *fakeReleases) Create(_ context.Context, _ string, r *project.Release) error {
	cp := *r
	f.rels[r.ID] = &cp
	return nil
}

func (f *fakeReleases) Get(_ context.Context, _, id string) (*project.Release, error) {
	r, ok := f.rels[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (f *fakeReleases) ListByProject(_ context.Context, _, projectID string) ([]project.Release, error) {
	var out []project.Release
	for _, r := range f.rels {
		if r.ProjectID == projectID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeReleases) Update(_ context.Context, _ string, r *project.Release) error {
	cp := *r
	f.rels[r.ID] = &cp
	return nil
}

func (f *fakeReleases) SetTaskRelease(_ context.Context, _, taskID string, releaseID *string) error {
	f.set[taskID] = releaseID
	return nil
}

func releaseTestService(role project.ProjectRole, rels *fakeReleases, tasks ...*project.Task) *Service {
	return New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: &role},
		Boards:   &fakeSprintBoards{},
		Columns:  &fakeSprintColumns{},
		Tasks:    &fakeSprintTasks{tasks: tasks},
		Releases: rels,
	})
}

func TestListReleases_Progress(t *testing.T) {
	rid := "r1"
	five, three := 5, 3
	rels := newFakeReleases(&project.Release{ID: rid, ProjectID: "p1", Name: "v1"})
	s := releaseTestService(project.RoleViewer, rels,
		&project.Task{ID: "t-done", ProjectID: "p1", ColumnID: "c2", ReleaseID: &rid, Estimate: &five},
		&project.Task{ID: "t-open", ProjectID: "p1", ColumnID: "c1", ReleaseID: &rid, Estimate: &three},
		&project.Task{ID: "t-none", ProjectID: "p1", ColumnID: "c2", Estimate: &five},
	)
	got, err := s.ListReleases(context.Background(), "o", "u", "p1", tenant.RoleMember)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListReleases = %+v, %v", got, err)
	}
	if r := got[0]; r.Total != 2 || r.Done != 1 || r.Points != 8 || r.DonePoints != 5 {
		t.Fatalf("progress = %d/%d tasks, %d/%d pts; want 1/2, 5/8", r.Done, r.Total, r.DonePoints, r.Points)
	}
}

func TestCreateRelease(t *testing.T) {
	ctx := context.Background()
	evening := time.Date(2026, 11, 3, 21, 30, 0, 0, time.FixedZone("UTC-5", -5*3600)) // Nov 4 in UTC

	if _, err := releaseTestService(project.RoleContributor, newFakeReleases()).
		CreateRelease(ctx, "o", "u", "p1", ReleaseInput{Name: "v1"}, tenant.RoleMember); err != domain.ErrForbidden {
		t.Fatalf("contributor: err %v, want ErrForbidden", err)
	}
	s := releaseTestService(project.RoleLead, newFakeReleases())
	if _, err := s.CreateRelease(ctx, "o", "u", "p1", ReleaseInput{Name: "  "}, tenant.RoleMember); err != domain.ErrValidation {
		t.Fatalf("blank name: err %v, want ErrValidation", err)
	}
	r, err := s.CreateRelease(ctx, "o", "u", "p1", ReleaseInput{Name: " v1 ", ReleaseDate: &evening}, tenant.RoleMember)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if r.Name != "v1" || r.ReleasedAt != nil || r.ReleaseDate == nil || r.ReleaseDate.Format(time.DateOnly) != "2026-11-04" {
		t.Errorf("created = %+v", r)
	}
}

func TestUpdateRelease_ReleaseAndReopen(t *testing.T) {
	ctx := context.Background()
	rels := newFakeReleases(&project.Release{ID: "r1", ProjectID: "p1", Name: "v1"})
	s := releaseTestService(project.RoleLead, rels)
	first := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return first }

	r, err := s.UpdateRelease(ctx, "o", "u", "p1", "r1", ReleaseInput{Name: "v1", Released: true}, tenant.RoleMember)
	if err != nil || r.ReleasedAt == nil || !r.ReleasedAt.Equal(first) {
		t.Fatalf("release: %+v, %v", r, err)
	}
	// Saving again while released keeps the original released time.
	s.now = func() time.Time { return first.Add(48 * time.Hour) }
	if r, err = s.UpdateRelease(ctx, "o", "u", "p1", "r1", ReleaseInput{Name: "v1.0", Released: true}, tenant.RoleMember); err != nil || !r.ReleasedAt.Equal(first) {
		t.Fatalf("re-save: %+v, %v", r, err)
	}
	if r, err = s.UpdateRelease(ctx, "o", "u", "p1", "r1", ReleaseInput{Name: "v1.0"}, tenant.RoleMember); err != nil || r.ReleasedAt != nil {
		t.Fatalf("reopen: %+v, %v", r, err)
	}
	if _, err := s.UpdateRelease(ctx, "o", "u", "p2", "r1", ReleaseInput{Name: "x"}, tenant.RoleMember); err != domain.ErrNotFound {
		t.Fatalf("other project: err %v, want ErrNotFound", err)
	}
}

func TestAssignRelease(t *testing.T) {
	ctx := context.Background()
	rid := "r1"
	rels := newFakeReleases(&project.Release{ID: rid, ProjectID: "p1", Name: "v1"})
	s := releaseTestService(project.RoleContributor, rels,
		&project.Task{ID: "t1", ProjectID: "p1"},
		&project.Task{ID: "t-other", ProjectID: "p2"},
	)
	if err := s.AssignRelease(ctx, "o", "u", "p1", []string{"t1", "t-other"}, &rid, tenant.RoleMember); err != domain.ErrValidation {
		t.Fatalf("cross-project: err %v, want ErrValidation", err)
	}
	if len(rels.set) != 0 {
		t.Fatalf("cross-project wrote %v; want nothing", rels.set)
	}
	if err := s.AssignRelease(ctx, "o", "u", "p1", []string{"t1"}, &rid, tenant.RoleMember); err != nil || rels.set["t1"] == nil {
		t.Fatalf("assign: %v, set %v", err, rels.set)
	}
}
