package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestReady(t *testing.T) {
	up := pingerFunc(func(context.Context) error { return nil })
	behind := pingerFunc(func(context.Context) error { return errors.New("schema at 31, binary needs 32") })

	cases := []struct {
		name   string
		h      Health
		status int
		check  string // the migrations check result
	}{
		{"all up", Health{DB: up, Redis: up, Migrations: up}, http.StatusOK, "ok"},
		{"schema behind", Health{DB: up, Redis: up, Migrations: behind}, http.StatusServiceUnavailable, "down: schema at 31, binary needs 32"},
		{"migrations check not wired", Health{DB: up, Redis: up}, http.StatusServiceUnavailable, "unconfigured"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		c.h.Ready(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil))
		var body struct {
			Ready  bool              `json:"ready"`
			Checks map[string]string `json:"checks"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: decode: %v", c.name, err)
		}
		if rec.Code != c.status || body.Ready != (c.status == http.StatusOK) || body.Checks["migrations"] != c.check {
			t.Errorf("%s: %d %+v, want %d with migrations=%q", c.name, rec.Code, body, c.status, c.check)
		}
	}
}
