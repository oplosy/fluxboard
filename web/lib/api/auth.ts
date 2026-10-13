import { API_BASE, apiFetch } from './client';
import type {
  Enroll2FAResponse,
  LoginResponse,
  SessionInfo,
  TokenResponse,
} from './types';

// Typed wrappers over the /auth/* surface (docs/04-AUTH.md §5). The refresh call
// itself lives in client.ts (it must run before a token exists).

export function register(input: { email: string; password: string; name: string }): Promise<void> {
  return apiFetch('/auth/register', { method: 'POST', body: input });
}

export function login(input: { email: string; password: string }): Promise<LoginResponse> {
  // Login runs before a session exists; don't trigger the 401 refresh/retry.
  return apiFetch<LoginResponse>('/auth/login', {
    method: 'POST',
    body: input,
    skipAuthRetry: true,
  });
}

export function verify2FA(input: { pending_token: string; code: string }): Promise<TokenResponse> {
  return apiFetch<TokenResponse>('/auth/2fa/verify', {
    method: 'POST',
    body: input,
    skipAuthRetry: true,
  });
}

export function requestEmailVerification(email: string): Promise<void> {
  return apiFetch('/auth/verify-email/request', {
    method: 'POST',
    body: { email },
    skipAuthRetry: true,
  });
}

/** Minimal account view, available before the email is verified. */
export interface AuthAccount {
  id: string;
  email: string;
  name: string;
  email_verified: boolean;
  has_password: boolean;
}

export function getAuthAccount(): Promise<AuthAccount> {
  return apiFetch('/auth/me');
}

/** Re-sends the verification link to the signed-in caller's own address. */
export function resendMyVerification(): Promise<void> {
  return apiFetch('/auth/verify-email/resend', { method: 'POST' });
}

export function confirmEmail(token: string): Promise<void> {
  return apiFetch('/auth/verify-email/confirm', {
    method: 'POST',
    body: { token },
    skipAuthRetry: true,
  });
}

export function forgotPassword(email: string): Promise<void> {
  return apiFetch('/auth/password/forgot', {
    method: 'POST',
    body: { email },
    skipAuthRetry: true,
  });
}

export function resetPassword(input: { token: string; password: string }): Promise<void> {
  return apiFetch('/auth/password/reset', { method: 'POST', body: input, skipAuthRetry: true });
}

/** Checks a reset link without consuming it (204 valid, 422 token_invalid). */
export function validateResetToken(token: string): Promise<void> {
  return apiFetch('/auth/password/reset/validate', {
    method: 'POST',
    body: { token },
    skipAuthRetry: true,
  });
}

// --- OAuth (Google, GitHub) ---

export type OAuthProvider = 'google' | 'github';

/** The sign-in providers the server has credentials for. */
export function getOAuthProviders(): Promise<{ providers: OAuthProvider[] }> {
  return apiFetch('/auth/oauth/providers', { skipAuthRetry: true });
}

/**
 * Full-page URL that starts a provider sign-in. `next` is the in-app path to
 * land on afterwards (the server re-sanitizes it).
 */
export function oauthStartUrl(provider: OAuthProvider, next: string): string {
  return `${API_BASE}/auth/oauth/${provider}/start?redirect=${encodeURIComponent(next)}`;
}

export function changePassword(input: {
  current_password: string;
  new_password: string;
}): Promise<void> {
  return apiFetch('/auth/password/change', { method: 'POST', body: input });
}

// --- Sessions & 2FA management (used by §3 account) ---

export function listSessions(): Promise<{ items: SessionInfo[] }> {
  return apiFetch('/auth/sessions');
}

export function revokeSession(id: string): Promise<void> {
  return apiFetch(`/auth/sessions/${id}`, { method: 'DELETE' });
}

export function logoutAll(): Promise<void> {
  return apiFetch('/auth/logout-all', { method: 'POST' });
}

export function enroll2FA(): Promise<Enroll2FAResponse> {
  return apiFetch('/auth/2fa/enroll', { method: 'POST' });
}

export function activate2FA(code: string): Promise<{ recovery_codes: string[] }> {
  return apiFetch('/auth/2fa/activate', { method: 'POST', body: { code } });
}

export function disable2FA(code: string): Promise<void> {
  return apiFetch('/auth/2fa', { method: 'DELETE', body: { code } });
}

// --- Invitations ---

export function acceptInvitation(token: string): Promise<void> {
  return apiFetch('/invitations/accept', { method: 'POST', body: { token } });
}
