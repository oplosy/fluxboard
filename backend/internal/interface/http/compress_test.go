package httpx

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompressJSON(t *testing.T) {
	body := `{"tasks":[` + strings.Repeat(`{"title":"a task"},`, 200) + `{}]}`
	serve := func(contentType string) *httptest.ResponseRecorder {
		h := compressJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", contentType)
			_, _ = io.WriteString(w, body)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rec := serve("application/json")
	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("JSON: Content-Encoding = %q, want gzip", rec.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(zr)
	if string(got) != body {
		t.Fatalf("JSON body did not round-trip")
	}

	// The SSE stream must reach the client unbuffered and uncompressed.
	if rec := serve("text/event-stream"); rec.Header().Get("Content-Encoding") != "" || rec.Body.String() != body {
		t.Fatalf("SSE: Content-Encoding = %q, want none", rec.Header().Get("Content-Encoding"))
	}
}
