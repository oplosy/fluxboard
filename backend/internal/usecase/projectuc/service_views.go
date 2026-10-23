package projectuc

import (
	"context"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Saved board views (FR-PROJ-011) ----------------------------------------
// Any VIEWER keeps personal views; only a LEAD shares one with the project or
// changes a shared one. Another user's personal view behaves as absent.

// ListViews returns the project's shared views and the caller's own (VIEWER).
func (s *Service) ListViews(ctx context.Context, orgID, userID, projectID string, orgRole tenant.OrgRole) ([]project.SavedView, error) {
	if s.views == nil {
		return nil, domain.ErrForbidden // saved views disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	return s.views.ListVisible(ctx, orgID, projectID, userID)
}

// CreateView saves a view; shared needs LEAD, personal needs VIEWER.
func (s *Service) CreateView(ctx context.Context, orgID, userID, projectID, name string, shared bool, f project.ViewFilters, orgRole tenant.OrgRole) (*project.SavedView, error) {
	if s.views == nil {
		return nil, domain.ErrForbidden // saved views disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, viewRole(shared)); err != nil {
		return nil, err
	}
	name, f, err := project.NormalizeSavedView(name, f)
	if err != nil {
		return nil, err
	}
	v := &project.SavedView{ID: newID(), OrgID: orgID, ProjectID: projectID, OwnerID: userID, Name: name, Shared: shared, Filters: f}
	if err := s.views.Create(ctx, orgID, v); err != nil {
		return nil, err
	}
	return s.views.Get(ctx, orgID, v.ID)
}

// UpdateView renames a view, replaces its filters or changes its sharing.
// A personal view is its owner's; a shared one, or sharing at all, needs LEAD.
func (s *Service) UpdateView(ctx context.Context, orgID, userID, projectID, viewID, name string, shared bool, f project.ViewFilters, orgRole tenant.OrgRole) (*project.SavedView, error) {
	if s.views == nil {
		return nil, domain.ErrForbidden // saved views disabled
	}
	v, err := s.editableView(ctx, orgID, userID, projectID, viewID, orgRole)
	if err != nil {
		return nil, err
	}
	if shared && !v.Shared {
		if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
			return nil, err
		}
	}
	if v.Name, v.Filters, err = project.NormalizeSavedView(name, f); err != nil {
		return nil, err
	}
	v.Shared = shared
	if err := s.views.Update(ctx, orgID, v); err != nil {
		return nil, err
	}
	return v, nil
}

// DeleteView removes a view under the same rules as UpdateView.
func (s *Service) DeleteView(ctx context.Context, orgID, userID, projectID, viewID string, orgRole tenant.OrgRole) error {
	if s.views == nil {
		return domain.ErrForbidden // saved views disabled
	}
	if _, err := s.editableView(ctx, orgID, userID, projectID, viewID, orgRole); err != nil {
		return err
	}
	return s.views.Delete(ctx, orgID, viewID)
}

func viewRole(shared bool) project.ProjectRole {
	if shared {
		return project.RoleLead
	}
	return project.RoleViewer
}

// editableView loads a view of the project the caller may change: their own
// personal view (VIEWER) or a shared view (LEAD). Someone else's personal view
// is ErrNotFound, so its existence does not leak.
func (s *Service) editableView(ctx context.Context, orgID, userID, projectID, viewID string, orgRole tenant.OrgRole) (*project.SavedView, error) {
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	v, err := s.views.Get(ctx, orgID, viewID)
	if err != nil {
		return nil, err
	}
	if v.ProjectID != projectID || (!v.Shared && v.OwnerID != userID) {
		return nil, domain.ErrNotFound
	}
	if v.Shared {
		if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
			return nil, err
		}
	}
	return v, nil
}
