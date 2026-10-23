package projectuc

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Releases (FR-PROJ-010) --------------------------------------------------
// Releases are managed by project LEADs; any CONTRIBUTOR may file tasks under
// one. Progress is derived on read: tasks and points in the done column over
// all of the release's tasks.

// ReleaseInput carries the editable fields of a release.
type ReleaseInput struct {
	Name        string
	Description string
	ReleaseDate *time.Time // planned day; only the UTC date is kept
	Released    bool       // update only: true marks it released, false reopens it
}

func releaseDay(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	d := time.Date(t.UTC().Year(), t.UTC().Month(), t.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return &d
}

// CreateRelease stores an unreleased release (LEAD).
func (s *Service) CreateRelease(ctx context.Context, orgID, userID, projectID string, in ReleaseInput, orgRole tenant.OrgRole) (*project.Release, error) {
	if s.releases == nil {
		return nil, domain.ErrForbidden // releases disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
		return nil, err
	}
	name, desc, err := project.NormalizeRelease(in.Name, in.Description)
	if err != nil {
		return nil, err
	}
	r := &project.Release{
		ID: newID(), OrgID: orgID, ProjectID: projectID, Name: name, Description: desc,
		ReleaseDate: releaseDay(in.ReleaseDate), CreatedBy: userID,
	}
	if err := s.releases.Create(ctx, orgID, r); err != nil {
		return nil, err
	}
	return s.releases.Get(ctx, orgID, r.ID)
}

// ListReleases returns a project's releases oldest-first with progress (VIEWER).
func (s *Service) ListReleases(ctx context.Context, orgID, userID, projectID string, orgRole tenant.OrgRole) ([]project.Release, error) {
	if s.releases == nil {
		return nil, domain.ErrForbidden // releases disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	releases, err := s.releases.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	if err := s.withReleaseProgress(ctx, orgID, projectID, releases); err != nil {
		return nil, err
	}
	return releases, nil
}

// withReleaseProgress fills the progress counters of one project's releases,
// counted in SQL rather than by loading every task of the project.
func (s *Service) withReleaseProgress(ctx context.Context, orgID, projectID string, releases []project.Release) error {
	if len(releases) == 0 {
		return nil
	}
	progress, err := s.releases.Progress(ctx, orgID, projectID, s.finalColumnID(ctx, orgID, projectID))
	if err != nil {
		return err
	}
	for i := range releases {
		p := progress[releases[i].ID]
		releases[i].Total, releases[i].Done, releases[i].Points, releases[i].DonePoints = p.Total, p.Done, p.Points, p.DonePoints
	}
	return nil
}

// UpdateRelease edits a release and marks it released or reopens it (LEAD).
// Marking an already released release keeps its original released time.
func (s *Service) UpdateRelease(ctx context.Context, orgID, userID, projectID, releaseID string, in ReleaseInput, orgRole tenant.OrgRole) (*project.Release, error) {
	if s.releases == nil {
		return nil, domain.ErrForbidden // releases disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
		return nil, err
	}
	r, err := s.releaseInProject(ctx, orgID, projectID, releaseID)
	if err != nil {
		return nil, err
	}
	if r.Name, r.Description, err = project.NormalizeRelease(in.Name, in.Description); err != nil {
		return nil, err
	}
	r.ReleaseDate = releaseDay(in.ReleaseDate)
	switch {
	case in.Released && r.ReleasedAt == nil:
		now := s.now().UTC()
		r.ReleasedAt = &now
	case !in.Released:
		r.ReleasedAt = nil
	}
	if err := s.releases.Update(ctx, orgID, r); err != nil {
		return nil, err
	}
	one := []project.Release{*r}
	if err := s.withReleaseProgress(ctx, orgID, projectID, one); err != nil {
		return nil, err
	}
	return &one[0], nil
}

// DeleteRelease removes a release (LEAD); its tasks stay, without a release.
func (s *Service) DeleteRelease(ctx context.Context, orgID, userID, projectID, releaseID string, orgRole tenant.OrgRole) error {
	if s.releases == nil {
		return domain.ErrForbidden // releases disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleLead); err != nil {
		return err
	}
	if _, err := s.releaseInProject(ctx, orgID, projectID, releaseID); err != nil {
		return err
	}
	return s.releases.Delete(ctx, orgID, releaseID)
}

// AssignRelease files tasks under a release of their project, nil clears it
// (CONTRIBUTOR). Every task must belong to the project.
func (s *Service) AssignRelease(ctx context.Context, orgID, userID, projectID string, taskIDs []string, releaseID *string, orgRole tenant.OrgRole) error {
	if s.releases == nil {
		return domain.ErrForbidden // releases disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor); err != nil {
		return err
	}
	if releaseID != nil {
		if _, err := s.releaseInProject(ctx, orgID, projectID, *releaseID); err != nil {
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
		if err := s.releases.SetTaskRelease(ctx, orgID, id, releaseID); err != nil {
			return err
		}
	}
	return nil
}

// releaseInProject loads a release, ensuring it belongs to the project.
func (s *Service) releaseInProject(ctx context.Context, orgID, projectID, releaseID string) (*project.Release, error) {
	r, err := s.releases.Get(ctx, orgID, releaseID)
	if err != nil {
		return nil, err
	}
	if r.ProjectID != projectID {
		return nil, domain.ErrNotFound
	}
	return r, nil
}
