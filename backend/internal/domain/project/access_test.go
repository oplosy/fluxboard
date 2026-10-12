package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeProjects map[string]*Project

func (f fakeProjects) Get(_ context.Context, _, id string) (*Project, error) {
	if p, ok := f[id]; ok {
		return p, nil
	}
	return nil, domain.ErrNotFound
}
func (fakeProjects) Create(context.Context, string, *Project) error { return nil }
func (fakeProjects) List(context.Context, string, string, bool, bool) ([]Project, error) {
	return nil, nil
}
func (fakeProjects) Update(context.Context, string, *Project) error                { return nil }
func (fakeProjects) SetArchived(context.Context, string, string, *time.Time) error { return nil }
func (fakeProjects) NextNumber(context.Context, string, string) (int, error)       { return 1, nil }

// fakeMembers maps projectID/userID to a role; err, when set, is returned for
// every lookup (a repository failure).
type fakeMembers struct {
	roles map[string]ProjectRole
	err   error
}

func (f fakeMembers) Get(_ context.Context, _, projectID, userID string) (*ProjectMember, error) {
	if f.err != nil {
		return nil, f.err
	}
	if r, ok := f.roles[projectID+"/"+userID]; ok {
		return &ProjectMember{Role: r}, nil
	}
	return nil, domain.ErrNotFound
}
func (fakeMembers) Add(context.Context, string, string, string, ProjectRole) error { return nil }
func (fakeMembers) List(context.Context, string, string) ([]ProjectMember, error) {
	return nil, nil
}
func (fakeMembers) Remove(context.Context, string, string, string) error { return nil }

func TestEffectiveRole(t *testing.T) {
	open := &Project{ID: "open", Visibility: VisibilityOrg}
	private := &Project{ID: "private", Visibility: VisibilityPrivate}
	members := fakeMembers{roles: map[string]ProjectRole{
		"open/viewer": RoleViewer, "private/contrib": RoleContributor, "open/guest": RoleContributor,
	}}
	cases := []struct {
		name    string
		p       *Project
		user    string
		orgRole tenant.OrgRole
		want    ProjectRole
		wantErr error
	}{
		{"owner is lead on a private project", private, "x", tenant.RoleOwner, RoleLead, nil},
		{"admin is lead on a private project", private, "x", tenant.RoleAdmin, RoleLead, nil},
		{"member row wins over org visibility", open, "viewer", tenant.RoleMember, RoleViewer, nil},
		{"private project member", private, "contrib", tenant.RoleMember, RoleContributor, nil},
		{"org member without a row sees an open project", open, "x", tenant.RoleMember, RoleViewer, nil},
		{"guest with a row has it", open, "guest", tenant.RoleGuest, RoleContributor, nil},
		{"guest without a row sees nothing", open, "x", tenant.RoleGuest, "", domain.ErrNotFound},
		{"member without a row on a private project", private, "x", tenant.RoleMember, "", domain.ErrNotFound},
	}
	for _, c := range cases {
		got, err := EffectiveRole(context.Background(), members, "org", c.p, c.user, c.orgRole)
		if got != c.want || !errors.Is(err, c.wantErr) {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, got, err, c.want, c.wantErr)
		}
	}

	boom := errors.New("db down")
	if _, err := EffectiveRole(context.Background(), fakeMembers{err: boom}, "org", open, "x", tenant.RoleMember); !errors.Is(err, boom) {
		t.Fatalf("repository error must propagate, got %v", err)
	}
}

func TestAccess(t *testing.T) {
	projects := fakeProjects{"open": {ID: "open", Visibility: VisibilityOrg}}
	members := fakeMembers{}
	ctx := context.Background()

	p, role, err := Access(ctx, projects, members, "org", "open", "x", tenant.RoleMember, RoleViewer)
	if err != nil || p.ID != "open" || role != RoleViewer {
		t.Fatalf("viewer access = (%v, %q, %v)", p, role, err)
	}
	if _, _, err := Access(ctx, projects, members, "org", "open", "x", tenant.RoleMember, RoleContributor); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("below min: err = %v, want ErrForbidden", err)
	}
	if _, _, err := Access(ctx, projects, members, "org", "missing", "x", tenant.RoleAdmin, RoleViewer); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing project: err = %v, want ErrNotFound", err)
	}
}
