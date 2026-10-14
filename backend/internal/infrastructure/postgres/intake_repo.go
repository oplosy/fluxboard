package postgres

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// IntakeRepo is the Postgres-backed project.IntakeRepository. intake_requests
// is [T] (RLS): every op runs through TenantPool.WithTenant.
type IntakeRepo struct{ tp *TenantPool }

// NewIntakeRepo builds an IntakeRepo.
func NewIntakeRepo(tp *TenantPool) *IntakeRepo { return &IntakeRepo{tp: tp} }

var _ project.IntakeRepository = (*IntakeRepo)(nil)

// intakeUUID maps "" (or a malformed id) to SQL NULL.
func intakeUUID(s string) pgtype.UUID {
	id, err := parseUUID(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func (r *IntakeRepo) Submit(ctx context.Context, orgID string, in *project.IntakeRequest, dedupeWindow time.Duration, rateMax int, rateWindow time.Duration) (string, bool, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return "", false, fmt.Errorf("intake submit: %w", err)
	}
	id, err := parseUUID(in.ID)
	if err != nil {
		return "", false, fmt.Errorf("intake submit: %w", err)
	}
	pid, err := parseUUID(in.ProjectID)
	if err != nil {
		return "", false, fmt.Errorf("intake submit: project: %w", err)
	}
	form := intakeUUID(in.FormID)
	now := time.Now()
	resultID, created := in.ID, true
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		resultID, created = in.ID, true
		key := in.FormID + "|" + strings.ToLower(in.SubmitterEmail) + "|" + hex.EncodeToString(in.ContentHash)
		if err := q.LockIntakeDedupe(ctx, key); err != nil {
			return err
		}
		dup, err := q.FindRecentIntakeDuplicate(ctx, gen.FindRecentIntakeDuplicateParams{
			OrgID: oid, FormID: form, SubmitterEmail: in.SubmitterEmail,
			ContentHash: in.ContentHash, Since: now.Add(-dedupeWindow),
		})
		switch {
		case err == nil:
			resultID, created = dup.String(), false
			return nil
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		n, err := q.CountRecentIntakeForForm(ctx, gen.CountRecentIntakeForFormParams{
			OrgID: oid, FormID: form, Since: now.Add(-rateWindow),
		})
		if err != nil {
			return err
		}
		if int(n) >= rateMax {
			return domain.ErrRateLimited
		}
		return q.InsertIntakeRequest(ctx, gen.InsertIntakeRequestParams{
			ID: id, OrgID: oid, ProjectID: pid, FormID: form,
			FormSchemaVersion: int32(in.FormSchemaVersion), FormSnapshot: in.FormSnapshot,
			Title: in.Title, Description: in.Description, Priority: string(in.Priority),
			SubmitterName: in.SubmitterName, SubmitterEmail: in.SubmitterEmail,
			ContentHash: in.ContentHash, OwnerID: intakeUUID(in.OwnerID),
		})
	})
	if err != nil {
		return "", false, err
	}
	return resultID, created, nil
}

func intakeFrom(row gen.GetIntakeRequestRow) *project.IntakeRequest {
	return &project.IntakeRequest{
		ID: row.ID.String(), OrgID: row.OrgID.String(), ProjectID: row.ProjectID.String(),
		FormID: uuidStr(row.FormID), FormSchemaVersion: int(row.FormSchemaVersion), FormSnapshot: row.FormSnapshot,
		Title: row.Title, Description: row.Description, Priority: project.Priority(row.Priority),
		SubmitterName: row.SubmitterName, SubmitterEmail: row.SubmitterEmail,
		Status: project.IntakeStatus(row.Status), OwnerID: uuidStr(row.OwnerID), DeclineReason: row.DeclineReason,
		TaskID: uuidStr(row.TaskID), DecidedBy: uuidStr(row.DecidedBy), DecidedAt: tsPtr(row.DecidedAt),
		ReceivedAt: row.ReceivedAt, AckSentAt: tsPtr(row.AckSentAt), OutcomeSentAt: tsPtr(row.OutcomeSentAt),
	}
}

