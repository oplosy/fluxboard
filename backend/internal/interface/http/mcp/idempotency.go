package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Idempotent replay for write tools (ADR-033 decision 4, FR-AGT-005): a write
// tool that is not idempotent by nature accepts an idempotency key (argument
// idempotency_key, or the Idempotency-Key header). The first successful result
// is kept for 24 h; a retry with the same key and arguments gets that result
// back instead of running the write again. Tools that are idempotent by
// nature (Idempotent) or handle keys themselves are left alone.

// IdempotencyStore is a small key/value store with expiry (Redis in cmd/api).
type IdempotencyStore interface {
	Get(ctx context.Context, key string) (string, error) // "" when absent
	SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error)
	Set(ctx context.Context, key, val string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

const (
	idemArg     = "idempotency_key"
	idemTTL     = 24 * time.Hour
	idemPending = time.Minute // a crashed run frees its key after this
	idemMaxLen  = 200
)

// idemRecord is what the store holds: the hash of the arguments the key was
// first used with, and the result once the write succeeded.
type idemRecord struct {
	ArgsHash string         `json:"h"`
	Pending  bool           `json:"p,omitempty"`
	Result   map[string]any `json:"r,omitempty"`
}

// replayable reports whether the server adds key-based replay to t.
func (t *Tool) replayable() bool { return t.Write && !t.Idempotent }

// withIdempotencyArg adds the optional idempotency_key property to a
// replayable tool's input schema, so clients that validate arguments accept it.
func withIdempotencyArg(t Tool) Tool {
	if !t.replayable() {
		return t
	}
	props, _ := t.InputSchema["properties"].(map[string]any)
	if _, ok := props[idemArg]; ok {
		return t
	}
	schema := make(map[string]any, len(t.InputSchema))
	for k, v := range t.InputSchema {
		schema[k] = v
	}
	np := make(map[string]any, len(props)+1)
	for k, v := range props {
		np[k] = v
	}
	np[idemArg] = str("Optional key (max 200 chars). Retrying with the same key and arguments " +
		"within 24 hours returns the first result instead of writing again.")
	schema["properties"] = np
	t.InputSchema = schema
	return t
}

// idemKey returns the store key for a call, or "" when the call carries no
// idempotency key. Keys are scoped to the org, the principal and the tool.
func idemKey(c Caller, tool string, args Args) (string, *ArgError) {
	key, _ := args[idemArg].(string)
	if v, present := args[idemArg]; present && v != nil {
		if _, ok := v.(string); !ok {
			return "", &ArgError{Msg: idemArg + " must be a string"}
		}
	}
	if key == "" {
		key = c.IdempotencyKey
	}
	if key == "" {
		return "", nil
	}
	if len(key) > idemMaxLen {
		return "", &ArgError{Msg: idemArg + " is too long"}
	}
	sum := sha256.Sum256([]byte(key))
	return "mcp:idem:" + c.OrgID + ":" + c.UserID + ":" + tool + ":" + hex.EncodeToString(sum[:]), nil
}

// argsHash fingerprints the arguments a key was used with (key excluded;
// encoding/json sorts map keys, so equal arguments hash equally).
func argsHash(args Args) string {
	rest := make(map[string]any, len(args))
	for k, v := range args {
		if k != idemArg {
			rest[k] = v
		}
	}
	b, _ := json.Marshal(rest)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
