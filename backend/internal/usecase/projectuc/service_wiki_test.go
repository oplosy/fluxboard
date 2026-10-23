package projectuc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeWiki struct {
	project.WikiRepository
	pages map[string]*project.WikiPage
	nums  map[string][]int
}

func newFakeWiki(ps ...*project.WikiPage) *fakeWiki {
	f := &fakeWiki{pages: map[string]*project.WikiPage{}, nums: map[string][]int{}}
	for _, p := range ps {
		f.pages[p.ID] = p
	}
	return f
}

func (f *fakeWiki) Create(_ context.Context, _ string, p *project.WikiPage, nums []int) error {
	cp := *p
	cp.Version = 1
	f.pages[p.ID], f.nums[p.ID] = &cp, nums
	return nil
}

func (f *fakeWiki) Get(_ context.Context, _, id string) (*project.WikiPage, error) {
	p, ok := f.pages[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakeWiki) ListByProject(_ context.Context, _, _, _ string) ([]project.WikiPage, error) {
	return nil, nil
}

func (f *fakeWiki) Update(_ context.Context, _ string, p *project.WikiPage, v int, nums []int) error {
	cur, ok := f.pages[p.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if cur.Version != v {
		return domain.ErrConflict
	}
	cp := *p
	cp.Version = v + 1
	f.pages[p.ID], f.nums[p.ID] = &cp, nums
	return nil
}

func (f *fakeWiki) Delete(_ context.Context, _, id string) error {
	if _, ok := f.pages[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.pages, id)
	return nil
}

func (f *fakeWiki) TaskRefs(_ context.Context, _, id string) ([]project.WikiTaskRef, error) {
	var out []project.WikiTaskRef
	for _, n := range f.nums[id] {
		out = append(out, project.WikiTaskRef{Number: n})
	}
	return out, nil
}

func wikiTestService(role project.ProjectRole, w *fakeWiki) *Service {
	return New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Key: "WEB", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: &role},
		Wiki:     w,
	})
}

func TestWiki_RoleGates(t *testing.T) {
	ctx := context.Background()
	seed := func() *fakeWiki {
		return newFakeWiki(&project.WikiPage{ID: "pg", ProjectID: "p1", Title: "T", Version: 1, CreatedBy: "author"})
	}
	viewer := wikiTestService(project.RoleViewer, seed())
	if _, err := viewer.GetWikiPage(ctx, "o", "u", "p1", "pg", tenant.RoleMember); err != nil {
		t.Errorf("viewer read: %v", err)
	}
	if _, err := viewer.ListWikiPages(ctx, "o", "u", "p1", "", tenant.RoleMember); err != nil {
		t.Errorf("viewer list: %v", err)
	}
	if _, err := viewer.CreateWikiPage(ctx, "o", "u", "p1", "T", "", tenant.RoleMember); err != domain.ErrForbidden {
		t.Errorf("viewer create: %v, want ErrForbidden", err)
	}
	if _, err := viewer.UpdateWikiPage(ctx, "o", "u", "p1", "pg", "T", "", 1, tenant.RoleMember); err != domain.ErrForbidden {
		t.Errorf("viewer update: %v, want ErrForbidden", err)
	}

	w := seed()
	contrib := wikiTestService(project.RoleContributor, w)
	if _, err := contrib.CreateWikiPage(ctx, "o", "u", "p1", "New", "", tenant.RoleMember); err != nil {
		t.Errorf("contributor create: %v", err)
	}
	if _, err := contrib.UpdateWikiPage(ctx, "o", "u", "p1", "pg", "T2", "", 1, tenant.RoleMember); err != nil {
		t.Errorf("contributor update: %v", err)
	}
	if err := contrib.DeleteWikiPage(ctx, "o", "u", "p1", "pg", tenant.RoleMember); err != domain.ErrForbidden {
		t.Errorf("contributor delete other's page: %v, want ErrForbidden", err)
	}
	if err := contrib.DeleteWikiPage(ctx, "o", "author", "p1", "pg", tenant.RoleMember); err != nil {
		t.Errorf("author delete: %v", err)
	}
	w = seed()
	lead := wikiTestService(project.RoleLead, w)
	if err := lead.DeleteWikiPage(ctx, "o", "u", "p1", "pg", tenant.RoleMember); err != nil {
		t.Errorf("lead delete: %v", err)
	}
	if err := lead.DeleteWikiPage(ctx, "o", "u", "p1", "pg", tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("delete missing: %v, want ErrNotFound", err)
	}
}

