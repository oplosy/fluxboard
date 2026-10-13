package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
)

// --- harness ---------------------------------------------------------------

var testTools = []Tool{
	{
		Name: "echo", Description: "echo", InputSchema: objectSchema(map[string]any{"text": str("t")}, "text"),
		Call: func(_ context.Context, c Caller, a Args) (map[string]any, error) {
			t, err := a.RequiredStr("text")
			if err != nil {
				return nil, err
			}
			return map[string]any{"text": t, "org": c.OrgID, "user": c.UserID}, nil
		},
	},
	{
		Name: "write.thing", Description: "writes", InputSchema: objectSchema(map[string]any{}), Write: true,
		Call: func(context.Context, Caller, Args) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		},
	},
	{
		Name: "agent.thing", Description: "authors rows", InputSchema: objectSchema(map[string]any{}), Write: true, AgentOnly: true,
		Call: func(_ context.Context, c Caller, _ Args) (map[string]any, error) {
			return map[string]any{"user": c.UserID, "role": string(c.Role)}, nil
		},
	},
	{
		Name: "fails.notfound", Description: "nf", InputSchema: objectSchema(map[string]any{}),
		Call: func(context.Context, Caller, Args) (map[string]any, error) {
			return nil, fmt.Errorf("project: %w", domain.ErrNotFound)
		},
	},
	{
		Name: "fails.internal", Description: "boom", InputSchema: objectSchema(map[string]any{}),
		Call: func(context.Context, Caller, Args) (map[string]any, error) {
			return nil, errors.New("pq: connection refused 10.0.0.5")
		},
	},
	{
		Name: "naïve.tool", Description: "non-ascii name", InputSchema: objectSchema(map[string]any{}),
		Call: func(context.Context, Caller, Args) (map[string]any, error) { return map[string]any{}, nil },
	},
}

// handler runs the server behind the real key→tenant guard, authenticated as
// an API key of org "org-1" with the given scope ("" = no credential). With an
// agentID the key is that agent's (role MEMBER), as HybridAuth resolves it.
func handler(scope apikey.Scope, agentID string) http.Handler {
	srv := NewServer(Config{Version: "test", WebOrigin: "https://app.example.com"}, testTools)
	guarded := (&mw.TenantGuard{}).ResolveFromAPIKey(srv)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if scope != "" {
			info := mw.APIKeyInfo{OrgID: "org-1", KeyID: "k1", Scopes: []apikey.Scope{scope}}
			user := "apikey:k1"
			if agentID != "" {
				info.AgentID, info.AgentRole, user = agentID, tenant.RoleMember, agentID
			}
			ctx := mw.WithPrincipal(r.Context(), mw.Principal{UserID: user, EmailVerified: true})
			ctx = mw.WithAPIKey(ctx, info)
			r = r.WithContext(ctx)
		}
		guarded.ServeHTTP(w, r)
	})
}

func modernMeta() map[string]any {
	return map[string]any{
		metaProtocolVersion:                  VersionModern,
		metaClientCapabilities:               map[string]any{},
		"io.modelcontextprotocol/clientInfo": map[string]any{"name": "test", "version": "1"},
	}
}

type call struct {
	scope   apikey.Scope
	agentID string
	headers map[string]string
	body    any
	method  string
}

func do(t *testing.T, c call) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	if c.scope == "" {
		c.scope = apikey.ScopeWrite
	}
	if c.method == "" {
		c.method = http.MethodPost
	}
	var buf bytes.Buffer
	switch b := c.body.(type) {
	case string:
		buf.WriteString(b)
	case nil:
	default:
		_ = json.NewEncoder(&buf).Encode(b)
	}
	r := httptest.NewRequestWithContext(context.Background(), c.method, "/api/v1/mcp", &buf)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range c.headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	handler(c.scope, c.agentID).ServeHTTP(rec, r)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

// modern builds a well-formed 2026-07-28 request + headers.
func modern(method string, id any, params map[string]any) call {
	if params == nil {
		params = map[string]any{}
	}
	params["_meta"] = modernMeta()
	h := map[string]string{"MCP-Protocol-Version": VersionModern, "Mcp-Method": method}
	if name, ok := params["name"].(string); ok {
		h["Mcp-Name"] = name
	}
	body := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		body["id"] = id
	}
	return call{headers: h, body: body}
}

