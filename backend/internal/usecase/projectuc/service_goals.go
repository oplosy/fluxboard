package projectuc

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Goals (FR-PROJ-013) -----------------------------------------------------
// Goals are org-level. An org MEMBER+ creates one and owns it; the owner or an
// org ADMIN+ edits, deletes and links work. Everyone in the org reads goals,
// but linked work of a project the viewer cannot see is skipped entirely, so
// neither its title nor its tasks reach the progress numbers (FR-PROJ-002).

// GoalInput carries the editable fields of a goal.
type GoalInput struct {
	Title       string
	Description string
	TargetDate  *time.Time // only the UTC date is kept
}

// GoalView is a goal with the linked work the viewer may see.
type GoalView struct {
	Goal  project.Goal
	Items []project.GoalItem
}

func canManageGoal(g *project.Goal, userID string, orgRole tenant.OrgRole) bool {
	return g.OwnerID == userID || orgRole.AtLeast(tenant.RoleAdmin)
}

// CreateGoal stores a goal owned by the caller (org MEMBER+).
func (s *Service) CreateGoal(ctx context.Context, orgID, userID string, in GoalInput, orgRole tenant.OrgRole) (*GoalView, error) {
	if s.goals == nil {
		return nil, domain.ErrForbidden // goals disabled
	}
	if !orgRole.AtLeast(tenant.RoleMember) {
		return nil, domain.ErrForbidden
	}
	title, desc, err := project.NormalizeGoal(in.Title, in.Description)
	if err != nil {
		return nil, err
	}
	g := &project.Goal{ID: newID(), OrgID: orgID, Title: title, Description: desc, OwnerID: userID, TargetDate: releaseDay(in.TargetDate)}
	if err := s.goals.Create(ctx, orgID, g); err != nil {
		return nil, err
	}
	return s.GetGoal(ctx, orgID, userID, g.ID, orgRole)
}

// ListGoals returns the org's goals with derived progress and status.
func (s *Service) ListGoals(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole) ([]project.Goal, error) {
	if s.goals == nil {
		return nil, domain.ErrForbidden // goals disabled
	}
	goals, err := s.goals.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	links, err := s.goals.Links(ctx, orgID, "")
	if err != nil {
		return nil, err
	}
	views, err := s.goalViews(ctx, orgID, userID, orgRole, goals, links)
	if err != nil {
		return nil, err
	}
	out := make([]project.Goal, len(views))
	for i := range views {
		out[i] = views[i].Goal
	}
	return out, nil
}

