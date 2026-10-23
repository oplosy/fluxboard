//go:build integration

// Tenant-isolation suite (docs/12-TESTING.md §6). It runs the real API wiring
// (newApp) in-process against Postgres (RLS on, app role), Redis and S3 with
// the demo fixture loaded — org A = acme, org B = beta-co — and checks:
//
//   - route matrix: every route under /orgs/{orgId}, introspected from the
//     router, is called by principals of org A (member, admin) and by an
//     outsider (no org). Through org B's own path the tenant guard must answer
//     403/404. Through org A's path with org B's resource IDs the API must not
//     succeed (no 2xx/3xx), crash (5xx) or leak B's marker. Org B's rows must be
//     unchanged afterwards. A route with a path parameter the suite has no
//     org-B resource for fails the suite, so new routes cannot slip through;
//   - resource IDs passed in request bodies (bulk actions, AI risk scan/dismiss)
//     and query strings (search, risk list) get the same treatment;
//   - raw RLS: without app.current_tenant every RLS table returns 0 rows, a
//     tenant never sees another org's rows, and an INSERT carrying another
//     org's org_id is rejected by the policy's WITH CHECK;
//   - search never returns another org's or a hidden private project's marker,
//     with positive controls proving the markers are searchable;
//   - an org GUEST sees only the projects it is a member of, never the other
//     'org'-visible ones (FR-TEN-010), in lists, search and direct reads;
//   - a foreign attachment download 404s before any presign.
//
// Env (missing → fail, never skip): TEST_DATABASE_URL (fluxboard_app),
// TEST_DATABASE_OWNER_URL (fluxboard_owner, loads the fixture), TEST_REDIS_ADDR,
// TEST_S3_ENDPOINT / TEST_S3_ACCESS_KEY / TEST_S3_SECRET_KEY. Like `make seed`,
// it resets the demo orgs in the target database.
package main

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/config"
	"github.com/mesutokul/fluxboard/backend/internal/domain/github"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/demoseed"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/planseed"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/aesgcm"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/argon2x"
)

// isoTOTPKey is a fresh 32-byte AES key (base64) per run that enables the
// GitHub integration's secret cipher; generated so no key literal is committed.
var isoTOTPKey = func() string {
	b := make([]byte, 32)
	if _, err := crand.Read(b); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}()

// Org B's GitHub webhook secret (stored encrypted in its seeded integration).
const orgBGithubSecret = "org-b-webhook-secret"

// Marker carried by org B's seeded rows; it must never reach an org A caller.
const orgBMarker = "betaco-marker"

// Principals attacking org B: two org A roles and a user in no org.
var attackers = []string{"member@demo.dev", "admin@demo.dev", "outsider@demo.dev"}

type isoRoute struct{ method, pattern string }

type isoEnv struct {
	h          http.Handler
	routes     []isoRoute
	app, owner *pgxpool.Pool
	orgA, orgB string
	bParams    map[string]string // route param → org-B resource ID
	bIDs       map[string]string // named org-B resources for targeted checks
	aWebTask   string
	bGithubEnc string // orgBGithubSecret, encrypted
	tokens     map[string]string
	ip         string
}

func TestTenantIsolation(t *testing.T) {
	e := setupIsolation(t)

	t.Run("positive controls", e.testPositiveControls)
	before := e.snapshotOrg(t, e.orgB)
	t.Run("route matrix via org B path", e.testGuardMatrix)
	t.Run("route matrix via org A path with org B IDs", e.testDataMatrix)
	t.Run("org B IDs in bodies and queries", e.testBodyReferences)
	after := e.snapshotOrg(t, e.orgB)
	t.Run("org B rows unchanged", func(t *testing.T) { compareSnapshots(t, before, after) })
	t.Run("raw RLS fails closed", e.testRawRLS)
	t.Run("RLS WITH CHECK rejects foreign org_id", e.testWithCheck)
	t.Run("search never crosses orgs or private projects", e.testSearch)
	t.Run("guest sees only its own projects", e.testGuestVisibility)
	t.Run("foreign attachment download 404s before presign", e.testAttachmentDownload)
	t.Run("github webhook stays inside its org", e.testGithubWebhook)
}

// ---- setup -------------------------------------------------------------------

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Fatalf("%s not set; the tenant-isolation suite needs Postgres, Redis and S3 (see make test-integration)", key)
	}
	return v
}

