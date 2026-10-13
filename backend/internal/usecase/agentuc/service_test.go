package agentuc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/agent"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// fakeRepo is an in-memory agent.Repository with the real repo's name rule:
// live names are unique per org, case-insensitively.
type fakeRepo struct {
	agents map[string]agent.Agent
}

func newFakeRepo() *fakeRepo { return &fakeRepo{agents: map[string]agent.Agent{}} }

func (f *fakeRepo) nameTaken(orgID, id, name string) bool {
	for _, a := range f.agents {
		if a.OrgID == orgID && a.ID != id && strings.EqualFold(a.Name, name) {
			return true
		}
	}
	return false
}

func (f *fakeRepo) Create(_ context.Context, a agent.Agent, key apikey.APIKey) error {
	if f.nameTaken(a.OrgID, a.ID, a.Name) {
		return domain.ErrConflict
	}
	a.Keys = []apikey.APIKey{key}
	f.agents[a.ID] = a
	return nil
}
func (f *fakeRepo) List(_ context.Context, orgID string) ([]agent.Agent, error) {
	var out []agent.Agent
	for _, a := range f.agents {
		if a.OrgID == orgID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, orgID, id string) (agent.Agent, error) {
	a, ok := f.agents[id]
	if !ok || a.OrgID != orgID {
		return agent.Agent{}, domain.ErrNotFound
	}
	return a, nil
}
func (f *fakeRepo) Count(ctx context.Context, orgID string) (int64, error) {
	as, _ := f.List(ctx, orgID)
	return int64(len(as)), nil
}
func (f *fakeRepo) Update(_ context.Context, a agent.Agent) error {
	if f.nameTaken(a.OrgID, a.ID, a.Name) {
		return domain.ErrConflict
	}
	f.agents[a.ID] = a
	return nil
}
func (f *fakeRepo) Delete(_ context.Context, orgID, id string) error {
	if a, ok := f.agents[id]; !ok || a.OrgID != orgID {
		return domain.ErrNotFound
	}
	delete(f.agents, id)
	return nil
}
func (f *fakeRepo) AddKey(_ context.Context, orgID, agentID string, key apikey.APIKey) error {
	a, ok := f.agents[agentID]
	if !ok || a.OrgID != orgID {
		return domain.ErrNotFound
	}
	a.Keys = append(a.Keys, key)
	f.agents[agentID] = a
	return nil
}
func (f *fakeRepo) RevokeKey(_ context.Context, orgID, agentID, keyID string) error {
	a, ok := f.agents[agentID]
	if !ok || a.OrgID != orgID {
		return domain.ErrNotFound
	}
	for i, k := range a.Keys {
		if k.ID == keyID {
			a.Keys = append(a.Keys[:i], a.Keys[i+1:]...)
			f.agents[agentID] = a
			return nil
		}
	}
	return domain.ErrNotFound
}

type fakeLimits struct{ max int }

func (f fakeLimits) Resolve(context.Context, string) (billing.Entitlements, error) {
	return billing.Entitlements{MaxAgents: f.max}, nil
}

type fakeAudit struct{ actions []audit.Action }

func (f *fakeAudit) Append(_ context.Context, e audit.Entry) error {
	f.actions = append(f.actions, e.Action)
	return nil
}

func newSvc() (*Service, *fakeRepo, *fakeAudit) {
	repo, au := newFakeRepo(), &fakeAudit{}
	return New(Deps{Agents: repo, Limits: fakeLimits{max: 10}, Audit: au}), repo, au
}

func codeOf(err error) string {
	var ce *domain.CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

func TestCreateMintsAgentWithKey(t *testing.T) {
	svc, repo, au := newSvc()
	c, err := svc.Create(context.Background(), "org1", "u1", CreateInput{Name: "  backend-2 "})
	if err != nil {
		t.Fatal(err)
	}
	a := c.Agent
	if a.Name != "backend-2" || a.Role != tenant.RoleMember || a.Status != agent.StatusActive || a.CreatedBy != "u1" {
		t.Fatalf("agent = %+v", a)
	}
	if !apikey.LooksLikeKey(c.Plaintext) || apikey.Hash(c.Plaintext) != c.Key.KeyHash {
		t.Fatal("plaintext must be a key whose hash is stored")
	}
	if c.Key.AgentID != a.ID || !c.Key.HasScope(apikey.ScopeWrite) {
		t.Fatalf("key must belong to the agent and carry write: %+v", c.Key)
	}
	if stored := repo.agents[a.ID]; len(stored.Keys) != 1 {
		t.Fatalf("stored agent should hold its first key: %+v", stored)
	}
	if len(au.actions) != 1 || au.actions[0] != audit.ActionAgentCreate {
		t.Fatalf("audit = %v", au.actions)
	}
}

func TestCreateValidates(t *testing.T) {
	svc, _, _ := newSvc()
	ctx := context.Background()
	if _, err := svc.Create(ctx, "org1", "u1", CreateInput{Name: "a/b"}); codeOf(err) != "agent_name_invalid" {
		t.Fatalf("bad name: %v", err)
	}
	if _, err := svc.Create(ctx, "org1", "u1", CreateInput{Name: "ok", Role: tenant.RoleAdmin}); codeOf(err) != "agent_role_invalid" {
		t.Fatalf("admin role must be refused: %v", err)
	}
	if _, err := svc.Create(ctx, "org1", "u1", CreateInput{Name: "qa", Role: tenant.RoleGuest}); err != nil {
		t.Fatalf("guest role: %v", err)
	}
	_, err := svc.Create(ctx, "org1", "u1", CreateInput{Name: "QA"})
	if codeOf(err) != "agent_name_taken" || !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate name: %v", err)
	}
}

func TestUpdatePauseRenameAndNoop(t *testing.T) {
	svc, _, au := newSvc()
	ctx := context.Background()
	c, _ := svc.Create(ctx, "org1", "u1", CreateInput{Name: "one"})
	_, _ = svc.Create(ctx, "org1", "u1", CreateInput{Name: "two"})

	paused := agent.StatusPaused
	name := "uno"
	a, err := svc.Update(ctx, "org1", "u1", c.Agent.ID, UpdateInput{Name: &name, Status: &paused})
	if err != nil || a.Name != "uno" || a.Status != agent.StatusPaused {
		t.Fatalf("update = %+v, %v", a, err)
	}
	taken := "TWO"
	if _, err := svc.Update(ctx, "org1", "u1", c.Agent.ID, UpdateInput{Name: &taken}); codeOf(err) != "agent_name_taken" {
		t.Fatalf("rename onto another agent: %v", err)
	}
	bogus := agent.Status("sleeping")
	if _, err := svc.Update(ctx, "org1", "u1", c.Agent.ID, UpdateInput{Status: &bogus}); codeOf(err) != "agent_status_invalid" {
		t.Fatalf("bad status: %v", err)
	}
	owner := tenant.RoleOwner
	if _, err := svc.Update(ctx, "org1", "u1", c.Agent.ID, UpdateInput{Role: &owner}); codeOf(err) != "agent_role_invalid" {
		t.Fatalf("owner role must be refused: %v", err)
	}
	n := len(au.actions)
	if _, err := svc.Update(ctx, "org1", "u1", c.Agent.ID, UpdateInput{Name: &name}); err != nil || len(au.actions) != n {
		t.Fatalf("a no-op update must not audit: %v, %v", err, au.actions)
	}
	if _, err := svc.Update(ctx, "org2", "u1", c.Agent.ID, UpdateInput{Name: &name}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("another org's agent: %v", err)
	}
}

func TestKeysAreCapped(t *testing.T) {
	svc, _, _ := newSvc()
	ctx := context.Background()
	c, _ := svc.Create(ctx, "org1", "u1", CreateInput{Name: "worker"})
	for i := 1; i < agent.MaxKeys; i++ {
		if _, err := svc.CreateKey(ctx, "org1", "u1", c.Agent.ID); err != nil {
			t.Fatalf("key %d: %v", i+1, err)
		}
	}
	if _, err := svc.CreateKey(ctx, "org1", "u1", c.Agent.ID); codeOf(err) != "agent_key_limit" {
		t.Fatalf("over the key cap: %v", err)
	}
	if err := svc.RevokeKey(ctx, "org1", "u1", c.Agent.ID, c.Key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateKey(ctx, "org1", "u1", c.Agent.ID); err != nil {
		t.Fatalf("revoking frees a key slot: %v", err)
	}
}

func TestListReportsCap(t *testing.T) {
	svc, _, _ := newSvc()
	ctx := context.Background()
	_, _ = svc.Create(ctx, "org1", "u1", CreateInput{Name: "a"})
	v, err := svc.List(ctx, "org1")
	if err != nil || len(v.Agents) != 1 || v.Max != 10 {
		t.Fatalf("list = %+v, %v", v, err)
	}
	if err := svc.Delete(ctx, "org1", "u1", v.Agents[0].ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := svc.Count(ctx, "org1"); n != 0 {
		t.Fatalf("delete frees the slot: count = %d", n)
	}
}
