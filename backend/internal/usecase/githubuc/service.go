// Package githubuc implements GitHub pull-request linking (FR-TASK-016,
// ADR-039): per-org webhook settings for ADMINs and the signed webhook that
// links pull requests to tasks by key and optionally moves them on merge.
package githubuc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/github"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// SecretCipher encrypts the webhook secret at rest (pkg/aesgcm.Cipher).
type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// Tasks is the slice of the task service this feature needs.
type Tasks interface {
	CanViewTask(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) error
	MoveToDone(ctx context.Context, orgID, actorID, taskID string) error
}

// Deps wires the Service. Cipher may be nil: the integration is then disabled.
type Deps struct {
	Repo   github.Repository
	Orgs   tenant.OrgRepository
	Tasks  Tasks
	Cipher SecretCipher
	Logger *slog.Logger
}

// Service is the GitHub integration usecase.
type Service struct {
	repo   github.Repository
	orgs   tenant.OrgRepository
	tasks  Tasks
	cipher SecretCipher
	logger *slog.Logger
}

// New builds a Service.
func New(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{repo: d.Repo, orgs: d.Orgs, tasks: d.Tasks, cipher: d.Cipher, logger: d.Logger}
}

// Settings is what the settings page shows. The secret is never included.
type Settings struct {
	Configured   bool
	Repos        []string
	MoveProjects []string
}

