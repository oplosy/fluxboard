//go:build integration

// Integration test for the object-store adapter against a real S3-compatible
// server (RustFS in compose and CI, ADR-027). It drives the same calls the
// attachment flow makes (FR-TASK-006): presign PUT → client upload → Stat on
// confirm → presign GET → Remove on delete/GC.
//
// Run with the compose stack up:
//
//	TEST_S3_ENDPOINT=localhost:9000 TEST_S3_ACCESS_KEY=minioadmin TEST_S3_SECRET_KEY=minioadmin \
//	  go test -tags=integration ./internal/infrastructure/minio/
//
// (make test-integration wires this). Like the Postgres suite, a missing
// endpoint fails instead of skipping so CI cannot go green without storage.
package miniox

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func newIntegrationStore(t *testing.T) *Client {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Fatal("TEST_S3_ENDPOINT not set; integration tests need an S3-compatible server (see make test-integration)")
	}
	// A fresh bucket per run also exercises New's create-if-missing path.
	c, err := New(context.Background(), Config{
		Endpoint:  endpoint,
		AccessKey: os.Getenv("TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("TEST_S3_SECRET_KEY"),
		Bucket:    "fluxboard-it-" + uuid.NewString()[:8],
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestStoreAttachmentLifecycle(t *testing.T) {
	c := newIntegrationStore(t)
	ctx := context.Background()
	key := "attachments/it/" + uuid.NewString() + ".txt"
	body := "fluxboard attachment body"

	putURL, err := c.PresignPut(ctx, key, "text/plain", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "text/plain")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", res.StatusCode)
	}

	size, err := c.Stat(ctx, key)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if size != int64(len(body)) {
		t.Fatalf("Stat size = %d, want %d", size, len(body))
	}

	getURL, err := c.PresignGet(ctx, key, "report.txt", 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	gres, err := http.Get(getURL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	got, _ := io.ReadAll(gres.Body)
	gres.Body.Close()
	if gres.StatusCode != http.StatusOK || string(got) != body {
		t.Fatalf("GET = %d %q, want 200 %q", gres.StatusCode, got, body)
	}
	if cd := gres.Header.Get("Content-Disposition"); cd != `attachment; filename="report.txt"` {
		t.Fatalf("Content-Disposition = %q", cd)
	}

	if err := c.Remove(ctx, key); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := c.Stat(ctx, key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Stat after Remove: err = %v, want domain.ErrNotFound", err)
	}
	if err := c.Remove(ctx, key); err != nil {
		t.Fatalf("second Remove should be a no-op: %v", err)
	}
}

// The confirm step relies on Stat mapping a never-uploaded key to
// domain.ErrNotFound (a client that skipped the PUT must get 404, not 500).
func TestStoreStatMissingIsNotFound(t *testing.T) {
	c := newIntegrationStore(t)
	if _, err := c.Stat(context.Background(), "attachments/it/never-uploaded"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Stat missing: err = %v, want domain.ErrNotFound", err)
	}
}

// An expired presigned PUT must be rejected by the server.
func TestStorePresignedPutExpires(t *testing.T) {
	c := newIntegrationStore(t)
	ctx := context.Background()
	putURL, err := c.PresignPut(ctx, "attachments/it/expired.txt", "text/plain", time.Second)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	time.Sleep(2 * time.Second)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, putURL, strings.NewReader("late"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("expired PUT status = %d, want 403", res.StatusCode)
	}
}