func setupIsolation(t *testing.T) *isoEnv {
	t.Helper()
	ctx := context.Background()
	appDSN := requireEnv(t, "TEST_DATABASE_URL")
	ownerDSN := requireEnv(t, "TEST_DATABASE_OWNER_URL")
	redisAddr := requireEnv(t, "TEST_REDIS_ADDR")
	s3 := requireEnv(t, "TEST_S3_ENDPOINT")

	owner, err := pgxpool.New(ctx, ownerDSN)
	if err != nil {
		t.Fatalf("owner pool: %v", err)
	}
	t.Cleanup(owner.Close)
	app, err := pgxpool.New(ctx, appDSN)
	if err != nil {
		t.Fatalf("app pool: %v", err)
	}
	t.Cleanup(app.Close)

	y, m, d := time.Now().UTC().Date()
	fx := demoseed.Build(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
	hash, err := argon2x.Hash(demoseed.DemoPassword)
	if err != nil {
		t.Fatal(err)
	}

	e := &isoEnv{
		app: app, owner: owner,
		orgA: demoseed.ID("org", "acme"), orgB: demoseed.ID("org", "beta-co"),
		tokens: map[string]string{},
		ip:     fmt.Sprintf("198.51.100.%d", rand.IntN(250)+1), // fresh login-throttle bucket per run
	}
	bProject := demoseed.ID("project", e.orgB, "BETA")
	e.aWebTask = demoseed.ID("task", demoseed.ID("project", e.orgA, "WEB"), "1")
	e.bIDs = map[string]string{
		"project": bProject,
		"task":    demoseed.ID("task", bProject, "1"),
		"task2":   demoseed.ID("task", bProject, "2"),
		"column":  demoseed.ID("column", bProject, "Todo"),
		"label":   demoseed.ID("label", e.orgB, "bug"),
		"member":  demoseed.ID("user", "beta.member"),
		"owner":   demoseed.ID("user", "beta.owner"),
	}
	for _, k := range []string{"comment", "subtask", "attachment", "sprint", "epic", "release", "wiki", "goal", "view", "field", "form", "request", "rule", "entry", "invitation", "notification", "apikey", "agent", "agentkey", "approval"} {
		e.bIDs[k] = demoseed.ID("isolation", k)
	}
	key, err := aesgcm.ParseKey(isoTOTPKey)
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := aesgcm.New(key)
	if err != nil {
		t.Fatal(err)
	}
	if e.bGithubEnc, err = cipher.Encrypt(orgBGithubSecret); err != nil {
		t.Fatal(err)
	}
	e.bParams = map[string]string{
		"projectId": e.bIDs["project"], "taskId": e.bIDs["task"], "linkedId": e.bIDs["task2"],
		"columnId": e.bIDs["column"], "labelId": e.bIDs["label"], "userId": e.bIDs["member"],
		"commentId": e.bIDs["comment"], "subtaskId": e.bIDs["subtask"], "attachmentId": e.bIDs["attachment"],
		"sprintId": e.bIDs["sprint"], "epicId": e.bIDs["epic"], "releaseId": e.bIDs["release"], "wikiId": e.bIDs["wiki"], "goalId": e.bIDs["goal"], "viewId": e.bIDs["view"], "fieldId": e.bIDs["field"], "formId": e.bIDs["form"], "requestId": e.bIDs["request"],
		"ruleId": e.bIDs["rule"], "entryId": e.bIDs["entry"],
		"agentId": e.bIDs["agent"], "keyId": e.bIDs["agentkey"], "approvalId": e.bIDs["approval"],
	}

	if err := pgx.BeginFunc(ctx, owner, func(tx pgx.Tx) error {
		if err := planseed.Upsert(ctx, tx); err != nil {
			return err
		}
		if err := demoseed.Write(ctx, tx, fx, hash, ""); err != nil {
			return err
		}
		return insertOrgBResources(ctx, tx, e)
	}); err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	cfg := &config.Config{
		AppEnv: config.EnvDev, DatabaseURL: appDSN, DatabaseURLMigrate: ownerDSN, RedisAddr: redisAddr,
		WebOrigin: "http://localhost:3000", GoogleRedirectURL: "http://localhost:8080/api/v1/auth/oauth/google/callback",
		StripeMode: "stub", StripeWebhookSecret: "whsec_isolation",
		MinIOEndpoint: s3, MinIOAccessKey: os.Getenv("TEST_S3_ACCESS_KEY"), MinIOSecretKey: os.Getenv("TEST_S3_SECRET_KEY"),
		MinIOBucket: "fluxboard-isolation", SMTPPort: 1025, SMTPFrom: "no-reply@fluxboard.local",
		AIProvider: "mock", AIMonthlyCap: 1000, TOTPEncKey: isoTOTPKey,
	}
	a, err := newApp(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newApp: %v", err)
	}
	t.Cleanup(a.Close)
	e.h = a.handler

	routes, ok := a.handler.(chi.Routes)
	if !ok {
		t.Fatal("router does not expose chi.Routes")
	}
	if err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/api/v1/orgs/{orgId}") {
			e.routes = append(e.routes, isoRoute{method, route})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(e.routes) < 50 {
		t.Fatalf("introspected only %d org routes; the router walk is broken", len(e.routes))
	}

	for _, email := range append(attackers, "user01@demo.dev", "beta.owner@demo.dev", "guest@demo.dev") {
		e.tokens[email] = e.login(t, email)
	}
	return e
}

// insertOrgBResources gives org B one row of every resource a route parameter
// can name, so the data matrix probes real foreign IDs rather than random ones.
func insertOrgBResources(ctx context.Context, tx pgx.Tx, e *isoEnv) error {
	b, owner := e.bIDs, e.bIDs["owner"]
	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO comments (id, org_id, task_id, author_id, body) VALUES ($1, $2, $3, $4, $5)`,
			[]any{b["comment"], e.orgB, b["task"], owner, orgBMarker + " comment"}},
		{`INSERT INTO subtasks (id, org_id, task_id, title, rank) VALUES ($1, $2, $3, $4, 'i')`,
			[]any{b["subtask"], e.orgB, b["task"], orgBMarker + " subtask"}},
		{`INSERT INTO attachments (id, org_id, task_id, uploader_id, object_key, filename, content_type, size_bytes, status, confirmed_at)
		  VALUES ($1, $2, $3, $4, 'attachments/isolation/beta.txt', 'beta.txt', 'text/plain', 5, 'committed', now())`,
			[]any{b["attachment"], e.orgB, b["task"], owner}},
		{`INSERT INTO sprints (id, org_id, project_id, name, created_by) VALUES ($1, $2, $3, 'Beta sprint', $4)`,
			[]any{b["sprint"], e.orgB, b["project"], owner}},
		{`INSERT INTO epics (id, org_id, project_id, name, created_by) VALUES ($1, $2, $3, 'Beta epic', $4)`,
			[]any{b["epic"], e.orgB, b["project"], owner}},
		{`INSERT INTO releases (id, org_id, project_id, name, created_by) VALUES ($1, $2, $3, 'Beta release', $4)`,
			[]any{b["release"], e.orgB, b["project"], owner}},
		{`INSERT INTO wiki_pages (id, org_id, project_id, title, body, created_by, updated_by) VALUES ($1, $2, $3, 'Beta page', 'Beta WEB-1', $4, $4)`,
			[]any{b["wiki"], e.orgB, b["project"], owner}},
		{`INSERT INTO wiki_page_tasks (org_id, project_id, page_id, task_number) VALUES ($1, $2, $3, 1)`,
			[]any{e.orgB, b["project"], b["wiki"]}},
		{`INSERT INTO github_integrations (org_id, secret_enc, repos, created_by) VALUES ($1, $2, '{betaco/app}', $3)`,
			[]any{e.orgB, e.bGithubEnc, owner}},
		{`INSERT INTO github_merge_projects (org_id, project_id) VALUES ($1, $2)`,
			[]any{e.orgB, b["project"]}},
		{`INSERT INTO task_pull_requests (org_id, task_id, repo, number, title, state, url, pr_updated_at)
		  VALUES ($1, $2, 'betaco/app', 1, $3, 'open', 'https://github.com/betaco/app/pull/1', now())`,
			[]any{e.orgB, b["task"], orgBMarker + " pull request"}},
		{`INSERT INTO github_deliveries (org_id, delivery_id) VALUES ($1, 'isolation-delivery')`,
			[]any{e.orgB}},
		{`INSERT INTO goals (id, org_id, title, owner_id) VALUES ($1, $2, $4, $3)`,
			[]any{b["goal"], e.orgB, owner, orgBMarker + " goal"}},
		{`INSERT INTO goal_links (id, org_id, goal_id, epic_id) VALUES ($1, $2, $3, $4)`,
			[]any{demoseed.ID("isolation", "goal-link"), e.orgB, b["goal"], b["epic"]}},
		{`INSERT INTO saved_views (id, org_id, project_id, owner_id, name, shared) VALUES ($1, $2, $3, $4, 'Beta view', true)`,
			[]any{b["view"], e.orgB, b["project"], owner}},
		{`INSERT INTO custom_fields (id, org_id, project_id, name, type, created_by) VALUES ($1, $2, $3, 'Beta field', 'text', $4)`,
			[]any{b["field"], e.orgB, b["project"], owner}},
		{`INSERT INTO project_forms (id, org_id, project_id, name, target_column_id, token_hash, created_by)
		  VALUES ($1, $2, $3, 'Beta form', $4, $5, $6)`,
			[]any{b["form"], e.orgB, b["project"], b["column"], []byte("isolation-form-token-hash"), owner}},
		{`INSERT INTO intake_requests (id, org_id, project_id, form_id, form_schema_version, title, content_hash)
		  VALUES ($1, $2, $3, $4, 1, 'Beta request', $5)`,
			[]any{b["request"], e.orgB, b["project"], b["form"], []byte("isolation-request-hash")}},
		{`INSERT INTO automation_rules (id, org_id, name, trigger, action, action_config, created_by)
		  VALUES ($1, $2, 'Beta rule', 'task.created', 'set_priority', '{"priority":"high"}', $3)`,
			[]any{b["rule"], e.orgB, owner}},
		{`INSERT INTO invitations (id, org_id, email, role, token_hash, invited_by, expires_at)
		  VALUES ($1, $2, 'invitee@beta.example', 'MEMBER', $3, $4, now() + interval '7 days')`,
			[]any{b["invitation"], e.orgB, []byte("isolation-invite-token-hash"), owner}},
		{`INSERT INTO time_entries (id, org_id, task_id, user_id, started_at, ended_at) VALUES ($1, $2, $3, $4, now() - interval '1 hour', now())`,
			[]any{b["entry"], e.orgB, b["task"], owner}},
		{`INSERT INTO task_links (task_id, linked_task_id, org_id) VALUES ($1, $2, $3)`,
			[]any{b["task"], b["task2"], e.orgB}},
		{`INSERT INTO task_approvals (id, org_id, task_id, requested_by) VALUES ($1, $2, $3, $4)`,
			[]any{b["approval"], e.orgB, b["task"], owner}},
		{`INSERT INTO task_approval_approvers (approval_id, org_id, user_id) VALUES ($1, $2, $3)`,
			[]any{b["approval"], e.orgB, b["member"]}},
		{`INSERT INTO notifications (id, org_id, user_id, category, title) VALUES ($1, $2, $3, 'assignment', $4)`,
			[]any{b["notification"], e.orgB, b["member"], orgBMarker + " notification"}},
		{`INSERT INTO api_keys (id, org_id, prefix, key_hash, name, scopes, created_by) VALUES ($1, $2, 'fbk_live_iso', 'isolation-key-hash', 'Beta key', '{read}', $3)`,
			[]any{b["apikey"], e.orgB, owner}},
		// An agent is a users row plus an agents row, with one key (ADR-033).
		{`INSERT INTO users (id, email, name, email_verified) VALUES ($1, 'agent-iso@agents.fluxboard.invalid', $2, true)
		  ON CONFLICT (id) DO NOTHING`, // users is global: it outlives a re-seeded org B
			[]any{b["agent"], orgBMarker + " agent"}},
		{`INSERT INTO agents (id, org_id, name, created_by) VALUES ($1, $2, $3, $4)`,
			[]any{b["agent"], e.orgB, orgBMarker + " agent", owner}},
		{`INSERT INTO api_keys (id, org_id, prefix, key_hash, name, scopes, created_by, agent_id)
		  VALUES ($1, $2, 'fbk_live_isa', 'isolation-agent-key-hash', 'Beta agent', '{read,write}', $3, $4)`,
			[]any{b["agentkey"], e.orgB, owner, b["agent"]}},
	}
	for _, s := range stmts {
		if _, err := tx.Exec(ctx, s.sql, s.args...); err != nil {
			return fmt.Errorf("org B resource (%s): %w", strings.Fields(s.sql)[2], err)
		}
	}
	return nil
}

// ---- HTTP helpers --------------------------------------------------------------

func (e *isoEnv) login(t *testing.T, email string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, demoseed.DemoPassword)
	rec := e.serve("", http.MethodPost, "/api/v1/auth/login", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body.String())
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.AccessToken == "" {
		t.Fatalf("login %s: no access token in %s", email, rec.Body.String())
	}
	return out.AccessToken
}

// serve runs one request in-process. SSE and other long-lived handlers are
// bounded by a short context.
func (e *isoEnv) serve(token, method, path, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", e.ip)
	req.RemoteAddr = e.ip + ":40000"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func (e *isoEnv) as(principal, method, path, body string) *httptest.ResponseRecorder {
	return e.serve(e.tokens[principal], method, path, body)
}

var paramRE = regexp.MustCompile(`\{([a-zA-Z]+)\}`)

// fill substitutes route parameters: {orgId} → org, every other parameter →
// the org-B resource of that kind. ok=false names a parameter with no org-B
// resource, which the caller turns into a failure.
func (e *isoEnv) fill(pattern, org string) (path string, missing string) {
	path = paramRE.ReplaceAllStringFunc(pattern, func(m string) string {
		name := m[1 : len(m)-1]
		switch {
		case name == "orgId":
			return org
		case name == "id" && strings.Contains(pattern, "/invitations/"):
			return e.bIDs["invitation"]
		case name == "id" && strings.Contains(pattern, "/notifications/"):
			return e.bIDs["notification"]
		case name == "id" && strings.Contains(pattern, "/api-keys/"):
			return e.bIDs["apikey"]
		}
		if v, ok := e.bParams[name]; ok {
			return v
		}
		missing = name
		return m
	})
	return path, missing
}

func hasResourceParam(pattern string) bool {
	for _, m := range paramRE.FindAllStringSubmatch(pattern, -1) {
		if m[1] != "orgId" {
			return true
		}
	}
	return false
}

func bodyFor(method string) string {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return "{}"
	}
	return ""
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

// ---- subtests --------------------------------------------------------------

// The negative checks only mean something if the same tokens and IDs work
// where they should: A's member reads A, B's owner reads B's seeded IDs.
func (e *isoEnv) testPositiveControls(t *testing.T) {
	checks := []struct{ who, path string }{
		{"member@demo.dev", "/api/v1/orgs/" + e.orgA + "/projects"},
		{"admin@demo.dev", "/api/v1/orgs/" + e.orgA + "/tasks/" + e.aWebTask},
		{"outsider@demo.dev", "/api/v1/orgs"},
		{"beta.owner@demo.dev", "/api/v1/orgs/" + e.orgB + "/tasks/" + e.bIDs["task"]},
		{"beta.owner@demo.dev", "/api/v1/orgs/" + e.orgB + "/tasks/" + e.bIDs["task"] + "/comments"},
	}
	for _, c := range checks {
		if rec := e.as(c.who, http.MethodGet, c.path, ""); rec.Code != http.StatusOK {
			t.Errorf("%s GET %s = %d %s, want 200", c.who, c.path, rec.Code, snippet(rec.Body.Bytes()))
		}
	}
}

// Through org B's own path the tenant guard must stop every non-member before
// any handler runs.
func (e *isoEnv) testGuardMatrix(t *testing.T) {
	var failures []string
	for _, r := range e.routes {
		path, missing := e.fill(r.pattern, e.orgB)
		if missing != "" {
			t.Fatalf("%s %s: no org-B resource for {%s}; add one to insertOrgBResources", r.method, r.pattern, missing)
		}
		for _, who := range attackers {
			rec := e.as(who, r.method, path, bodyFor(r.method))
			if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
				failures = append(failures, fmt.Sprintf("%s %s %s → %d %s", who, r.method, r.pattern, rec.Code, snippet(rec.Body.Bytes())))
			}
		}
	}
	t.Logf("%d org routes × %d principals probed through org B's path", len(e.routes), len(attackers))
	report(t, "org-B path must answer 403/404", failures)
}

// Through org A's path (the guard lets A's members in) every org-B resource ID
// must be treated as absent: no success, no crash, no leaked marker.
func (e *isoEnv) testDataMatrix(t *testing.T) {
	var failures []string
	probed := 0
	for _, r := range e.routes {
		if !hasResourceParam(r.pattern) {
			continue
		}
		path, missing := e.fill(r.pattern, e.orgA)
		if missing != "" {
			t.Fatalf("%s %s: no org-B resource for {%s}; add one to insertOrgBResources", r.method, r.pattern, missing)
		}
		probed++
		for _, who := range attackers[:2] { // the outsider is not in A either; the guard matrix covers it
			rec := e.as(who, r.method, path, bodyFor(r.method))
			if bad := badCrossTenant(rec); bad != "" {
				failures = append(failures, fmt.Sprintf("%s %s %s → %d (%s) %s", who, r.method, r.pattern, rec.Code, bad, snippet(rec.Body.Bytes())))
			}
		}
	}
	if probed < 40 {
		t.Fatalf("only %d routes carry resource params; introspection looks broken", probed)
	}
	t.Logf("%d routes with resource params × 2 principals probed with org-B IDs", probed)
	report(t, "org-A path with org-B IDs must fail closed", failures)
}

// badCrossTenant classifies a response to a cross-tenant request: success,
// redirects, server errors, auth failures (which would mean the probe never
// reached the handler) and any leaked org-B marker are all failures.
func badCrossTenant(rec *httptest.ResponseRecorder) string {
	switch {
	case strings.Contains(rec.Body.String(), orgBMarker):
		return "leaks org B data"
	case rec.Code < 400:
		return "succeeded"
	case rec.Code == http.StatusUnauthorized:
		return "unauthenticated probe"
	case rec.Code >= 500:
		return "server error"
	}
	return ""
}

// Resource IDs that travel in bodies or query strings bypass the path matrix.
func (e *isoEnv) testBodyReferences(t *testing.T) {
	base := "/api/v1/orgs/" + e.orgA
	aDone := demoseed.ID("column", demoseed.ID("project", e.orgA, "WEB"), "Done")
	probes := []struct{ method, path, body string }{
		{http.MethodPost, base + "/tasks/bulk", fmt.Sprintf(`{"action":"move","task_ids":[%q],"column_id":%q}`, e.bIDs["task"], aDone)},
		{http.MethodPost, base + "/tasks/bulk", fmt.Sprintf(`{"action":"assign","task_ids":[%q],"assignee_id":%q}`, e.bIDs["task"], demoseed.ID("user", "member"))},
		{http.MethodPost, base + "/tasks/bulk", fmt.Sprintf(`{"action":"label","task_ids":[%q],"label_id":%q}`, e.bIDs["task"], e.bIDs["label"])},
		{http.MethodPost, base + "/ai/risks/scan", fmt.Sprintf(`{"project_id":%q}`, e.bIDs["project"])},
		{http.MethodPost, base + "/ai/risks/dismiss", fmt.Sprintf(`{"task_id":%q}`, e.bIDs["task"])},
		{http.MethodGet, base + "/ai/risks?project_id=" + e.bIDs["project"], ""},
	}
	var failures []string
	for _, p := range probes {
		for _, who := range attackers[:2] {
			rec := e.as(who, p.method, p.path, p.body)
			if bad := badCrossTenant(rec); bad != "" {
				failures = append(failures, fmt.Sprintf("%s %s %s %s → %d (%s) %s", who, p.method, p.path, p.body, rec.Code, bad, snippet(rec.Body.Bytes())))
			}
		}
	}
	// A search filtered to a foreign project may answer 200, but empty.
	for _, who := range attackers[:2] {
		rec := e.as(who, http.MethodGet, base+"/tasks/search?project_id="+e.bIDs["project"], "")
		if rec.Code >= 500 || strings.Contains(rec.Body.String(), orgBMarker) || searchTotal(rec) > 0 {
			failures = append(failures, fmt.Sprintf("%s search?project_id=<B> → %d %s", who, rec.Code, snippet(rec.Body.Bytes())))
		}
	}
	report(t, "org-B IDs in bodies/queries must fail closed", failures)
}

// ---- data-level checks ------------------------------------------------------

type tableDigest struct {
	rows int64
	md5  string
}

// snapshotOrg digests every row an org owns in every table with an org_id
// column (owner pool, so RLS does not hide anything).
func (e *isoEnv) snapshotOrg(t *testing.T, orgID string) map[string]tableDigest {
	t.Helper()
	ctx := context.Background()
	rows, err := e.owner.Query(ctx, `
SELECT c.table_name FROM information_schema.columns c
JOIN information_schema.tables t USING (table_schema, table_name)
WHERE c.table_schema = 'public' AND c.column_name = 'org_id' AND t.table_type = 'BASE TABLE'
ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]tableDigest{}
	for _, tbl := range tables {
		var d tableDigest
		q := fmt.Sprintf(`SELECT count(*), md5(coalesce(string_agg(x::text, '|' ORDER BY x::text), '')) FROM %s x WHERE org_id = $1`,
			pgx.Identifier{tbl}.Sanitize())
		if err := e.owner.QueryRow(ctx, q, orgID).Scan(&d.rows, &d.md5); err != nil {
			t.Fatalf("snapshot %s: %v", tbl, err)
		}
		out[tbl] = d
	}
	return out
}

