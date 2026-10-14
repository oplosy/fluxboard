package projectuc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/rank"
)

// IdempotencyTTL bounds a create-from-template replay key (24h, as invites).
const IdempotencyTTL = 24 * time.Hour

// IdempotencyStore backs the Idempotency-Key replay guard (docs/08 §4). Get
// returns "" (no error) when the key is absent. Implemented by
// redisx.IdempotencyStore (wired in cmd), mirroring tenantuc.
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, val string, ttl time.Duration) error
}

// ErrTemplatesUnavailable: no seeder is wired.
var ErrTemplatesUnavailable = errors.New("project templates unavailable")

// Templates returns the project template catalog (FR-PROJ-007).
func (s *Service) Templates() []project.Template { return project.Templates }

// CreateProjectFromTemplate builds a project, its board, the template's columns
// (with roles), labels and starter tasks in one transaction, and makes the
// caller LEAD (FR-PROJ-007, ADR-035). A retry with the same Idempotency-Key
// returns the project created the first time. The org-role and plan-limit
// gates run in the route middleware, as for CreateProject.
func (s *Service) CreateProjectFromTemplate(ctx context.Context, orgID, userID, templateKey string, in CreateProjectInput, idempotencyKey string) (*project.Project, error) {
	if s.seeder == nil {
		return nil, ErrTemplatesUnavailable
	}
	tpl, ok := project.TemplateByKey(templateKey)
	if !ok {
		return nil, domain.ErrValidation
	}
	if err := in.normalize(); err != nil {
		return nil, err
	}

	idemKey := ""
	if s.idem != nil && strings.TrimSpace(idempotencyKey) != "" {
		idemKey = "project:tpl:idem:" + orgID + ":" + userID + ":" + strings.TrimSpace(idempotencyKey)
		if priorID, err := s.idem.Get(ctx, idemKey); err == nil && priorID != "" {
			if p, err := s.projects.Get(ctx, orgID, priorID); err == nil {
				return p, nil
			}
		}
	}

	seed := buildSeed(tpl, orgID, userID, in)
	if err := s.seeder.Seed(ctx, orgID, seed); err != nil {
		return nil, err // ErrConflict on duplicate key
	}
	if idemKey != "" {
		if err := s.idem.Set(ctx, idemKey, seed.Project.ID, IdempotencyTTL); err != nil {
			s.logger.Warn("project template idempotency set failed", "err", err)
		}
	}
	return s.projects.Get(ctx, orgID, seed.Project.ID)
}

// buildSeed assigns ids and ranks to a template instance.
func buildSeed(tpl project.Template, orgID, userID string, in CreateProjectInput) *project.ProjectSeed {
	seed := &project.ProjectSeed{
		Project: project.Project{
			ID: newID(), OrgID: orgID, Key: in.Key, Name: in.Name,
			Description: in.Description, Color: in.Color, Visibility: in.Visibility, CreatedBy: userID,
		},
		LeadID:     userID,
		TaskLabels: map[string][]string{},
	}
	seed.Board = project.Board{ID: newID(), ProjectID: seed.Project.ID, Name: "Board"}
	colRanks := rank.Initial(len(tpl.Columns))
	for i, c := range tpl.Columns {
		seed.Columns = append(seed.Columns, project.Column{
			ID: newID(), BoardID: seed.Board.ID, Name: c.Name, Rank: colRanks[i], Role: c.Role,
		})
	}
	for _, l := range tpl.Labels {
		seed.Labels = append(seed.Labels, project.Label{ID: newID(), OrgID: orgID, Name: l.Name, Color: l.Color})
	}
	perColumn := make([]int, len(tpl.Columns))
	for _, t := range tpl.Tasks {
		perColumn[t.Column]++
	}
	taskRanks := make([][]string, len(tpl.Columns))
	for i, n := range perColumn {
		taskRanks[i] = rank.Initial(n)
	}
	used := make([]int, len(tpl.Columns))
	for _, t := range tpl.Tasks {
		task := project.Task{
			ID: newID(), OrgID: orgID, ProjectID: seed.Project.ID, ColumnID: seed.Columns[t.Column].ID,
			Title: t.Title, Description: t.Description, Priority: t.Priority,
			Rank: taskRanks[t.Column][used[t.Column]], CreatedBy: userID,
		}
		used[t.Column]++
		seed.Tasks = append(seed.Tasks, task)
		if len(t.Labels) > 0 {
			seed.TaskLabels[task.ID] = t.Labels
		}
	}
	return seed
}
