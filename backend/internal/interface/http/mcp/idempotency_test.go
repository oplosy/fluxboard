package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
)

type memIdem struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memIdem) Get(_ context.Context, k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], nil
}
func (s *memIdem) SetNX(_ context.Context, k, v string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[k]; ok {
		return false, nil
	}
	s.m[k] = v
	return true, nil
}
func (s *memIdem) Set(_ context.Context, k, v string, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
func (s *memIdem) Del(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}

// idemServer serves one replayable write tool that counts its runs and fails
// while fail is set, as an agent with a write key.
func idemServer(runs *int, fail *bool) (http.Handler, *memIdem) {
	store := &memIdem{m: map[string]string{}}
	tool := Tool{
		Name: "notes.add", Description: "adds a note", Write: true,
		InputSchema: objectSchema(map[string]any{"text": str("t")}, "text"),
		Call: func(_ context.Context, _ Caller, a Args) (map[string]any, error) {
			if *fail {
				return nil, domain.ErrConflict
			}
			*runs++
			return map[string]any{"run": *runs, "text": a["text"]}, nil
		},
	}
	srv := NewServer(Config{Version: "test", Idempotency: store}, []Tool{tool})
	guarded := (&mw.TenantGuard{}).ResolveFromAPIKey(srv)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := mw.WithPrincipal(r.Context(), mw.Principal{UserID: "agent-1", EmailVerified: true})
		ctx = mw.WithAPIKey(ctx, mw.APIKeyInfo{OrgID: "org-1", KeyID: "k1", Scopes: []apikey.Scope{apikey.ScopeWrite},
			AgentID: "agent-1", AgentRole: tenant.RoleMember})
		guarded.ServeHTTP(w, r.WithContext(ctx))
	}), store
}

func idemCall(t *testing.T, h http.Handler, args map[string]any, header string) map[string]any {
	t.Helper()
	c := modern("tools/call", 1, map[string]any{"name": "notes.add", "arguments": args})
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(c.body)
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/mcp", &buf)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.headers {
		r.Header.Set(k, v)
	}
	if header != "" {
		r.Header.Set("Idempotency-Key", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body, err)
	}
	return result(t, out)
}

func runOf(res map[string]any) float64 {
	sc, _ := res["structuredContent"].(map[string]any)
	n, _ := sc["run"].(float64)
	return n
}

func TestIdempotentReplay(t *testing.T) {
	runs, fail := 0, false
	h, _ := idemServer(&runs, &fail)

	first := idemCall(t, h, map[string]any{"text": "a", "idempotency_key": "k1"}, "")
	again := idemCall(t, h, map[string]any{"text": "a", "idempotency_key": "k1"}, "")
	if runs != 1 || runOf(first) != 1 || runOf(again) != 1 {
		t.Fatalf("replay ran the write again: runs=%d first=%v again=%v", runs, first, again)
	}
	// Same key, other arguments: refused, nothing written.
	other := idemCall(t, h, map[string]any{"text": "b", "idempotency_key": "k1"}, "")
	if other["isError"] != true || !strings.Contains(other["content"].([]any)[0].(map[string]any)["text"].(string), "different arguments") {
		t.Fatalf("reused key: %v", other)
	}
	// The header works like the argument; no key means no replay.
	idemCall(t, h, map[string]any{"text": "c"}, "hdr")
	idemCall(t, h, map[string]any{"text": "c"}, "hdr")
	idemCall(t, h, map[string]any{"text": "d"}, "")
	idemCall(t, h, map[string]any{"text": "d"}, "")
	if runs != 4 {
		t.Fatalf("runs = %d, want 4 (k1, hdr, and two keyless calls)", runs)
	}
}

func TestIdempotentFailureFreesKey(t *testing.T) {
	runs, fail := 0, true
	h, store := idemServer(&runs, &fail)
	res := idemCall(t, h, map[string]any{"text": "a", "idempotency_key": "k"}, "")
	if res["isError"] != true || len(store.m) != 0 {
		t.Fatalf("a failed call must not keep its key: %v %v", res, store.m)
	}
	fail = false
	if res := idemCall(t, h, map[string]any{"text": "a", "idempotency_key": "k"}, ""); runOf(res) != 1 {
		t.Fatalf("retry after failure should run: %v", res)
	}
}

func TestIdempotentPendingKey(t *testing.T) {
	runs, fail := 0, false
	h, store := idemServer(&runs, &fail)
	key, _ := idemKey(Caller{OrgID: "org-1", UserID: "agent-1"}, "notes.add", Args{"idempotency_key": "busy"})
	rec, _ := json.Marshal(idemRecord{ArgsHash: argsHash(Args{"text": "a"}), Pending: true})
	store.m[key] = string(rec)
	res := idemCall(t, h, map[string]any{"text": "a", "idempotency_key": "busy"}, "")
	if res["isError"] != true || runs != 0 {
		t.Fatalf("an in-flight key must not run twice: %v runs=%d", res, runs)
	}
}

func TestIdempotencyArgInSchema(t *testing.T) {
	srv := NewServer(Config{Idempotency: &memIdem{m: map[string]string{}}}, testTools)
	props := func(name string) map[string]any {
		return srv.byName[name].InputSchema["properties"].(map[string]any)
	}
	if _, ok := props("write.thing")[idemArg]; !ok {
		t.Fatal("a replayable write tool must advertise idempotency_key")
	}
	if _, ok := props("echo")[idemArg]; ok {
		t.Fatal("read tools take no idempotency_key")
	}
	// The shared testTools schema itself is not mutated.
	for _, tl := range testTools {
		if _, ok := tl.InputSchema["properties"].(map[string]any)[idemArg]; ok {
			t.Fatal("withIdempotencyArg mutated the caller's schema")
		}
	}
	if _, err := idemKey(Caller{}, "x", Args{idemArg: 5}); err == nil {
		t.Fatal("a non-string key must be an argument error")
	}
	if _, err := idemKey(Caller{}, "x", Args{idemArg: strings.Repeat("k", 201)}); err == nil {
		t.Fatal("an over-long key must be an argument error")
	}
}
