package projectuc

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Workload returns each assignee's open and scheduled work in a project and
// the time they logged on it, for the UTC days from through to (VIEWER,
// FR-AN-004). The range must be ordered and at most MaxWorkloadDays long.
func (s *Service) Workload(ctx context.Context, orgID, userID, projectID string, from, to time.Time, orgRole tenant.OrgRole) ([]project.WorkloadRow, error) {
	if to.Before(from) || to.Sub(from) >= project.MaxWorkloadDays*24*time.Hour {
		return nil, domain.ErrValidation
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	tasks, err := s.tasks.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	logged := map[string]int64{}
	if s.timeLog != nil {
		if logged, err = s.timeLog.SumByUser(ctx, orgID, projectID, from, to.AddDate(0, 0, 1)); err != nil {
			return nil, err
		}
	}
	return project.ComputeWorkload(tasks, s.finalColumnID(ctx, orgID, projectID), from, to, logged), nil
}