func TestWiki_VersionConflict(t *testing.T) {
	ctx := context.Background()
	w := newFakeWiki()
	s := wikiTestService(project.RoleContributor, w)
	v, err := s.CreateWikiPage(ctx, "o", "u", "p1", "Plan", "v1", tenant.RoleMember)
	if err != nil || v.Page.Version != 1 {
		t.Fatalf("create: %+v %v", v, err)
	}
	id := v.Page.ID
	v, err = s.UpdateWikiPage(ctx, "o", "u", "p1", id, "Plan", "v2", 1, tenant.RoleMember)
	if err != nil || v.Page.Version != 2 || v.Page.Body != "v2" {
		t.Fatalf("update: %+v %v", v, err)
	}
	if _, err := s.UpdateWikiPage(ctx, "o", "u", "p1", id, "Plan", "stale", 1, tenant.RoleMember); err != domain.ErrConflict {
		t.Fatalf("stale update: %v, want ErrConflict", err)
	}
	if w.pages[id].Body != "v2" {
		t.Errorf("stale update overwrote the page: %q", w.pages[id].Body)
	}
	if _, err := s.UpdateWikiPage(ctx, "o", "u", "p1", id, "Plan", "x", 0, tenant.RoleMember); err != domain.ErrValidation {
		t.Errorf("missing version: %v, want ErrValidation", err)
	}
}

func TestWiki_MentionIndexAndProject(t *testing.T) {
	ctx := context.Background()
	w := newFakeWiki(&project.WikiPage{ID: "other", ProjectID: "p2", Title: "x", Version: 1})
	s := wikiTestService(project.RoleContributor, w)
	v, err := s.CreateWikiPage(ctx, "o", "u", "p1", "Plan", "WEB-2 WEB-1 OPS-9 web-3 WEB-2", tenant.RoleMember)
	if err != nil || !reflect.DeepEqual(w.nums[v.Page.ID], []int{1, 2}) {
		t.Fatalf("create: nums %v, err %v", w.nums[v.Page.ID], err)
	}
	if _, err := s.UpdateWikiPage(ctx, "o", "u", "p1", v.Page.ID, "Plan", "only WEB-7", 1, tenant.RoleMember); err != nil || !reflect.DeepEqual(w.nums[v.Page.ID], []int{7}) {
		t.Fatalf("update: nums %v, err %v", w.nums[v.Page.ID], err)
	}
	// A page of another project is not reachable through this project's routes.
	if _, err := s.GetWikiPage(ctx, "o", "u", "p1", "other", tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("get other project's page: %v, want ErrNotFound", err)
	}
	if _, err := s.UpdateWikiPage(ctx, "o", "u", "p1", "other", "x", "", 1, tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("update other project's page: %v, want ErrNotFound", err)
	}
	if err := s.DeleteWikiPage(ctx, "o", "u", "p1", "other", tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("delete other project's page: %v, want ErrNotFound", err)
	}
}

func TestWiki_BodyBound(t *testing.T) {
	s := wikiTestService(project.RoleContributor, newFakeWiki())
	big := strings.Repeat("a", project.MaxWikiBodyLen+1)
	if _, err := s.CreateWikiPage(context.Background(), "o", "u", "p1", "T", big, tenant.RoleMember); err != domain.ErrValidation {
		t.Fatalf("oversized body: %v, want ErrValidation", err)
	}
}
