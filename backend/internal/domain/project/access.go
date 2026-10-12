package project

import (
	"context"
	"errors"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// EffectiveRole is the project-access policy (FR-PROJ-002, docs/05 §2) shared
// by every usecase that reads or writes project data. Org RLS only isolates
// tenants; this is what keeps private projects private inside one:
//
//   - org ADMIN/OWNER is LEAD on every project;
//   - otherwise a project member has their membership role;
//   - otherwise an org MEMBER is VIEWER on an org-visible project;
//   - anyone else cannot see the project: domain.ErrNotFound, so a hidden
//     project is indistinguishable from a missing one.
func EffectiveRole(ctx context.Context, members ProjectMemberRepository, orgID string, p *Project, userID string, orgRole tenant.OrgRole) (ProjectRole, error) {
	scope := ScopeFor(orgRole)
	if scope.All {
		return RoleLead, nil
	}
	m, err := members.Get(ctx, orgID, p.ID, userID)
	if err == nil {
		return m.Role, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	if p.Visibility == VisibilityOrg && scope.OrgVisible {
		return RoleViewer, nil
	}
	return "", domain.ErrNotFound
}

// Scope is which projects an org role sees without a membership row — the
// list/search side of EffectiveRole. Membership rows are always visible on top.
type Scope struct {
	All        bool // every project (org ADMIN/OWNER)
	OrgVisible bool // 'org'-visible projects (org MEMBER+; never GUEST, FR-TEN-010)
}

// ScopeFor returns the project scope of an org role.
func ScopeFor(orgRole tenant.OrgRole) Scope {
	return Scope{All: orgRole.AtLeast(tenant.RoleAdmin), OrgVisible: orgRole.AtLeast(tenant.RoleMember)}
}

// Access loads a project and requires at least min on it, returning the project
// and the caller's effective role. Not visible (or missing) → domain.ErrNotFound;
// visible but below min → domain.ErrForbidden.
func Access(ctx context.Context, projects ProjectRepository, members ProjectMemberRepository, orgID, projectID, userID string, orgRole tenant.OrgRole, min ProjectRole) (*Project, ProjectRole, error) {
	p, err := projects.Get(ctx, orgID, projectID)
	if err != nil {
		return nil, "", err
	}
	role, err := EffectiveRole(ctx, members, orgID, p, userID, orgRole)
	if err != nil {
		return nil, "", err
	}
	if !role.AtLeast(min) {
		return nil, "", domain.ErrForbidden
	}
	return p, role, nil
}
