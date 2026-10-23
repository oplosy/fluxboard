package projectuc

import (
	"context"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Epics (FR-PROJ-008) -----------------------------------------------------
// Epics are managed by project LEADs; any CONTRIBUTOR may file tasks under one.
// Progress is derived on read: tasks in the done column over all its tasks.

// CreateEpic stores an epic (LEAD).
func (s *Service) CreateEpic(ctx context.Context, orgID, userID, projectID, name, color string, orgRole tenant.OrgRole) (*project.Epic, error) {
	if s.epics == nil {
		return nil, domain.ErrForbidden // epics disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
		return nil, err
	}
	name, color, err := project.NormalizeEpic(name, color)
	if err != nil {
		return nil, err
	}
	e := &project.Epic{ID: newID(), OrgID: orgID, ProjectID: projectID, Name: name, Color: color, CreatedBy: userID}
	if err := s.epics.Create(ctx, orgID, e); err != nil {
		return nil, err
	}
	return s.epics.Get(ctx, orgID, e.ID)
}

// ListEpics returns a project's epics oldest-first with progress (VIEWER).
func (s *Service) ListEpics(ctx context.Context, orgID, userID, projectID string, orgRole tenant.OrgRole) ([]project.Epic, error) {
	if s.epics == nil {
		return nil, domain.ErrForbidden // epics disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	epics, err := s.epics.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.withProgress(ctx, orgID, projectID, epics); err != nil {
		return nil, err
	}
	return epics, nil
}

// withProgress fills the progress counters of one project's epics, counted in
// SQL rather than by loading every task of the project.
func (s *Service) withProgress(ctx context.Context, orgID, projectID string, epics []project.Epic) error {
	if len(epics) == 0 {
		return nil
	}
	progress, err := s.epics.Progress(ctx, orgID, projectID, s.finalColumnID(ctx, orgID, projectID))
	if err != nil {
		return err
	}
	for i := range epics {
		p := progress[epics[i].ID]
		epics[i].Total, epics[i].Done, epics[i].Points, epics[i].DonePoints = p.Total, p.Done, p.Points, p.DonePoints
	}
	return nil
}

// UpdateEpic renames or recolors an epic (LEAD).
func (s *Service) UpdateEpic(ctx context.Context, orgID, userID, projectID, epicID, name, color string, orgRole tenant.OrgRole) (*project.Epic, error) {
	if s.epics == nil {
		return nil, domain.ErrForbidden // epics disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
		return nil, err
	}
	e, err := s.epicInProject(ctx, orgID, projectID, epicID)
	if err != nil {
		return nil, err
	}
	if e.Name, e.Color, err = project.NormalizeEpic(name, color); err != nil {
		return nil, err
	}
	if err := s.epics.Update(ctx, orgID, e); err != nil {
		return nil, err
	}
	one := []project.Epic{*e}
	if err := s.withProgress(ctx, orgID, projectID, one); err != nil {
		return nil, err
	}
	return &one[0], nil
}

// DeleteEpic removes an epic (LEAD); its tasks stay, without an epic.
func (s *Service) DeleteEpic(ctx context.Context, orgID, userID, projectID, epicID string, orgRole tenant.OrgRole) error {
	if s.epics == nil {
		return domain.ErrForbidden // epics disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
		return err
	}
	if _, err := s.epicInProject(ctx, orgID, projectID, epicID); err != nil {
		return err
	}
	return s.epics.Delete(ctx, orgID, epicID)
}

// AssignEpic files tasks under an epic of their project, nil clears it
// (CONTRIBUTOR). Every task must belong to the project.
func (s *Service) AssignEpic(ctx context.Context, orgID, userID, projectID string, taskIDs []string, epicID *string, orgRole tenant.OrgRole) error {
	if s.epics == nil {
		return domain.ErrForbidden // epics disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor); err != nil {
		return err
	}
	if epicID != nil {
		if _, err := s.epicInProject(ctx, orgID, projectID, *epicID); err != nil {
			return err
		}
	}
	if len(taskIDs) == 0 || len(taskIDs) > project.MaxBulkTasks {
		return domain.ErrValidation
	}
	for _, id := range taskIDs {
		t, err := s.tasks.Get(ctx, orgID, id)
		if err != nil {
			return err
		}
		if t.ProjectID != projectID {
			return domain.ErrValidation
		}
	}
	for _, id := range taskIDs {
		if err := s.epics.SetTaskEpic(ctx, orgID, id, epicID); err != nil {
			return err
		}
	}
	return nil
}

// epicInProject loads an epic, ensuring it belongs to the project.
func (s *Service) epicInProject(ctx context.Context, orgID, projectID, epicID string) (*project.Epic, error) {
	e, err := s.epics.Get(ctx, orgID, epicID)
	if err != nil {
		return nil, err
	}
	if e.ProjectID != projectID {
		return nil, domain.ErrNotFound
	}
	return e, nil
}