func (r *IntakeRepo) Get(ctx context.Context, orgID, id string) (*project.IntakeRequest, error) {
	rid, err := parseUUID(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	var out *project.IntakeRequest
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		row, err := q.GetIntakeRequest(ctx, gen.GetIntakeRequestParams{OrgID: oid, ID: rid})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = intakeFrom(row)
		return nil
	})
	return out, err
}

func (r *IntakeRepo) List(ctx context.Context, orgID, projectID string, status project.IntakeStatus, limit int) ([]project.IntakeRequest, error) {
	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	var out []project.IntakeRequest
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		rows, err := q.ListIntakeRequests(ctx, gen.ListIntakeRequestsParams{
			OrgID: oid, ProjectID: pid, Status: ptrOrNil(string(status)), Lim: int32(limit),
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, *intakeFrom(gen.GetIntakeRequestRow(row)))
		}
		return nil
	})
	return out, err
}

// guarded maps a zero-row status-guarded update to ErrConflict.
func guarded(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrConflict
	}
	return nil
}

func (r *IntakeRepo) Triage(ctx context.Context, orgID, id, ownerID string, priority project.Priority) error {
	rid, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		return guarded(q.TriageIntakeRequest(ctx, gen.TriageIntakeRequestParams{
			OrgID: oid, ID: rid, OwnerID: intakeUUID(ownerID), Priority: string(priority),
		}))
	})
}

func (r *IntakeRepo) Accept(ctx context.Context, orgID, id, taskID, by string) error {
	rid, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		return guarded(q.AcceptIntakeRequest(ctx, gen.AcceptIntakeRequestParams{
			OrgID: oid, ID: rid, TaskID: intakeUUID(taskID), DecidedBy: intakeUUID(by),
		}))
	})
}

func (r *IntakeRepo) Decline(ctx context.Context, orgID, id, reason, by string) error {
	rid, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		return guarded(q.DeclineIntakeRequest(ctx, gen.DeclineIntakeRequestParams{
			OrgID: oid, ID: rid, DeclineReason: reason, DecidedBy: intakeUUID(by),
		}))
	})
}

func (r *IntakeRepo) CompleteDone(ctx context.Context, orgID string) (int, error) {
	oid, _ := parseUUID(orgID)
	n := 0
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		ids, err := q.CompleteIntakeRequests(ctx, oid)
		n = len(ids)
		return err
	})
	return n, err
}

func (r *IntakeRepo) MailDue(ctx context.Context, orgID string, limit int) ([]project.IntakeMail, error) {
	oid, _ := parseUUID(orgID)
	var out []project.IntakeMail
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		rows, err := q.ListIntakeMailDue(ctx, gen.ListIntakeMailDueParams{OrgID: oid, Lim: int32(limit)})
		if err != nil {
			return err
		}
		for _, row := range rows {
			st := project.IntakeStatus(row.Status)
			out = append(out, project.IntakeMail{
				ID: row.ID.String(), Title: row.Title, SubmitterName: row.SubmitterName,
				SubmitterEmail: row.SubmitterEmail, FormName: row.FormName, Status: st, DeclineReason: row.DeclineReason,
				AckDue:     !row.AckSentAt.Valid,
				OutcomeDue: !row.OutcomeSentAt.Valid && (st == project.IntakeDeclined || st == project.IntakeDone),
			})
		}
		return nil
	})
	return out, err
}

func (r *IntakeRepo) MarkAcked(ctx context.Context, orgID, id string) error {
	rid, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		return q.MarkIntakeAcked(ctx, gen.MarkIntakeAckedParams{OrgID: oid, ID: rid})
	})
}

func (r *IntakeRepo) MarkOutcomeSent(ctx context.Context, orgID, id string) error {
	rid, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	oid, _ := parseUUID(orgID)
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		return q.MarkIntakeOutcomeSent(ctx, gen.MarkIntakeOutcomeSentParams{OrgID: oid, ID: rid})
	})
}