func errCode(t *testing.T, out map[string]any) int {
	t.Helper()
	e, ok := out["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected a JSON-RPC error, got %v", out)
	}
	return int(e["code"].(float64))
}

func result(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	r, ok := out["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected a result, got %v", out)
	}
	return r
}

func toolNames(r map[string]any) []string {
	var names []string
	for _, tl := range r["tools"].([]any) {
		names = append(names, tl.(map[string]any)["name"].(string))
	}
	return names
}

// --- modern (2026-07-28) ------------------------------------------------------

func TestModernDiscover(t *testing.T) {
	rec, out := do(t, modern("server/discover", 1, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	r := result(t, out)
	if r["resultType"] != "complete" || r["supportedVersions"].([]any)[0] != VersionModern {
		t.Fatalf("discover = %v", r)
	}
	info := r["_meta"].(map[string]any)[metaServerInfo].(map[string]any)
	if info["name"] != "fluxboard" || r["capabilities"].(map[string]any)["tools"] == nil {
		t.Fatalf("missing serverInfo/capabilities: %v", r)
	}
}

func TestModernToolsListFiltersByScope(t *testing.T) {
	_, out := do(t, modern("tools/list", 1, nil))
	if got := toolNames(result(t, out)); strings.Join(got, ",") != "echo,fails.internal,fails.notfound,naïve.tool,write.thing" {
		t.Fatalf("write key sees %v (want sorted, all)", got)
	}
	c := modern("tools/list", 2, nil)
	c.scope = apikey.ScopeRead
	_, out = do(t, c)
	for _, n := range toolNames(result(t, out)) {
		if n == "write.thing" {
			t.Fatal("a read-only key must not see write tools")
		}
	}
}

func TestAgentOnlyTools(t *testing.T) {
	// An org key neither sees nor may call an agent-only tool.
	call := modern("tools/call", 1, map[string]any{"name": "agent.thing"})
	_, out := do(t, call)
	r := result(t, out)
	if r["isError"] != true || !strings.Contains(r["content"].([]any)[0].(map[string]any)["text"].(string), "agent") {
		t.Fatalf("org key calling an agent-only tool: %v", r)
	}
	// An agent's key sees it and calls it as itself, with its own role.
	list := modern("tools/list", 2, nil)
	list.agentID = "agent-7"
	_, out = do(t, list)
	if !slices.Contains(toolNames(result(t, out)), "agent.thing") {
		t.Fatal("an agent's key must see agent-only tools")
	}
	call = modern("tools/call", 3, map[string]any{"name": "agent.thing"})
	call.agentID = "agent-7"
	_, out = do(t, call)
	sc := result(t, out)["structuredContent"].(map[string]any)
	if sc["user"] != "agent-7" || sc["role"] != "MEMBER" {
		t.Fatalf("agent call ran as %v, want the agent with its MEMBER role", sc)
	}
}

func TestModernToolsCallStructuredResult(t *testing.T) {
	rec, out := do(t, modern("tools/call", 7, map[string]any{"name": "echo", "arguments": map[string]any{"text": "hi"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	r := result(t, out)
	sc := r["structuredContent"].(map[string]any)
	if r["isError"] != false || sc["text"] != "hi" || sc["org"] != "org-1" || sc["user"] != "apikey:k1" {
		t.Fatalf("result = %v", r)
	}
	text := r["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"text":"hi"`) {
		t.Fatalf("text content must mirror structuredContent, got %q", text)
	}
	if out["id"] != float64(7) {
		t.Fatalf("id not echoed: %v", out["id"])
	}
}

func TestModernHeaderValidation(t *testing.T) {
	cases := map[string]func(c *call){
		"missing version header": func(c *call) { delete(c.headers, "MCP-Protocol-Version") },
		"version header differs": func(c *call) { c.headers["MCP-Protocol-Version"] = "2025-11-25" },
		"method header differs":  func(c *call) { c.headers["Mcp-Method"] = "tools/list" },
		"name header differs":    func(c *call) { c.headers["Mcp-Name"] = "write.thing" },
		"name header missing":    func(c *call) { delete(c.headers, "Mcp-Name") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := modern("tools/call", 1, map[string]any{"name": "echo", "arguments": map[string]any{"text": "x"}})
			mutate(&c)
			rec, out := do(t, c)
			if rec.Code != http.StatusBadRequest || errCode(t, out) != codeHeaderMismatch {
				t.Fatalf("want 400 HeaderMismatch, got %d %v", rec.Code, out)
			}
		})
	}
}

func TestModernMcpNameBase64Sentinel(t *testing.T) {
	c := modern("tools/call", 1, map[string]any{"name": "naïve.tool"})
	c.headers["Mcp-Name"] = "=?base64?" + base64.StdEncoding.EncodeToString([]byte("naïve.tool")) + "?="
	rec, out := do(t, c)
	if rec.Code != http.StatusOK || result(t, out)["isError"] != false {
		t.Fatalf("encoded Mcp-Name must be decoded before comparing: %d %v", rec.Code, out)
	}
}

func TestModernUnsupportedVersion(t *testing.T) {
	c := modern("tools/list", 1, nil)
	c.body.(map[string]any)["params"].(map[string]any)["_meta"].(map[string]any)[metaProtocolVersion] = "1900-01-01"
	c.headers["MCP-Protocol-Version"] = "1900-01-01"
	rec, out := do(t, c)
	if rec.Code != http.StatusBadRequest || errCode(t, out) != codeUnsupportedVersion {
		t.Fatalf("want 400 -32022, got %d %v", rec.Code, out)
	}
	data := out["error"].(map[string]any)["data"].(map[string]any)
	if data["requested"] != "1900-01-01" || len(data["supported"].([]any)) != 4 {
		t.Fatalf("data = %v", data)
	}
}

func TestModernMissingCapabilities(t *testing.T) {
	c := modern("tools/list", 1, nil)
	delete(c.body.(map[string]any)["params"].(map[string]any)["_meta"].(map[string]any), metaClientCapabilities)
	rec, out := do(t, c)
	if rec.Code != http.StatusBadRequest || errCode(t, out) != codeInvalidParams {
		t.Fatalf("want 400 -32602, got %d %v", rec.Code, out)
	}
}

func TestModernUnknownMethod(t *testing.T) {
	rec, out := do(t, modern("resources/list", 1, nil))
	if rec.Code != http.StatusNotFound || errCode(t, out) != codeMethodNotFound {
		t.Fatalf("want 404 -32601, got %d %v", rec.Code, out)
	}
}

// --- tool errors -------------------------------------------------------------

func TestToolErrors(t *testing.T) {
	t.Run("unknown tool is a protocol error", func(t *testing.T) {
		_, out := do(t, modern("tools/call", 1, map[string]any{"name": "nope"}))
		if errCode(t, out) != codeInvalidParams {
			t.Fatalf("got %v", out)
		}
	})
	t.Run("bad arguments are a tool error the model can fix", func(t *testing.T) {
		_, out := do(t, modern("tools/call", 1, map[string]any{"name": "echo", "arguments": map[string]any{"text": 5}}))
		r := result(t, out)
		if r["isError"] != true || !strings.Contains(fmt.Sprint(r["content"]), "text must be a string") {
			t.Fatalf("got %v", r)
		}
	})
	t.Run("write tool with a read key", func(t *testing.T) {
		c := modern("tools/call", 1, map[string]any{"name": "write.thing"})
		c.scope = apikey.ScopeRead
		_, out := do(t, c)
		if r := result(t, out); r["isError"] != true || !strings.Contains(fmt.Sprint(r["content"]), "read-only") {
			t.Fatalf("got %v", r)
		}
	})
	t.Run("domain not-found becomes an actionable tool error", func(t *testing.T) {
		_, out := do(t, modern("tools/call", 1, map[string]any{"name": "fails.notfound"}))
		if r := result(t, out); r["isError"] != true {
			t.Fatalf("got %v", r)
		}
	})
	t.Run("internal failure leaks nothing", func(t *testing.T) {
		rec, out := do(t, modern("tools/call", 1, map[string]any{"name": "fails.internal"}))
		if errCode(t, out) != codeInternalError || strings.Contains(rec.Body.String(), "10.0.0.5") {
			t.Fatalf("got %d %s", rec.Code, rec.Body)
		}
	})
}

// --- legacy (2025-x) ------------------------------------------------------------

func legacy(method string, id any, params map[string]any, version string) call {
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		body["id"] = id
	}
	if params != nil {
		body["params"] = params
	}
	h := map[string]string{}
	if version != "" {
		h["MCP-Protocol-Version"] = version
	}
	return call{headers: h, body: body}
}

func TestLegacyInitializeNegotiates(t *testing.T) {
	for requested, want := range map[string]string{
		"2025-06-18": "2025-06-18",
		"2025-11-25": "2025-11-25",
		"2024-11-05": "2025-11-25", // unknown → our newest legacy
	} {
		_, out := do(t, legacy("initialize", 1, map[string]any{
			"protocolVersion": requested, "capabilities": map[string]any{},
			"clientInfo": map[string]any{"name": "claude-code", "version": "2"},
		}, ""))
		r := result(t, out)
		if r["protocolVersion"] != want || r["serverInfo"].(map[string]any)["name"] != "fluxboard" {
			t.Fatalf("initialize(%s) = %v", requested, r)
		}
		if _, has := r["resultType"]; has {
			t.Fatal("legacy results must not carry modern-only fields")
		}
	}
}

func TestLegacyFlow(t *testing.T) {
	rec, _ := do(t, legacy("notifications/initialized", nil, nil, "2025-11-25"))
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("notification: %d %q", rec.Code, rec.Body)
	}
	_, out := do(t, legacy("tools/list", 2, nil, "2025-06-18"))
	if len(toolNames(result(t, out))) != len(testTools)-1 { // all but the agent-only tool
		t.Fatalf("tools/list = %v", out)
	}
	_, out = do(t, legacy("tools/call", 3, map[string]any{"name": "echo", "arguments": map[string]any{"text": "a"}}, ""))
	if result(t, out)["structuredContent"].(map[string]any)["text"] != "a" {
		t.Fatalf("headerless (2025-03-26) call failed: %v", out)
	}
	_, out = do(t, legacy("ping", 4, nil, "2025-11-25"))
	if _, ok := out["result"]; !ok {
		t.Fatalf("ping = %v", out)
	}
}

func TestLegacyRejectsUnknownVersionHeader(t *testing.T) {
	rec, _ := do(t, legacy("tools/list", 1, nil, "2099-01-01"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	// Modern header without the required _meta is a malformed modern request.
	rec, out := do(t, legacy("tools/list", 1, nil, VersionModern))
	if rec.Code != http.StatusBadRequest || errCode(t, out) != codeInvalidParams {
		t.Fatalf("want 400 -32602, got %d %v", rec.Code, out)
	}
}

// --- transport ------------------------------------------------------------------

func TestTransportGuards(t *testing.T) {
	t.Run("GET and DELETE are 405", func(t *testing.T) {
		for _, m := range []string{http.MethodGet, http.MethodDelete} {
			rec, _ := do(t, call{method: m})
			if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
				t.Fatalf("%s → %d", m, rec.Code)
			}
		}
	})
	t.Run("foreign Origin is 403", func(t *testing.T) {
		c := modern("tools/list", 1, nil)
		c.headers["Origin"] = "https://evil.example"
		if rec, _ := do(t, c); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d", rec.Code)
		}
		c.headers["Origin"] = "https://app.example.com"
		if rec, _ := do(t, c); rec.Code != http.StatusOK {
			t.Fatalf("own origin got %d", rec.Code)
		}
	})
	t.Run("no API key is 401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/mcp", strings.NewReader(`{}`))
		handler("", "").ServeHTTP(rec, r)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", rec.Code)
		}
	})
	t.Run("batches, bad JSON and null ids are rejected", func(t *testing.T) {
		for body, want := range map[string]int{
			`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`: codeInvalidRequest,
			`{not json`: codeParseError,
			`{"jsonrpc":"2.0","id":null,"method":"ping"}`: codeInvalidRequest,
			`{"jsonrpc":"1.0","id":1,"method":"ping"}`:    codeInvalidRequest,
		} {
			rec, out := do(t, call{body: body})
			if rec.Code != http.StatusBadRequest || errCode(t, out) != want {
				t.Fatalf("%s → %d %v", body, rec.Code, out)
			}
		}
	})
}
