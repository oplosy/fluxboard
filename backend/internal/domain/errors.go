// Package domain holds the enterprise sentinel errors shared by every domain
// subpackage. The HTTP layer maps these to status codes in exactly one place
// (internal/interface/http/errmap.go). Domain packages import stdlib only.
package domain

import "errors"

var (
	// ErrNotFound: the requested entity does not exist (or is not visible to
	// the caller's tenant). Maps to 404.
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized: authentication is missing or invalid. Maps to 401.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden: authenticated but not authorized for this action. Maps to 403.
	ErrForbidden = errors.New("forbidden")
	// ErrEmailUnverified: the caller's email is not verified; app routes are
	// blocked until they confirm (FR-AUTH-002). Maps to 403 email_unverified.
	ErrEmailUnverified = errors.New("email not verified")
	// ErrConflict: state precondition failed (e.g. duplicate, rank collision).
	// Maps to 409.
	ErrConflict = errors.New("conflict")
	// ErrPlanLimit: entitlement/plan quota exceeded. Maps to 402/403 per route.
	ErrPlanLimit = errors.New("plan limit exceeded")
	// ErrValidation: input failed validation. Maps to 400/422.
	ErrValidation = errors.New("validation failed")
	// ErrRateLimited: too many requests; caller should back off. Maps to 429
	// (with a Retry-After header set by the HTTP layer).
	ErrRateLimited = errors.New("rate limited")
)

// CodedError refines a sentinel with a stable machine-readable Code (rendered as
// the envelope `code`, e.g. "email_taken") and, for input problems, the request
// Field it concerns. Kind decides the HTTP status; Message is user-safe. Clients
// branch on Code, never on Message.
type CodedError struct {
	Kind    error
	Code    string
	Field   string
	Message string
	Details map[string]any
}

func (e *CodedError) Error() string { return e.Message }
func (e *CodedError) Unwrap() error { return e.Kind }

// NewCodedError builds a CodedError not tied to a single request field.
func NewCodedError(kind error, code, message string) *CodedError {
	return &CodedError{Kind: kind, Code: code, Message: message}
}

// NewFieldError builds a validation CodedError for one request field.
func NewFieldError(field, code, message string) *CodedError {
	return &CodedError{Kind: ErrValidation, Code: code, Field: field, Message: message}
}
