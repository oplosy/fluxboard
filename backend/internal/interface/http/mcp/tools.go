package mcp

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Caller is who invokes a tool: the org-bound API key resolved by the
// middleware chain, with the role the tenant guard mapped its scope to.
type Caller struct {
	OrgID    string
	UserID   string // principal id ("apikey:<id>")
	Role     tenant.OrgRole
	CanWrite bool
	// IdempotencyKey is the request's Idempotency-Key header, for tools that
	// accept one when the argument is omitted.
	IdempotencyKey string
}

// Tool is one callable MCP tool. Write tools are hidden from, and refused to,
// read-only keys.
type Tool struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any
	Write       bool
	Destructive bool
	Idempotent  bool
	Call        func(ctx context.Context, c Caller, args Args) (map[string]any, error)
}

func (t *Tool) descriptor() toolDescriptor {
	return toolDescriptor{
		Name:        t.Name,
		Title:       t.Title,
		Description: t.Description,
		InputSchema: t.InputSchema,
		Annotations: &toolAnnotations{
			Title:           t.Title,
			ReadOnlyHint:    !t.Write,
			DestructiveHint: t.Destructive,
			IdempotentHint:  !t.Write || t.Idempotent,
			OpenWorldHint:   false,
		},
	}
}

// ArgError is a problem with a tool's arguments. It is reported to the model as
// a tool execution error (isError) so it can correct the call and retry.
type ArgError struct{ Msg string }

func (e *ArgError) Error() string { return e.Msg }

func argErrorf(format string, a ...any) error { return &ArgError{Msg: fmt.Sprintf(format, a...)} }

// Args are a tool call's decoded arguments with typed accessors that reject
// wrong types instead of silently defaulting.
type Args map[string]any

// Str returns an optional string argument ("" when absent).
func (a Args) Str(key string) (string, error) {
	v, ok := a[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", argErrorf("%s must be a string", key)
	}
	return strings.TrimSpace(s), nil
}

// RequiredStr returns a non-empty string argument.
func (a Args) RequiredStr(key string) (string, error) {
	s, err := a.Str(key)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", argErrorf("%s is required", key)
	}
	return s, nil
}

// ID returns an optional id argument ("" when absent). Ids are UUIDs; anything
// else is an argument error rather than a failed lookup, so a model that passes
// a project key or a task number learns which value to use instead.
func (a Args) ID(key string) (string, error) {
	s, err := a.Str(key)
	if err != nil || s == "" {
		return s, err
	}
	if _, err := uuid.Parse(s); err != nil {
		return "", argErrorf("%s must be an id (a UUID such as the \"id\" fields returned by other tools), got %q", key, s)
	}
	return s, nil
}

// RequiredID returns a required id argument.
func (a Args) RequiredID(key string) (string, error) {
	s, err := a.ID(key)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", argErrorf("%s is required", key)
	}
	return s, nil
}

// Bool returns an optional boolean argument (false when absent).
func (a Args) Bool(key string) (bool, error) {
	v, ok := a[key]
	if !ok || v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, argErrorf("%s must be a boolean", key)
	}
	return b, nil
}

// Int returns an optional integer argument, def when absent.
func (a Args) Int(key string, def int) (int, error) {
	v, ok := a[key]
	if !ok || v == nil {
		return def, nil
	}
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1<<31 {
		return 0, argErrorf("%s must be an integer", key)
	}
	return int(f), nil
}

// Strs returns an optional array-of-strings argument.
func (a Args) Strs(key string) ([]string, error) {
	v, ok := a[key]
	if !ok || v == nil {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, argErrorf("%s must be an array of strings", key)
	}
	out := make([]string, 0, len(raw))
	for i, e := range raw {
		s, ok := e.(string)
		if !ok {
			return nil, argErrorf("%s[%d] must be a string", key, i)
		}
		out = append(out, s)
	}
	return out, nil
}

// toolErrorMessage maps an error from a tool to the text the model sees, and
// whether it is a tool execution error (true) rather than an internal failure.
// Business outcomes become actionable messages; anything else stays opaque.
func toolErrorMessage(err error) (string, bool) {
	var ae *ArgError
	if errors.As(err, &ae) {
		return "Invalid arguments: " + ae.Msg, true
	}
	var ce *domain.CodedError
	if errors.As(err, &ce) && ce.Message != "" {
		return ce.Message, true
	}
	switch {
	case errors.Is(err, domain.ErrValidation):
		return "The request was rejected as invalid: " + err.Error(), true
	case errors.Is(err, domain.ErrNotFound):
		return "Not found, or not visible to this API key.", true
	case errors.Is(err, domain.ErrForbidden):
		return "Not allowed for this API key or role.", true
	case errors.Is(err, domain.ErrConflict):
		return "Conflict with the current state: " + err.Error(), true
	case errors.Is(err, domain.ErrPlanLimit):
		return "The organization's plan limit for this action is reached.", true
	case errors.Is(err, domain.ErrRateLimited):
		return "Rate limited. Wait a moment and retry.", true
	case errors.Is(err, domain.ErrUnauthorized):
		return "Not authorized.", true
	}
	return "", false
}

// --- JSON Schema helpers (2020-12, the MCP default dialect) -----------------

func objectSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func integer(desc string, minimum, maximum int) map[string]any {
	return map[string]any{"type": "integer", "description": desc, "minimum": minimum, "maximum": maximum}
}

func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
