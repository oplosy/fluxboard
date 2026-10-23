package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	h := CORS("http://localhost:3000", "http://localhost:3002")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	call := func(method, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), method, "/api/v1/x", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// A preflight from an extra origin is allowed and cacheable.
	rec := call(http.MethodOptions, "http://localhost:3002")
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3002" {
		t.Fatalf("preflight: %d %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Access-Control-Max-Age") != "7200" {
		t.Errorf("Max-Age = %q, want 7200", rec.Header().Get("Access-Control-Max-Age"))
	}
	// An unknown origin gets no CORS grant.
	if rec := call(http.MethodGet, "https://evil.example"); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("unknown origin granted: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
