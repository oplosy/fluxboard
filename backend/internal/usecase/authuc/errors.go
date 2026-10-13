package authuc

import (
	"errors"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/argon2x"
)

// Stable error codes the web client maps to user-facing messages. Messages here
// are safe defaults for API consumers; the client may phrase them differently.
var (
	errInvalidCredentials = domain.NewCodedError(domain.ErrUnauthorized,
		"invalid_credentials", "Incorrect email or password.")
	errTokenInvalid = domain.NewCodedError(domain.ErrValidation,
		"token_invalid", "This link is invalid, has expired, or was already used.")
	errEmailInvalid = domain.NewFieldError("email",
		"email_invalid", "Enter a valid email address.")
	errNameTooLong = domain.NewFieldError("name",
		"name_too_long", "Name must be at most 100 characters.")
	errCurrentPasswordIncorrect = domain.NewFieldError("current_password",
		"current_password_incorrect", "Your current password is incorrect.")
	errPasswordUnchanged = domain.NewFieldError("new_password",
		"password_unchanged", "Choose a password different from your current one.")
	errPasswordNotSet = domain.NewCodedError(domain.ErrForbidden,
		"password_not_set", "This account signs in with Google or GitHub. Use “Forgot password” to set a password.")
	errInvalidCode = domain.NewFieldError("code",
		"invalid_code", "That code is incorrect or has expired.")
	errPending2FAExpired = domain.NewCodedError(domain.ErrUnauthorized,
		"pending_2fa_expired", "Your sign-in attempt expired. Sign in again.")
	errInvalidCode2FA = domain.NewCodedError(domain.ErrUnauthorized,
		"invalid_code", "That code is incorrect or has expired.")
	errAccountLocked = domain.NewCodedError(domain.ErrForbidden,
		"account_locked", "This account is locked. Contact support.")
)

// errEmailTaken is a conflict carried on the email field so the form can point
// at it (registration reveals it deliberately, ADR-032).
var errEmailTaken = &domain.CodedError{
	Kind: domain.ErrConflict, Code: "email_taken", Field: "email",
	Message: "An account with this email already exists.",
}

// policyError maps an argon2x policy rejection onto the given request field.
// Non-policy errors pass through unchanged.
func policyError(err error, field string) error {
	var pe argon2x.PolicyError
	if errors.As(err, &pe) {
		return domain.NewFieldError(field, pe.Code, pe.Reason)
	}
	return err
}
