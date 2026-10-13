package taskuc

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Agent work selection and hand-over (ADR-033, FR-AGT-007/008).

// maxNextAttempts bounds how many ready tasks NextTask tries to claim before
// giving up (other agents may be claiming the same ones).
const maxNextAttempts = 20

// isAgent reports whether userID is an agent (false when no lookup is wired).
func (s *Service) isAgent(ctx context.Context, orgID, userID string) (bool, error) {
	if s.agents == nil {
		return false, nil
	}
	return s.agents.IsAgent(ctx, orgID, userID)
}

// projectColumns returns a project's board columns, rank-ordered.
func (s *Service) projectColumns(ctx context.Context, orgID, projectID string) ([]project.Column, error) {
	board, err := s.boards.GetByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	return s.columns.ListByBoard(ctx, orgID, board.ID)
}

// refuseAgentDone is ErrHumanApprovalRequired when an agent targets the done
// column of projectID (human-approved Done, ADR-033 decision 6).
func (s *Service) refuseAgentDone(ctx context.Context, orgID, userID, projectID, columnID string) error {
	if s.agents == nil {
		return nil
	}
	cols, err := s.projectColumns(ctx, orgID, projectID)
	if err != nil {
		return err
	}
	if project.DoneColumnID(cols) != columnID {
		return nil
	}
	agent, err := s.isAgent(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if agent {
		return project.ErrHumanApprovalRequired
	}
	return nil
}

// priorityWeight orders priorities, most urgent first.
var priorityWeight = map[project.Priority]int{
	project.PriorityUrgent: 4, project.PriorityHigh: 3, project.PriorityMedium: 2,
	project.PriorityLow: 1, project.PriorityNone: 0,
}

// NextTask finds the calling agent's next ready task in a project and, unless
// claim is false, claims it. Work sent back to the agent (in progress,
// assigned to it, unclaimed) comes first; then todo tasks that are unassigned
// or assigned to it. A task is ready when no other agent holds a live claim on
// it and every blocker sits in its project's done column. Within a tier the
// most urgent comes first, then board order. A task the agent already holds
// is offered again rather than more work. nil when nothing is ready.
func (s *Service) NextTask(ctx context.Context, orgID, agentID, projectID string, claim bool, lease time.Duration, orgRole tenant.OrgRole) (*project.Task, error) {
	need := project.RoleViewer
	if claim {
		need = project.RoleContributor
	}
	if _, err := s.access(ctx, orgID, projectID, agentID, orgRole, need); err != nil {
		return nil, err
	}
	cols, err := s.projectColumns(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	todo := project.ColumnForRole(cols, project.ColumnTodo)
	if todo == nil {
		return nil, project.ErrNoColumnForRole
	}
	inProgress := project.ColumnForRole(cols, project.ColumnInProgress)
	tasks, err := s.tasks.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	type candidate struct {
		t    project.Task
		tier int
	}
	var cands []candidate
	for _, t := range tasks {
		if t.ClaimActive(now) && *t.ClaimedBy != agentID {
			continue
		}
		mine := t.AssigneeID != nil && *t.AssigneeID == agentID
		held := t.ClaimActive(now) // by this agent: others were skipped above
		inWork := t.ColumnID == todo.ID || (inProgress != nil && t.ColumnID == inProgress.ID)
		switch {
		case held && inWork:
			cands = append(cands, candidate{t, 0}) // unfinished work it holds
		case inProgress != nil && t.ColumnID == inProgress.ID && mine:
			cands = append(cands, candidate{t, 1}) // sent back to it
		case t.ColumnID == todo.ID && (t.AssigneeID == nil || mine):
			cands = append(cands, candidate{t, 2})
		}
	}
	// ListByProject is in board order; a stable sort keeps it within a tier
	// and priority.
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].tier != cands[j].tier {
			return cands[i].tier < cands[j].tier
		}
		return priorityWeight[cands[i].t.Priority] > priorityWeight[cands[j].t.Priority]
	})

	doneByProject := map[string]string{projectID: project.DoneColumnID(cols)}
	attempts := 0
	for i := range cands {
		t := &cands[i].t
		ready, err := s.unblocked(ctx, orgID, t.ID, doneByProject)
		if err != nil {
			return nil, err
		}
		if !ready {
			continue
		}
		if !claim {
			return t, nil
		}
		if attempts++; attempts > maxNextAttempts {
			break
		}
		got, err := s.ClaimTask(ctx, orgID, agentID, t.ID, lease, orgRole)
		var cc *project.ClaimConflict
		switch {
		case err == nil:
			return got, nil
		case errors.As(err, &cc), errors.Is(err, project.ErrClaimHeld), errors.Is(err, domain.ErrNotFound):
			continue // taken or removed meanwhile: try the next one
		default:
			return nil, err
		}
	}
	return nil, nil
}

// unblocked reports whether every blocker of taskID sits in its project's done
// column (done columns are cached per project in doneByProject).
func (s *Service) unblocked(ctx context.Context, orgID, taskID string, doneByProject map[string]string) (bool, error) {
	if s.links == nil {
		return true, nil
	}
	blockers, err := s.links.Blockers(ctx, orgID, taskID)
	if err != nil {
		return false, err
	}
	for _, b := range blockers {
		done, ok := doneByProject[b.ProjectID]
		if !ok {
			cols, err := s.projectColumns(ctx, orgID, b.ProjectID)
			if err != nil {
				return false, err
			}
			done = project.DoneColumnID(cols)
			doneByProject[b.ProjectID] = done
		}
		if b.ColumnID != done {
			return false, nil
		}
	}
	return true, nil
}

// EndClaim finishes the calling agent's claim but keeps it as the assignee:
// the work is handed over (submitted for review). ErrClaimLost when it holds
// no claim.
func (s *Service) EndClaim(ctx context.Context, orgID, agentID, taskID string, orgRole tenant.OrgRole) (*project.Task, error) {
	if s.conc == nil {
		return nil, errNoConcurrency
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, agentID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	ok, err := s.conc.EndClaim(ctx, orgID, taskID, agentID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, project.ErrClaimLost
	}
	return s.tasks.Get(ctx, orgID, taskID)
}
