package projectuc

import (
	"context"
	"strings"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Project wiki (FR-PROJ-013) ----------------------------------------------
// Pages are read by VIEWER+, created and edited by CONTRIBUTOR+, deleted by a
// LEAD or the page's author. Edits are version-checked. The mention index only
// holds tasks of the page's own project, so reading a page never reveals tasks
// the reader cannot already see.

// WikiPageView is a page with the live tasks its body mentions.
type WikiPageView struct {
	Page  *project.WikiPage
	Tasks []project.WikiTaskRef
}

// CreateWikiPage stores a page (CONTRIBUTOR).
func (s *Service) CreateWikiPage(ctx context.Context, orgID, userID, projectID, title, body string, orgRole tenant.OrgRole) (*WikiPageView, error) {
	if s.wiki == nil {
		return nil, domain.ErrForbidden // wiki disabled
	}
	p, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor)
	if err != nil {
		return nil, err
	}
	title, body, err = project.NormalizeWikiPage(title, body)
	if err != nil {
		return nil, err
	}
	page := &project.WikiPage{ID: newID(), OrgID: orgID, ProjectID: projectID, Title: title, Body: body, CreatedBy: userID, UpdatedBy: userID}
	if err := s.wiki.Create(ctx, orgID, page, project.ExtractTaskNumbers(body, p.Key)); err != nil {
		return nil, err
	}
	return s.wikiView(ctx, orgID, page.ID)
}

// ListWikiPages returns a project's pages by title, optionally filtered by a
// title substring (VIEWER). Bodies are not included.
func (s *Service) ListWikiPages(ctx context.Context, orgID, userID, projectID, query string, orgRole tenant.OrgRole) ([]project.WikiPage, error) {
	if s.wiki == nil {
		return nil, domain.ErrForbidden // wiki disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	return s.wiki.ListByProject(ctx, orgID, projectID, strings.TrimSpace(query))
}

// GetWikiPage returns a page with the tasks it mentions (VIEWER).
func (s *Service) GetWikiPage(ctx context.Context, orgID, userID, projectID, pageID string, orgRole tenant.OrgRole) (*WikiPageView, error) {
	if s.wiki == nil {
		return nil, domain.ErrForbidden // wiki disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	if _, err := s.pageInProject(ctx, orgID, projectID, pageID); err != nil {
		return nil, err
	}
	return s.wikiView(ctx, orgID, pageID)
}

// UpdateWikiPage edits a page that the caller last read at ifVersion
// (CONTRIBUTOR); a page saved since then is refused with ErrConflict.
func (s *Service) UpdateWikiPage(ctx context.Context, orgID, userID, projectID, pageID, title, body string, ifVersion int, orgRole tenant.OrgRole) (*WikiPageView, error) {
	if s.wiki == nil {
		return nil, domain.ErrForbidden // wiki disabled
	}
	p, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor)
	if err != nil {
		return nil, err
	}
	page, err := s.pageInProject(ctx, orgID, projectID, pageID)
	if err != nil {
		return nil, err
	}
	if ifVersion < 1 {
		return nil, domain.ErrValidation
	}
	if page.Title, page.Body, err = project.NormalizeWikiPage(title, body); err != nil {
		return nil, err
	}
	page.UpdatedBy = userID
	if err := s.wiki.Update(ctx, orgID, page, ifVersion, project.ExtractTaskNumbers(page.Body, p.Key)); err != nil {
		return nil, err
	}
	return s.wikiView(ctx, orgID, pageID)
}

// DeleteWikiPage removes a page (project LEAD or the page's author).
func (s *Service) DeleteWikiPage(ctx context.Context, orgID, userID, projectID, pageID string, orgRole tenant.OrgRole) error {
	if s.wiki == nil {
		return domain.ErrForbidden // wiki disabled
	}
	_, role, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer)
	if err != nil {
		return err
	}
	page, err := s.pageInProject(ctx, orgID, projectID, pageID)
	if err != nil {
		return err
	}
	if role != project.RoleLead && page.CreatedBy != userID {
		return domain.ErrForbidden
	}
	return s.wiki.Delete(ctx, orgID, pageID)
}

// WikiPagesMentioningTask lists the project's pages that name a task number
// (VIEWER); it backs the "Mentioned in" list of the task detail.
func (s *Service) WikiPagesMentioningTask(ctx context.Context, orgID, userID, projectID string, number int, orgRole tenant.OrgRole) ([]project.WikiPage, error) {
	if s.wiki == nil {
		return nil, domain.ErrForbidden // wiki disabled
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	return s.wiki.MentioningTask(ctx, orgID, projectID, number)
}

func (s *Service) wikiView(ctx context.Context, orgID, pageID string) (*WikiPageView, error) {
	page, err := s.wiki.Get(ctx, orgID, pageID)
	if err != nil {
		return nil, err
	}
	refs, err := s.wiki.TaskRefs(ctx, orgID, pageID)
	if err != nil {
		return nil, err
	}
	return &WikiPageView{Page: page, Tasks: refs}, nil
}

// pageInProject loads a page, ensuring it belongs to the project.
func (s *Service) pageInProject(ctx context.Context, orgID, projectID, pageID string) (*project.WikiPage, error) {
	page, err := s.wiki.Get(ctx, orgID, pageID)
	if err != nil {
		return nil, err
	}
	if page.ProjectID != projectID {
		return nil, domain.ErrNotFound
	}
	return page, nil
}
