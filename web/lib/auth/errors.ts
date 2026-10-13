import { ApiError } from '@/lib/api/client';

// Maps the backend's stable auth error codes (docs/04-AUTH.md §7) to the copy
// the auth screens show. The server message is the fallback for codes this
// table does not know yet; generic HTTP failures get a friendly default.

const MESSAGES: Record<string, string> = {
  // Credentials / session
  invalid_credentials: 'Incorrect email or password.',
  pending_2fa_expired: 'Your sign-in attempt expired. Sign in again.',
  invalid_code: 'That code is incorrect or has expired.',
  account_locked: 'This account is locked. Contact support to restore access.',
  // Registration / profile fields
  email_invalid: 'Enter a valid email address, like name@example.com.',
  email_taken: 'An account with this email already exists.',
  name_too_long: 'Name must be at most 100 characters.',
  // Password policy
  password_too_short: 'Password must be at least 12 characters.',
  password_too_long: 'Password must be at most 128 characters.',
  password_breached: 'This password has appeared in a data breach. Choose a different one.',
  password_matches_email: 'Password must not be the same as your email.',
  password_unchanged: 'Choose a password different from your current one.',
  current_password_incorrect: 'Your current password is incorrect.',
  password_not_set:
    'Your account signs in with Google or GitHub. Use “Forgot password” to set a password.',
  // Links
  token_invalid: 'This link is invalid, has expired, or was already used.',
  // OAuth (arrive as ?oauth_error=<code> on /login)
  oauth_unavailable: 'That sign-in option isn’t available right now.',
  oauth_cancelled: 'Sign-in was cancelled.',
  oauth_failed: 'We couldn’t complete sign-in with that provider. Try again.',
  oauth_state_invalid:
    'That sign-in attempt expired or was started in another browser. Please try again.',
  oauth_email_unverified:
    'Your provider account has no verified email address. Verify it there, then try again.',
  oauth_verify_first:
    'An account with this email exists but isn’t verified yet. Sign in with your password and verify your email first.',
};

/** User-facing copy for a known auth error code, or undefined. */
export function messageForCode(code: string | null | undefined): string | undefined {
  return code ? MESSAGES[code] : undefined;
}

/** Seconds to wait from a 429 envelope (details.retry_after), if present. */
export function retryAfterSeconds(err: unknown): number | undefined {
  if (!(err instanceof ApiError) || err.status !== 429) return undefined;
  const v = err.details?.retry_after;
  return typeof v === 'number' && v > 0 ? v : undefined;
}

function humanizeWait(secs: number): string {
  if (secs < 60) return `${secs} second${secs === 1 ? '' : 's'}`;
  const mins = Math.ceil(secs / 60);
  return `${mins} minute${mins === 1 ? '' : 's'}`;
}

/** Best message for any thrown value from an auth call. */
export function authErrorMessage(err: unknown): string | null {
  if (!err) return null;
  if (err instanceof ApiError) {
    if (err.status === 429) {
      const secs = retryAfterSeconds(err);
      return secs
        ? `Too many attempts. Try again in ${humanizeWait(secs)}.`
        : 'Too many attempts. Please wait a moment and try again.';
    }
    const known = messageForCode(err.code);
    if (known) return known;
    if (err.status >= 500) return 'Something went wrong on our side. Please try again.';
    return err.message || 'Something went wrong. Please try again.';
  }
  if (err instanceof TypeError) return 'Can’t reach the server. Check your connection and try again.';
  return 'Something went wrong. Please try again.';
}

/**
 * Field-level errors from an ApiError: details.fields mapped through the code
 * table (the code describes the single field a coded error names).
 */
export function apiFieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) return {};
  const fields = err.fieldErrors();
  const known = messageForCode(err.code);
  const out: Record<string, string> = {};
  for (const [field, msg] of Object.entries(fields)) out[field] = known ?? msg;
  return out;
}

/** True when the error belongs next to a field rather than in the form banner. */
export function isFieldError(err: unknown): boolean {
  return Object.keys(apiFieldErrors(err)).length > 0;
}
