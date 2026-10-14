package projectuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
)

type fakeSeeder struct {
	calls int
	last  *project.ProjectSeed
	repo  *fakeProjects
}

func (f *fakeSeeder) Seed(_ context.Context, _ string, s *project.ProjectSeed) error {
	f.calls++
	f.last = s
	p := s.Project
	f.repo.created = &p
	return nil
}

type memIdem map[string]string

func (m memIdem) Get(_ context.Context, k string) (string, error) { return m[k], nil }
func (m memIdem) Set(_ context.Context, k, v string, _ time.Duration) error {
	m[k] = v
	return nil
}

func TestCreateProjectFromTemplate(t *testing.T) {
	projects := &fakeProjects{}
	seeder := &fakeSeeder{repo: projects}
	s := New(Deps{Projects: projects, Seeder: seeder, Idem: memIdem{}})
	ctx := context.Background()
	in := CreateProjectInput{Key: " ops ", Name: "Ops"}

	p, err := s.CreateProjectFromTemplate(ctx, "org", "user", "operations-checklist", in, "k1")
	if err != nil {
		t.Fatal(err)
	}
	tpl, _ := project.TemplateByKey("operations-checklist")
	seed := seeder.last
	if p.Key != "OPS" || seed.LeadID != "user" || len(seed.Columns) != len(tpl.Columns) ||
		len(seed.Labels) != len(tpl.Labels) || len(seed.Tasks) != len(tpl.Tasks) {
		t.Fatalf("seed = %+v", seed)
	}
	for i, c := range seed.Columns {
		if c.Role != tpl.Columns[i].Role || (i > 0 && c.Rank <= seed.Columns[i-1].Rank) {
			t.Fatalf("column %d = %+v", i, c)
		}
	}
	ranks := map[string]bool{}
	for _, task := range seed.Tasks {
		if ranks[task.ColumnID+task.Rank] {
			t.Fatalf("duplicate rank in a column: %+v", task)
		}
		ranks[task.ColumnID+task.Rank] = true
	}

	// Same Idempotency-Key: the first project comes back, nothing is re-seeded.
	again, err := s.CreateProjectFromTemplate(ctx, "org", "user", "operations-checklist", in, "k1")
	if err != nil || again.ID != p.ID || seeder.calls != 1 {
		t.Fatalf("replay: %v, id %s vs %s, calls %d", err, again.ID, p.ID, seeder.calls)
	}

	for _, bad := range []struct{ tpl, key string }{{"nope", "OPS"}, {"operations-checklist", "o"}} {
		_, err := s.CreateProjectFromTemplate(ctx, "org", "user", bad.tpl, CreateProjectInput{Key: bad.key, Name: "x"}, "")
		if !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("%v: err = %v", bad, err)
		}
	}
}
