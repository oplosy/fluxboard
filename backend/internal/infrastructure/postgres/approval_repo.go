package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// ApprovalRepo is the Postgres-backed project.ApprovalRepository ([T], RLS).
type ApprovalRepo struct{ tp *TenantPool }

// NewApprovalRepo builds an ApprovalRepo over the tenant pool.
func NewApprovalRepo(tp *TenantPool) *ApprovalRepo { return &ApprovalRepo{tp: tp} }

var _ project.ApprovalRepository = (*ApprovalRepo)(nil)

func approvalFrom(r gen.TaskApproval) project.Approval {
	return project.Approval{
		ID: r.ID.String(), OrgID: r.OrgID.String(), TaskID: r.TaskID.String(),
		RequestedBy: r.RequestedBy.String(), Note: r.Note, Deadline: tsPtr(r.Deadline),
		State: project.ApprovalState(r.State), StaleAt: tsPtr(r.StaleAt), DecidedAt: tsPtr(r.DecidedAt),
		CreatedAt: r.CreatedAt, Approvers: []project.Approver{},
	}
}

// approversFor attaches each request's approvers (one query for all).
func approversFor(ctx context.Context, q *gen.Queries, oid uuid.UUID, as []project.Approval) error {
	if len(as) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(as))
	idx := make(map[uuid.UUID]int, len(as))
	for i := range as {
		id, err := parseUUID(as[i].ID)
		if err != nil {
			return err
		}
		ids[i], idx[id] = id, i
	}
	rows, err := q.ListApprovalApprovers(ctx, gen.ListApprovalApproversParams{OrgID: oid, ApprovalIds: ids})
	if err != nil {
		return err
	}
	for _, r := range rows {
		a := &as[idx[r.ApprovalID]]
		a.Approvers = append(a.Approvers, project.Approver{
			UserID: r.UserID.String(), Decision: project.ApprovalDecision(derefStr(r.Decision)),
			Comment: r.Comment, DecidedAt: tsPtr(r.DecidedAt),
		})
	}
	return nil
}

func (r *ApprovalRepo) Create(ctx context.Context, orgID string, a *project.Approval) error {
	oid, err := parseUUID(orgID)
	if err != nil {
		return fmt.Errorf("approval create: %w", err)
	}
	id, err := parseUUID(a.ID)
	if err != nil {
		return fmt.Errorf("approval create: id: %w", err)
	}
	tid, err := parseUUID(a.TaskID)
	if err != nil {
		return fmt.Errorf("approval create: task: %w", err)
	}
	by, err := parseUUID(a.RequestedBy)
	if err != nil {
		return fmt.Errorf("approval create: requested_by: %w", err)
	}
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		if err := q.InsertTaskApproval(ctx, gen.InsertTaskApprovalParams{
			ID: id, OrgID: oid, TaskID: tid, RequestedBy: by, Note: a.Note, Deadline: nullTS(a.Deadline),
		}); err != nil {
			return err
		}
		for _, p := range a.Approvers {
			uid, err := parseUUID(p.UserID)
			if err != nil {
				return fmt.Errorf("approver id: %w", err)
			}
			if err := q.InsertTaskApprovalApprover(ctx, gen.InsertTaskApprovalApproverParams{
				ApprovalID: id, OrgID: oid, UserID: uid,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if isUnique(err) {
		return project.ErrApprovalOpen
	}
	return err
}

func (r *ApprovalRepo) Get(ctx context.Context, orgID, taskID, approvalID string) (*project.Approval, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("approval get: %w", err)
	}
	tid, err := parseUUID(taskID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	id, err := parseUUID(approvalID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	var out *project.Approval
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		row, err := q.GetTaskApproval(ctx, gen.GetTaskApprovalParams{OrgID: oid, TaskID: tid, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		as := []project.Approval{approvalFrom(row)}
		if err := approversFor(ctx, q, oid, as); err != nil {
			return err
		}
		out = &as[0]
		return nil
	})
	return out, err
}

func (r *ApprovalRepo) ListByTask(ctx context.Context, orgID, taskID string) ([]project.Approval, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("approval list: %w", err)
	}
	tid, err := parseUUID(taskID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	out := []project.Approval{}
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		rows, err := q.ListTaskApprovals(ctx, gen.ListTaskApprovalsParams{OrgID: oid, TaskID: tid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, approvalFrom(row))
		}
		return approversFor(ctx, q, oid, out)
	})
	return out, err
}

func (r *ApprovalRepo) Update(ctx context.Context, orgID, taskID, approvalID string, fn func(*project.Approval) error) (*project.Approval, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("approval update: %w", err)
	}
	tid, err := parseUUID(taskID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	id, err := parseUUID(approvalID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	var out *project.Approval
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		row, err := q.LockTaskApproval(ctx, gen.LockTaskApprovalParams{OrgID: oid, TaskID: tid, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		as := []project.Approval{approvalFrom(row)}
		if err := approversFor(ctx, q, oid, as); err != nil {
			return err
		}
		a := &as[0]
		if err := fn(a); err != nil {
			return err
		}
		if err := q.UpdateTaskApprovalState(ctx, gen.UpdateTaskApprovalStateParams{
			OrgID: oid, ID: id, State: string(a.State), DecidedAt: nullTS(a.DecidedAt),
		}); err != nil {
			return err
		}
		for _, p := range a.Approvers {
			if p.Decision == "" {
				continue
			}
			uid, err := parseUUID(p.UserID)
			if err != nil {
				return err
			}
			dec := string(p.Decision)
			if err := q.SetApproverDecision(ctx, gen.SetApproverDecisionParams{
				OrgID: oid, ApprovalID: id, UserID: uid, Decision: &dec, Comment: p.Comment, DecidedAt: nullTS(p.DecidedAt),
			}); err != nil {
				return err
			}
		}
		out = a
		return nil
	})
	return out, err
}

func (r *ApprovalRepo) ListWaiting(ctx context.Context, orgID, userID string) ([]project.WaitingApproval, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("approval waiting: %w", err)
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("approval waiting: user: %w", err)
	}
	out := []project.WaitingApproval{}
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		rows, err := q.ListWaitingApprovals(ctx, gen.ListWaitingApprovalsParams{OrgID: oid, UserID: uid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, project.WaitingApproval{
				ApprovalID: row.ApprovalID.String(), TaskID: row.TaskID.String(), ProjectID: row.ProjectID.String(), TaskNumber: int(row.TaskNumber),
				TaskTitle: row.Title, RequestedBy: row.RequestedBy.String(), Deadline: tsPtr(row.Deadline),
				CreatedAt: row.CreatedAt,
			})
		}
		return nil
	})
	return out, err
}
