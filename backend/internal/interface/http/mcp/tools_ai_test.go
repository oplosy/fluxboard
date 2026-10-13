package mcp

// ai.* tools contract (ADR-025, ADR-034): argument errors are ArgErrors (tool
// errors the model can fix), domain errors pass through as sentinels, happy
// paths return result maps. Runs each tool's Call over an aiuc.Service built on
// in-memory fakes + the real mock provider.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/admin"
	"github.com/mesutokul/fluxboard/backend/internal/domain/ai"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	infraai "github.com/mesutokul/fluxboard/backend/internal/infrastructure/ai"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/aiuc"
)

type mcpRuns struct {
	mu   sync.Mutex
	rows map[string]*ai.Run
}

func (f *mcpRuns) Create(_ context.Context, _ string, r *ai.Run) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]*ai.Run{}
	}
	cp := *r
	f.rows[r.ID] = &cp
	return nil
}

func (f *mcpRuns) Get(_ context.Context, _, id string) (*ai.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.rows[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}

func (f *mcpRuns) ListRecent(_ context.Context, _ string, _ int) ([]ai.Run, error) {
	return nil, nil
}

func (f *mcpRuns) CountSince(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

func (f *mcpRuns) PurgeBefore(_ context.Context, _ string, _ time.Time) (int64, error) {
	return 0, nil
}

type mcpRisks struct{}

func (mcpRisks) UpsertOpen(_ context.Context, _ string, _ *ai.Risk) error { return nil }
func (mcpRisks) ListOpen(_ context.Context, _, _ string) ([]ai.Risk, error) {
	return []ai.Risk{}, nil
}
func (mcpRisks) Dismiss(_ context.Context, _, _ string) error { return domain.ErrNotFound }

type mcpFlags struct{}

func (mcpFlags) List(_ context.Context, _ string) ([]admin.FeatureFlag, error) { return nil, nil }
func (mcpFlags) Set(_ context.Context, _, _ string, _ bool) error              { return nil }

type mcpTasks struct{ byID map[string]*project.Task }

func (f *mcpTasks) Get(_ context.Context, _, id string) (*project.Task, error) {
	if t, ok := f.byID[id]; ok {
		return t, nil
	}
	return nil, domain.ErrNotFound
}

func (f *mcpTasks) ListByProject(_ context.Context, _, _ string) ([]project.Task, error) {
	return nil, nil
}

func (f *mcpTasks) Create(_ context.Context, _ string, _ *project.Task) error { return nil }
func (f *mcpTasks) ListByColumn(_ context.Context, _, _ string) ([]project.Task, error) {
	return nil, nil
}
func (f *mcpTasks) Update(_ context.Context, _ string, _ *project.Task) error { return nil }
func (f *mcpTasks) Move(_ context.Context, _, _, _, _ string) error           { return nil }
func (f *mcpTasks) SoftDelete(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (f *mcpTasks) Restore(_ context.Context, _, _ string) error { return nil }
func (f *mcpTasks) GetTrashed(_ context.Context, _, _ string) (*project.Task, error) {
	return nil, domain.ErrNotFound
}
func (f *mcpTasks) ListTrashed(_ context.Context, _, _ string) ([]project.Task, error) {
	return nil, nil
}
func (f *mcpTasks) PurgeExpired(_ context.Context, _ string, _ time.Time) (int, error) {
	return 0, nil
}
func (f *mcpTasks) Search(_ context.Context, _ string, _ project.SearchFilter) (*project.SearchResult, error) {
	return &project.SearchResult{}, nil
}
func (f *mcpTasks) CountLive(_ context.Context, _ string, ids []string) (int, error) {
	return len(ids), nil
}
func (f *mcpTasks) BulkAssign(_ context.Context, _ string, _ []string, _ *string) error {
	return nil
}
func (f *mcpTasks) BulkMove(_ context.Context, _, _ string, _, _ []string) error { return nil }

type mcpProjects struct{}

func (mcpProjects) Get(_ context.Context, _, _ string) (*project.Project, error) {
	return nil, domain.ErrNotFound
}
func (mcpProjects) Create(_ context.Context, _ string, _ *project.Project) error { return nil }
func (mcpProjects) List(_ context.Context, _, _ string, _ project.Scope, _ bool) ([]project.Project, error) {
	return nil, nil
}
func (mcpProjects) Update(_ context.Context, _ string, _ *project.Project) error { return nil }
func (mcpProjects) SetArchived(_ context.Context, _, _ string, _ *time.Time) error {
	return nil
}
func (mcpProjects) NextNumber(_ context.Context, _, _ string) (int, error) { return 1, nil }

type mcpAudit struct{}

func (mcpAudit) Append(_ context.Context, _ audit.Entry) error { return nil }

type mcpIdem struct {
	mu sync.Mutex
	m  map[string]string
}

func (f *mcpIdem) Get(_ context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m[key], nil
}

func (f *mcpIdem) Set(_ context.Context, key, val string, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = map[string]string{}
	}
	f.m[key] = val
	return nil
}

type mcpCreator struct{ n int }

func (f *mcpCreator) CreateTask(_ context.Context, orgID, _ string, in aiuc.TaskCreateInput, _ tenant.OrgRole) (*project.Task, error) {
	f.n++
	return &project.Task{ID: "t-new", OrgID: orgID, ProjectID: in.ProjectID, Number: f.n, Title: in.Title}, nil
}

func (f *mcpCreator) TrashTask(_ context.Context, _, _, _ string, _ tenant.OrgRole) error {
	return nil
}

func aiHarness() map[string]*Tool {
	svc := aiuc.New(aiuc.Deps{
		Runs: &mcpRuns{}, Risks: mcpRisks{}, Flags: mcpFlags{},
		Tasks: &mcpTasks{byID: map[string]*project.Task{}}, Projects: mcpProjects{},
		Audit: mcpAudit{}, Provider: infraai.NewMock(), Idem: &mcpIdem{},
		Creator: &mcpCreator{},
	})
	tools := aiTools(svc)
	out := make(map[string]*Tool, len(tools))
	for i := range tools {
		out[tools[i].Name] = &tools[i]
	}
	return out
}

const (
	testProject = "01a13504-591e-72d4-98b3-1b2fdc2de33a"
	testColumn  = "01a13504-5951-7bd7-8645-72ebc7da24cd"
	missingID   = "00000000-0000-0000-0000-000000000001"
)

var aiCaller = Caller{OrgID: "o1", UserID: "u1", Role: tenant.RoleAdmin, CanWrite: true}

func isArgError(err error) bool {
	var ae *ArgError
	return errors.As(err, &ae)
}

func TestAIParse(t *testing.T) {
	tools := aiHarness()
	res, err := tools["ai.parse"].Call(context.Background(), aiCaller, Args{"input": "ship login. fix logout"})
	if err != nil {
		t.Fatal(err)
	}
	if res["items"] == nil {
		t.Fatalf("result = %#v", res)
	}
	if _, err := tools["ai.parse"].Call(context.Background(), aiCaller, Args{"input": "   "}); !isArgError(err) {
		t.Fatalf("blank input: want ArgError, got %v", err)
	}
}

func TestAIPlanApply(t *testing.T) {
	tools := aiHarness()
	apply := tools["ai.plan_apply"]
	if !apply.Write || !apply.Idempotent {
		t.Fatal("ai.plan_apply must be a write, idempotent tool")
	}
	args := Args{
		"project_id": testProject, "column_id": testColumn, "idempotency_key": "k1",
		"items": []any{
			map[string]any{"title": "one"},
			map[string]any{"title": "two", "priority": "high", "due_date": "2026-10-01T00:00:00Z"},
		},
	}
	res, err := apply.Call(context.Background(), aiCaller, args)
	if err != nil {
		t.Fatal(err)
	}
	if res["tasks"] == nil || res["run_id"] == nil {
		t.Fatalf("result = %#v", res)
	}
	for name, bad := range map[string]Args{
		"item not an object": {"project_id": testProject, "column_id": testColumn, "items": []any{"nope"}},
		"bad date":           {"project_id": testProject, "column_id": testColumn, "items": []any{map[string]any{"title": "x", "due_date": "yesterday"}}},
		"no items":           {"project_id": testProject, "column_id": testColumn},
		"no project":         {"column_id": testColumn, "items": []any{map[string]any{"title": "x"}}},
		"bad assignee":       {"project_id": testProject, "column_id": testColumn, "items": []any{map[string]any{"title": "x", "assignee_id": "bob"}}},
	} {
		if _, err := apply.Call(context.Background(), aiCaller, bad); !isArgError(err) {
			t.Fatalf("%s: want ArgError, got %v", name, err)
		}
	}
}

func TestAIIdempotencyKeyFromHeader(t *testing.T) {
	tools := aiHarness()
	c := aiCaller
	c.IdempotencyKey = "hdr-1"
	args := Args{"project_id": testProject, "column_id": testColumn, "items": []any{map[string]any{"title": "one"}}}
	first, err := tools["ai.plan_apply"].Call(context.Background(), c, args)
	if err != nil {
		t.Fatal(err)
	}
	again, err := tools["ai.plan_apply"].Call(context.Background(), c, args)
	if err != nil {
		t.Fatal(err)
	}
	if again["replayed"] != true || again["run_id"] != first["run_id"] {
		t.Fatalf("the Idempotency-Key header must make a retry replay: %v vs %v", first, again)
	}
}

func TestAINotFoundPaths(t *testing.T) {
	tools := aiHarness()
	if _, err := tools["ai.digest"].Call(context.Background(), aiCaller, Args{"project_id": missingID}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("digest: got %v", err)
	}
	if _, err := tools["ai.risks_dismiss"].Call(context.Background(), aiCaller, Args{"task_id": missingID}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("dismiss: got %v", err)
	}
	// A value that is not an id at all is an argument error, not a lookup.
	if _, err := tools["ai.digest"].Call(context.Background(), aiCaller, Args{"project_id": "AGT"}); !isArgError(err) {
		t.Fatalf("non-uuid project_id: want ArgError, got %v", err)
	}
	msg, ok := toolErrorMessage(domain.ErrNotFound)
	if !ok || msg == "" {
		t.Fatal("not-found must map to an actionable tool error")
	}
}