// GetGoal returns one goal with its visible linked items.
func (s *Service) GetGoal(ctx context.Context, orgID, userID, goalID string, orgRole tenant.OrgRole) (*GoalView, error) {
	if s.goals == nil {
		return nil, domain.ErrForbidden // goals disabled
	}
	g, err := s.goals.Get(ctx, orgID, goalID)
	if err != nil {
		return nil, err
	}
	links, err := s.goals.Links(ctx, orgID, goalID)
	if err != nil {
		return nil, err
	}
	views, err := s.goalViews(ctx, orgID, userID, orgRole, []project.Goal{*g}, links)
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// UpdateGoal edits a goal (owner or org ADMIN+).
func (s *Service) UpdateGoal(ctx context.Context, orgID, userID, goalID string, in GoalInput, orgRole tenant.OrgRole) (*GoalView, error) {
	g, err := s.manageGoal(ctx, orgID, userID, goalID, orgRole)
	if err != nil {
		return nil, err
	}
	if g.Title, g.Description, err = project.NormalizeGoal(in.Title, in.Description); err != nil {
		return nil, err
	}
	g.TargetDate = releaseDay(in.TargetDate)
	if err := s.goals.Update(ctx, orgID, g); err != nil {
		return nil, err
	}
	return s.GetGoal(ctx, orgID, userID, goalID, orgRole)
}

// DeleteGoal removes a goal; the linked epics and tasks stay (owner or org ADMIN+).
func (s *Service) DeleteGoal(ctx context.Context, orgID, userID, goalID string, orgRole tenant.OrgRole) error {
	if _, err := s.manageGoal(ctx, orgID, userID, goalID, orgRole); err != nil {
		return err
	}
	return s.goals.Delete(ctx, orgID, goalID)
}

// LinkGoalItem links an epic or task to a goal (owner or org ADMIN+). The item
// must sit in a project the caller can see; a hidden one reads as not found.
func (s *Service) LinkGoalItem(ctx context.Context, orgID, userID, goalID string, kind project.GoalItemKind, itemID string, orgRole tenant.OrgRole) error {
	if !kind.Valid() {
		return domain.ErrValidation
	}
	if _, err := s.manageGoal(ctx, orgID, userID, goalID, orgRole); err != nil {
		return err
	}
	var projectID string
	if kind == project.GoalItemEpic {
		if s.epics == nil {
			return domain.ErrForbidden // epics disabled
		}
		e, err := s.epics.Get(ctx, orgID, itemID)
		if err != nil {
			return err
		}
		projectID = e.ProjectID
	} else {
		t, err := s.tasks.Get(ctx, orgID, itemID)
		if err != nil {
			return err
		}
		projectID = t.ProjectID
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return err
	}
	return s.goals.Link(ctx, orgID, goalID, kind, itemID)
}

// UnlinkGoalItem removes a link (owner or org ADMIN+).
func (s *Service) UnlinkGoalItem(ctx context.Context, orgID, userID, goalID string, kind project.GoalItemKind, itemID string, orgRole tenant.OrgRole) error {
	if !kind.Valid() {
		return domain.ErrValidation
	}
	if _, err := s.manageGoal(ctx, orgID, userID, goalID, orgRole); err != nil {
		return err
	}
	return s.goals.Unlink(ctx, orgID, goalID, kind, itemID)
}

// manageGoal loads a goal the caller may change: owner or org ADMIN+.
func (s *Service) manageGoal(ctx context.Context, orgID, userID, goalID string, orgRole tenant.OrgRole) (*project.Goal, error) {
	if s.goals == nil {
		return nil, domain.ErrForbidden // goals disabled
	}
	g, err := s.goals.Get(ctx, orgID, goalID)
	if err != nil {
		return nil, err
	}
	if !canManageGoal(g, userID, orgRole) {
		return nil, domain.ErrForbidden
	}
	return g, nil
}

// goalProject is one visible project's work, loaded once per request.
type goalProject struct {
	key   string
	done  string // done column id
	tasks []project.Task
	byID  map[string]*project.Task
	epics map[string]*project.Epic
}

// goalViews derives progress, status and the visible items of goals from their
// links. A task linked directly and via its epic counts once.
func (s *Service) goalViews(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole, goals []project.Goal, links []project.GoalLink) ([]GoalView, error) {
	visible, err := s.projects.List(ctx, orgID, userID, project.ScopeFor(orgRole), true)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]string, len(visible))
	for _, p := range visible {
		keys[p.ID] = p.Key
	}
	loaded := make(map[string]*goalProject)
	load := func(projectID string) (*goalProject, error) {
		if gp, ok := loaded[projectID]; ok {
			return gp, nil
		}
		key, ok := keys[projectID]
		if !ok {
			return nil, nil // not visible to the caller
		}
		tasks, err := s.tasks.ListByProject(ctx, orgID, projectID)
		if err != nil {
			return nil, err
		}
		gp := &goalProject{
			key: key, done: s.finalColumnID(ctx, orgID, projectID), tasks: tasks,
			byID: make(map[string]*project.Task, len(tasks)), epics: make(map[string]*project.Epic),
		}
		for i := range gp.tasks {
			gp.byID[gp.tasks[i].ID] = &gp.tasks[i]
		}
		if s.epics != nil {
			epics, err := s.epics.ListByProject(ctx, orgID, projectID)
			if err != nil {
				return nil, err
			}
			for i := range epics {
				gp.epics[epics[i].ID] = &epics[i]
			}
		}
		loaded[projectID] = gp
		return gp, nil
	}
	byGoal := make(map[string][]project.GoalLink)
	for _, l := range links {
		byGoal[l.GoalID] = append(byGoal[l.GoalID], l)
	}

	now := s.now()
	out := make([]GoalView, len(goals))
	for i := range goals {
		g := goals[i]
		seen := make(map[string]bool)
		items := make([]project.GoalItem, 0)
		// count adds a task to the goal totals once and reports whether it is done.
		count := func(gp *goalProject, t *project.Task) bool {
			isDone := t.ColumnID == gp.done
			if !seen[t.ID] {
				seen[t.ID] = true
				pts := 0
				if t.Estimate != nil {
					pts = *t.Estimate
				}
				g.Total++
				g.Points += pts
				if isDone {
					g.Done++
					g.DonePoints += pts
				}
			}
			return isDone
		}
		for _, l := range byGoal[g.ID] {
			gp, err := load(l.ProjectID)
			if err != nil {
				return nil, err
			}
			if gp == nil {
				continue
			}
			switch l.Kind {
			case project.GoalItemEpic:
				e, ok := gp.epics[l.ItemID]
				if !ok {
					continue
				}
				it := project.GoalItem{Kind: l.Kind, ID: e.ID, ProjectID: l.ProjectID, ProjectKey: gp.key, Title: e.Name}
				for j := range gp.tasks {
					if t := &gp.tasks[j]; t.EpicID != nil && *t.EpicID == e.ID {
						it.Total++
						if count(gp, t) {
							it.Done++
						}
					}
				}
				items = append(items, it)
			case project.GoalItemTask:
				t, ok := gp.byID[l.ItemID]
				if !ok {
					continue
				}
				it := project.GoalItem{Kind: l.Kind, ID: t.ID, ProjectID: l.ProjectID, ProjectKey: gp.key, Number: t.Number, Title: t.Title, Total: 1}
				if count(gp, t) {
					it.Done = 1
				}
				items = append(items, it)
			}
		}
		g.Status = project.DeriveGoalStatus(g.Total, g.Done, g.TargetDate, now)
		out[i] = GoalView{Goal: g, Items: items}
	}
	return out, nil
}