func compareSnapshots(t *testing.T, before, after map[string]tableDigest) {
	var changed []string
	for tbl, b := range before {
		if a := after[tbl]; a != b {
			changed = append(changed, fmt.Sprintf("%s (%d→%d rows)", tbl, b.rows, a.rows))
		}
	}
	sort.Strings(changed)
	report(t, "org B rows changed by cross-tenant requests", changed)
}

// Without app.current_tenant the app role must see nothing; with tenant A it
// must see none of org B's rows, while tenant B sees its own (proving the
// fixture is there and the policy is what filters).
func (e *isoEnv) testRawRLS(t *testing.T) {
	ctx := context.Background()
	rows, err := e.owner.Query(ctx, `
SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public' AND c.relkind = 'r' AND c.relrowsecurity ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 20 {
		t.Fatalf("only %d RLS tables found", len(tables))
	}
	withOrg := map[string]bool{}
	snap := e.snapshotOrg(t, e.orgB)
	for tbl := range snap {
		withOrg[tbl] = true
	}

	var failures []string
	populated := 0
	for _, tbl := range tables {
		id := pgx.Identifier{tbl}.Sanitize()
		var total int64
		if err := e.owner.QueryRow(ctx, "SELECT count(*) FROM "+id).Scan(&total); err != nil {
			t.Fatal(err)
		}
		if total > 0 {
			populated++
		}
		// Both outcomes are fail-closed: on a fresh connection the unset GUC is
		// NULL and the policy matches nothing; on a pooled connection a previous
		// transaction-local set_config leaves '' behind and the policy's ::uuid
		// cast errors (22P02). Anything else is a failure.
		var unscoped int64
		if err := e.app.QueryRow(ctx, "SELECT count(*) FROM "+id).Scan(&unscoped); err != nil {
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "22P02" {
				failures = append(failures, fmt.Sprintf("%s: unexpected error without a tenant: %v", tbl, err))
			}
		} else if unscoped != 0 {
			failures = append(failures, fmt.Sprintf("%s: %d rows visible without app.current_tenant (of %d)", tbl, unscoped, total))
		}
		if !withOrg[tbl] {
			continue
		}
		foreign, own := e.countAs(t, e.orgA, tbl, e.orgB), e.countAs(t, e.orgB, tbl, e.orgB)
		if foreign != 0 {
			failures = append(failures, fmt.Sprintf("%s: tenant A sees %d of org B's rows", tbl, foreign))
		}
		if own != snap[tbl].rows {
			failures = append(failures, fmt.Sprintf("%s: tenant B sees %d of its %d rows", tbl, own, snap[tbl].rows))
		}
	}
	if populated < 10 {
		t.Fatalf("only %d RLS tables hold rows; the fixture did not load", populated)
	}
	report(t, "RLS must fail closed", failures)
}

// countAs counts the rows of org target in tbl as the app role scoped to tenant.
func (e *isoEnv) countAs(t *testing.T, tenant, tbl, target string) int64 {
	t.Helper()
	var n int64
	err := pgx.BeginFunc(context.Background(), e.app, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), "SELECT set_config('app.current_tenant', $1, true)", tenant); err != nil {
			return err
		}
		return tx.QueryRow(context.Background(),
			"SELECT count(*) FROM "+pgx.Identifier{tbl}.Sanitize()+" WHERE org_id = $1", target).Scan(&n)
	})
	if err != nil {
		t.Fatalf("count %s as tenant: %v", tbl, err)
	}
	return n
}

func (e *isoEnv) testWithCheck(t *testing.T) {
	inserts := map[string]string{
		"labels":   `INSERT INTO labels (id, org_id, name) VALUES (gen_random_uuid(), $1, 'iso-cross-tenant')`,
		"projects": `INSERT INTO projects (id, org_id, key, name, created_by) VALUES (gen_random_uuid(), $1, 'ISOX', 'cross', '` + demoseed.ID("user", "member") + `')`,
	}
	for tbl, sql := range inserts {
		err := pgx.BeginFunc(context.Background(), e.app, func(tx pgx.Tx) error {
			if _, err := tx.Exec(context.Background(), "SELECT set_config('app.current_tenant', $1, true)", e.orgA); err != nil {
				return err
			}
			_, err := tx.Exec(context.Background(), sql, e.orgB)
			return err
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s: INSERT with org B's org_id under tenant A: err = %v, want RLS violation (42501)", tbl, err)
		}
	}
}

func searchTotal(rec *httptest.ResponseRecorder) int {
	var out struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Total
}

func (e *isoEnv) testSearch(t *testing.T) {
	cases := []struct {
		who, org, q string
		wantHits    bool
	}{
		{"beta.owner@demo.dev", e.orgB, "betaco", true}, // control: the marker is searchable
		{"user01@demo.dev", e.orgA, "secaudit", true},   // control: SEC lead finds the private marker
		{"member@demo.dev", e.orgA, "betaco", false},    // other org
		{"admin@demo.dev", e.orgA, "betaco", false},     // other org, even for an admin
		{"member@demo.dev", e.orgA, "secaudit", false},  // private project the member is not in
	}
	for _, c := range cases {
		rec := e.as(c.who, http.MethodGet, "/api/v1/orgs/"+c.org+"/tasks/search?q="+c.q, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s search %q: %d %s", c.who, c.q, rec.Code, snippet(rec.Body.Bytes()))
			continue
		}
		if got := searchTotal(rec) > 0; got != c.wantHits {
			t.Errorf("%s search %q in %s: hits=%v, want %v", c.who, c.q, c.org, got, c.wantHits)
		}
	}
}

// FR-TEN-010: an org GUEST sees only projects it is a member of. guest@ is
// VIEWER on acme WEB; API is org-visible and must stay hidden from it, while
// member@ (org MEMBER, in neither project) sees API as the control.
func (e *isoEnv) testGuestVisibility(t *testing.T) {
	org := "/api/v1/orgs/" + e.orgA
	web, api := demoseed.ID("project", e.orgA, "WEB"), demoseed.ID("project", e.orgA, "API")

	listed := func(who string) []string {
		rec := e.as(who, http.MethodGet, org+"/projects", "")
		var out struct {
			Projects []struct {
				ID string `json:"id"`
			} `json:"projects"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("%s list projects: %d %s", who, rec.Code, snippet(rec.Body.Bytes()))
		}
		ids := make([]string, 0, len(out.Projects))
		for _, p := range out.Projects {
			ids = append(ids, p.ID)
		}
		return ids
	}
	if got := listed("guest@demo.dev"); len(got) != 1 || got[0] != web {
		t.Errorf("guest lists %v, want only WEB %s", got, web)
	}
	if got := listed("member@demo.dev"); !contains(got, api) {
		t.Errorf("control: member lists %v, want it to include API %s", got, api)
	}

	// Search: the guest's hits are all in WEB, and filtering to API finds
	// nothing for the guest while the member finds API tasks.
	rec := e.as("guest@demo.dev", http.MethodGet, org+"/tasks/search?q=Fix&limit=100", "")
	var hits struct {
		Tasks []struct {
			ProjectID string `json:"project_id"`
		} `json:"tasks"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &hits) != nil || len(hits.Tasks) == 0 {
		t.Fatalf("guest search: %d %s, want WEB hits", rec.Code, snippet(rec.Body.Bytes()))
	}
	for _, h := range hits.Tasks {
		if h.ProjectID != web {
			t.Errorf("guest search returned a task of project %s, want only WEB", h.ProjectID)
		}
	}
	apiSearch := org + "/tasks/search?q=Fix&project_id=" + api
	if rec := e.as("member@demo.dev", http.MethodGet, apiSearch, ""); rec.Code != http.StatusOK || searchTotal(rec) == 0 {
		t.Errorf("control: member search in API = %d %s, want hits", rec.Code, snippet(rec.Body.Bytes()))
	}
	if rec := e.as("guest@demo.dev", http.MethodGet, apiSearch, ""); rec.Code != http.StatusOK || searchTotal(rec) != 0 {
		t.Errorf("guest search in API = %d %s, want 0 hits", rec.Code, snippet(rec.Body.Bytes()))
	}
	// Facet-only listing (no text query) takes the same visibility filter.
	apiList := org + "/tasks/search?project_id=" + api
	if rec := e.as("member@demo.dev", http.MethodGet, apiList, ""); rec.Code != http.StatusOK || searchTotal(rec) == 0 {
		t.Errorf("control: member facet-only list in API = %d %s, want hits", rec.Code, snippet(rec.Body.Bytes()))
	}
	if rec := e.as("guest@demo.dev", http.MethodGet, apiList, ""); rec.Code != http.StatusOK || searchTotal(rec) != 0 {
		t.Errorf("guest facet-only list in API = %d %s, want 0 hits", rec.Code, snippet(rec.Body.Bytes()))
	}

	if rec := e.as("guest@demo.dev", http.MethodGet, org+"/projects/"+api, ""); rec.Code != http.StatusNotFound {
		t.Errorf("guest GET API project = %d, want 404", rec.Code)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (e *isoEnv) testAttachmentDownload(t *testing.T) {
	suffix := "/tasks/" + e.bIDs["task"] + "/attachments/" + e.bIDs["attachment"] + "/download"
	// Control: the rightful owner gets a presigned URL, so the store is wired.
	if rec := e.as("beta.owner@demo.dev", http.MethodGet, "/api/v1/orgs/"+e.orgB+suffix, ""); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "download_url") {
		t.Fatalf("owner download = %d %s, want 200 with download_url", rec.Code, snippet(rec.Body.Bytes()))
	}
	for _, who := range attackers[:2] {
		rec := e.as(who, http.MethodGet, "/api/v1/orgs/"+e.orgA+suffix, "")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "X-Amz") {
			t.Errorf("%s foreign download = %d %s, want 404 with no presigned URL", who, rec.Code, snippet(rec.Body.Bytes()))
		}
	}
}

func report(t *testing.T, what string, failures []string) {
	t.Helper()
	if len(failures) == 0 {
		return
	}
	sort.Strings(failures)
	t.Errorf("%s — %d violation(s):\n  %s", what, len(failures), strings.Join(failures, "\n  "))
}

// githubHook posts a signed (or not) GitHub delivery to an org's public webhook.
func (e *isoEnv) githubHook(orgRef, secret, delivery, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/integrations/github/"+orgRef+"/webhook", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", delivery)
	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", github.Sign(secret, []byte(body)))
	}
	req.Header.Set("X-Forwarded-For", e.ip)
	req.RemoteAddr = e.ip + ":40000"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

// testGithubWebhook drives the real stack: settings are ADMIN-only, the
// secret is returned once, a signed delivery links only same-org tasks even
// when the PR names an org-B key, replays and bad signatures change nothing,
// and one org's secret never authenticates another org's URL.
func (e *isoEnv) testGithubWebhook(t *testing.T) {
	ctx := context.Background()
	base := "/api/v1/orgs/" + e.orgA + "/integrations/github"
	if rec := e.as("member@demo.dev", http.MethodGet, base, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("member reading settings: %d, want 403", rec.Code)
	}
	rec := e.as("admin@demo.dev", http.MethodPost, base+"/secret", "")
	var sec struct {
		Secret string `json:"secret"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &sec) != nil || len(sec.Secret) != 64 {
		t.Fatalf("rotate secret: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.as("admin@demo.dev", http.MethodGet, base, ""); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), sec.Secret) {
		t.Fatalf("settings read: %d (must succeed and never echo the secret) %s", rec.Code, rec.Body.String())
	}
	foreign := fmt.Sprintf(`{"repos":[],"move_on_merge_project_ids":[%q]}`, e.bIDs["project"])
	if rec := e.as("admin@demo.dev", http.MethodPut, base, foreign); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("org-B project in move-on-merge: %d, want 422", rec.Code)
	}

	title := "WEB-1 and BETA-1 " + orgBMarker
	payload := func(n int) string {
		return fmt.Sprintf(`{"action":"opened","pull_request":{"number":%d,"title":%q,"html_url":"https://github.com/acme/app/pull/%d","state":"open","merged":false,"updated_at":"2026-05-01T10:00:00Z","head":{"ref":"x"}},"repository":{"full_name":"acme/app"}}`, n, title, n)
	}
	count := func(org, task string) int {
		var n int
		if err := e.owner.QueryRow(ctx, `SELECT count(*) FROM task_pull_requests WHERE org_id = $1 AND task_id = $2 AND repo = 'acme/app'`, org, task).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if rec := e.githubHook(e.orgA, "", "iso-1", payload(5)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned delivery: %d, want 401", rec.Code)
	}
	if rec := e.githubHook(e.orgA, orgBGithubSecret, "iso-1", payload(5)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("org B's secret on org A's URL: %d, want 401", rec.Code)
	}
	if rec := e.githubHook("no-such-org", sec.Secret, "iso-1", payload(5)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unknown org: %d, want 401", rec.Code)
	}
	if count(e.orgA, e.aWebTask) != 0 {
		t.Fatal("a rejected delivery linked a PR")
	}

	if rec := e.githubHook(e.orgA, sec.Secret, "iso-1", payload(5)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "processed") {
		t.Fatalf("signed delivery: %d %s", rec.Code, rec.Body.String())
	}
	if count(e.orgA, e.aWebTask) != 1 {
		t.Fatal("PR was not linked to the org A task named WEB-1")
	}
	if count(e.orgB, e.bIDs["task"]) != 0 {
		t.Fatal("PR leaked onto org B's BETA-1 task")
	}
	if rec := e.githubHook(e.orgA, sec.Secret, "iso-1", payload(6)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "duplicate") {
		t.Fatalf("replay: %d %s", rec.Code, rec.Body.String())
	}
	var replayed int
	if err := e.owner.QueryRow(ctx, `SELECT count(*) FROM task_pull_requests WHERE org_id = $1 AND number = 6`, e.orgA).Scan(&replayed); err != nil || replayed != 0 {
		t.Fatalf("replayed delivery wrote %d rows (err %v)", replayed, err)
	}

	// The linked PR shows in task detail for org A.
	list := e.as("member@demo.dev", http.MethodGet, "/api/v1/orgs/"+e.orgA+"/tasks/"+e.aWebTask+"/pull-requests", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "acme/app") {
		t.Fatalf("task PR list: %d %s", list.Code, list.Body.String())
	}
}
