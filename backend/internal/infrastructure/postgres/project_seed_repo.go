package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// ProjectSeedRepo is the Postgres-backed project.ProjectSeeder: the whole seed
// runs in ONE WithTenant transaction, so a failure rolls everything back
// (FR-PROJ-007, ADR-035).
type ProjectSeedRepo struct{ tp *TenantPool }

// NewProjectSeedRepo builds a ProjectSeedRepo over the tenant pool.
func NewProjectSeedRepo(tp *TenantPool) *ProjectSeedRepo { return &ProjectSeedRepo{tp: tp} }

var _ project.ProjectSeeder = (*ProjectSeedRepo)(nil)

func (r *ProjectSeedRepo) Seed(ctx context.Context, orgID string, s *project.ProjectSeed) error {
	oid, err := parseUUID(orgID)
	if err != nil {
		return fmt.Errorf("project seed: org: %w", err)
	}
	pid, err := parseUUID(s.Project.ID)
	if err != nil {
		return fmt.Errorf("project seed: id: %w", err)
	}
	lead, err := parseUUID(s.LeadID)
	if err != nil {
		return fmt.Errorf("project seed: lead: %w", err)
	}
	bid, err := parseUUID(s.Board.ID)
	if err != nil {
		return fmt.Errorf("project seed: board: %w", err)
	}
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		p := s.Project
		if err := q.CreateProject(ctx, gen.CreateProjectParams{
			ID: pid, OrgID: oid, Key: p.Key, Name: p.Name, Description: p.Description,
			Color: p.Color, Visibility: string(p.Visibility), CreatedBy: lead,
		}); err != nil {
			return err
		}
		if err := q.AddProjectMember(ctx, gen.AddProjectMemberParams{
			ProjectID: pid, OrgID: oid, UserID: lead, Role: string(project.RoleLead),
		}); err != nil {
			return err
		}
		if err := q.CreateBoard(ctx, gen.CreateBoardParams{ID: bid, OrgID: oid, ProjectID: pid, Name: s.Board.Name}); err != nil {
			return err
		}
		for _, c := range s.Columns {
			cid, err := parseUUID(c.ID)
			if err != nil {
				return fmt.Errorf("project seed: column: %w", err)
			}
			if err := q.CreateColumn(ctx, gen.CreateColumnParams{
				ID: cid, OrgID: oid, BoardID: bid, Name: c.Name, Rank: c.Rank, WipLimit: int32Ptr(c.WIPLimit),
			}); err != nil {
				return err
			}
			if c.Role != "" {
				if _, err := q.SetColumnRole(ctx, gen.SetColumnRoleParams{
					OrgID: oid, BoardID: bid, ID: cid, Role: ptrOrNil(string(c.Role)),
				}); err != nil {
					return err
				}
			}
		}
		labelIDs := make(map[string]uuid.UUID, len(s.Labels))
		for _, l := range s.Labels {
			lid, err := parseUUID(l.ID)
			if err != nil {
				return fmt.Errorf("project seed: label: %w", err)
			}
			got, err := q.EnsureLabel(ctx, gen.EnsureLabelParams{ID: lid, OrgID: oid, Name: l.Name, Color: l.Color})
			if err != nil {
				return err
			}
			labelIDs[l.Name] = got
		}
		for i := range s.Tasks {
			t := &s.Tasks[i]
			tid, err := parseUUID(t.ID)
			if err != nil {
				return fmt.Errorf("project seed: task: %w", err)
			}
			cid, err := parseUUID(t.ColumnID)
			if err != nil {
				return fmt.Errorf("project seed: task column: %w", err)
			}
			num, err := q.NextTaskNumber(ctx, gen.NextTaskNumberParams{OrgID: oid, ID: pid})
			if err != nil {
				return err
			}
			t.Number = int(num)
			if err := q.CreateTask(ctx, gen.CreateTaskParams{
				ID: tid, OrgID: oid, ProjectID: pid, ColumnID: cid, Number: num,
				Title: t.Title, Description: t.Description, Priority: string(t.Priority),
				Rank: t.Rank, CreatedBy: lead,
			}); err != nil {
				return err
			}
			for _, name := range s.TaskLabels[t.ID] {
				lid, ok := labelIDs[name]
				if !ok {
					return fmt.Errorf("%w: task label %q not in seed", domain.ErrValidation, name)
				}
				if err := q.AttachLabel(ctx, gen.AttachLabelParams{TaskID: tid, LabelID: lid, OrgID: oid}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if isUnique(err) {
		return domain.ErrConflict
	}
	return err
}