// GetSettings returns the org's integration settings (ADMIN, gated by the route).
func (s *Service) GetSettings(ctx context.Context, orgID string) (*Settings, error) {
	in, err := s.repo.Get(ctx, orgID)
	if errors.Is(err, domain.ErrNotFound) {
		return &Settings{Repos: []string{}, MoveProjects: []string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return &Settings{Configured: true, Repos: nonNil(in.Repos), MoveProjects: nonNil(in.MoveProjects)}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// RotateSecret generates a new webhook secret, stores it encrypted and returns
// the plaintext once. Creating and rotating are the same call; the previous
// secret stops working immediately.
func (s *Service) RotateSecret(ctx context.Context, orgID, actorID string) (string, error) {
	if s.cipher == nil {
		return "", domain.NewCodedError(domain.ErrForbidden, "github_disabled", "GitHub integration is disabled on this server")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("github secret: %w", err)
	}
	secret := hex.EncodeToString(raw)
	enc, err := s.cipher.Encrypt(secret)
	if err != nil {
		return "", fmt.Errorf("github secret: %w", err)
	}
	if err := s.repo.SaveSecret(ctx, orgID, enc, actorID); err != nil {
		return "", err
	}
	return secret, nil
}

// UpdateSettings replaces the repository allow-list and the projects whose
// linked tasks move to done on merge.
func (s *Service) UpdateSettings(ctx context.Context, orgID string, repos, moveProjects []string) (*Settings, error) {
	if len(repos) > github.MaxRepos {
		return nil, domain.NewFieldError("repos", "too_many", "too many repositories")
	}
	clean := make([]string, 0, len(repos))
	seen := map[string]bool{}
	for _, r := range repos {
		r = strings.ToLower(strings.TrimSpace(r))
		if !github.ValidRepo(r) {
			return nil, domain.NewFieldError("repos", "invalid_repo", "repositories must look like owner/name")
		}
		if !seen[r] {
			seen[r] = true
			clean = append(clean, r)
		}
	}
	if err := s.repo.SaveSettings(ctx, orgID, clean, moveProjects); err != nil {
		return nil, err
	}
	return s.GetSettings(ctx, orgID)
}

// Disconnect removes the integration (secret, allow-list, options). Linked PRs stay.
func (s *Service) Disconnect(ctx context.Context, orgID string) error {
	return s.repo.Delete(ctx, orgID)
}

// ListForTask returns the pull requests linked to a task (project VIEWER+).
func (s *Service) ListForTask(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) ([]github.LinkedPR, error) {
	if err := s.tasks.CanViewTask(ctx, orgID, userID, taskID, orgRole); err != nil {
		return nil, err
	}
	return s.repo.ListByTask(ctx, orgID, taskID)
}

// Delivery is one webhook request as received.
type Delivery struct {
	OrgRef     string // org slug or id from the URL
	Event      string // X-GitHub-Event
	DeliveryID string // X-GitHub-Delivery
	Signature  string // X-Hub-Signature-256
	Body       []byte
}

// Outcome of an accepted delivery.
type Outcome string

const (
	OutcomePong      Outcome = "pong"
	OutcomeIgnored   Outcome = "ignored"   // action or repository we do not act on
	OutcomeDuplicate Outcome = "duplicate" // delivery id already processed
	OutcomeProcessed Outcome = "processed"
)

// ErrUnsupportedEvent is returned for an event type other than ping/pull_request.
var ErrUnsupportedEvent = errors.New("github: unsupported event")

// HandleWebhook authenticates and processes one delivery. The signature is
// checked before the payload is parsed; unknown org, no integration and a bad
// signature all return domain.ErrUnauthorized so nothing about the org leaks.
func (s *Service) HandleWebhook(ctx context.Context, d Delivery) (Outcome, error) {
	if s.cipher == nil {
		return "", domain.ErrUnauthorized
	}
	org, err := s.resolveOrg(ctx, d.OrgRef)
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	in, err := s.repo.Get(ctx, org.ID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.ErrUnauthorized
		}
		return "", err
	}
	secret, err := s.cipher.Decrypt(in.SecretEnc)
	if err != nil {
		return "", fmt.Errorf("github webhook: decrypt secret: %w", err)
	}
	if !github.VerifySignature(secret, d.Body, d.Signature) {
		return "", domain.ErrUnauthorized
	}

	switch d.Event {
	case github.EventPing:
		return OutcomePong, nil
	case github.EventPullRequest:
	default:
		return "", ErrUnsupportedEvent
	}
	if d.DeliveryID == "" || len(d.DeliveryID) > 100 {
		return "", domain.ErrValidation
	}
	ev, err := github.ParsePullRequest(d.Body)
	if err != nil {
		return "", domain.ErrValidation
	}
	if !github.Handled(ev.Action) || !in.AllowsRepo(ev.Repo) {
		return OutcomeIgnored, nil
	}

	var refs []github.TaskRef
	if keys := github.ExtractKeys(ev.Title, ev.HeadRef); len(keys) > 0 {
		if refs, err = s.repo.ResolveTasks(ctx, org.ID, keys); err != nil {
			return "", err
		}
	}
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		ids = append(ids, r.TaskID)
	}
	dup, err := s.repo.Apply(ctx, org.ID, d.DeliveryID, ev, ids)
	if err != nil {
		return "", err
	}
	if dup {
		return OutcomeDuplicate, nil
	}

	// Moving is best effort after the links are durable: a task with open
	// blockers (ErrConflict) simply stays where it is.
	if ev.Action == "closed" && ev.Merged() {
		for _, r := range refs {
			if !in.MovesOnMerge(r.ProjectID) {
				continue
			}
			if err := s.tasks.MoveToDone(ctx, org.ID, in.CreatedBy, r.TaskID); err != nil {
				s.logger.WarnContext(ctx, "github merge move skipped", "org", org.ID, "task", r.TaskID, "err", err)
			}
		}
	}
	return OutcomeProcessed, nil
}

func (s *Service) resolveOrg(ctx context.Context, ref string) (*tenant.Organization, error) {
	var (
		org *tenant.Organization
		err error
	)
	if _, perr := uuid.Parse(ref); perr == nil {
		org, err = s.orgs.GetByID(ctx, ref)
	} else {
		org, err = s.orgs.GetBySlug(ctx, ref)
	}
	if err != nil {
		return nil, err
	}
	if org.Deleted() {
		return nil, domain.ErrNotFound
	}
	return org, nil
}
