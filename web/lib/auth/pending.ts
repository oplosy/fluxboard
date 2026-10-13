import { useSyncExternalStore } from 'react';

// sessionStorage key carrying the short-lived pending-2FA token between the
// /login and /login/2fa steps. Kept out of the URL (it's a bearer-ish token).
export const PENDING_2FA_KEY = 'fluxboard.pending_2fa';

// The email typed on one auth screen, carried to the next (sign in → forgot
// password → back) so users don't retype it. sessionStorage rather than a query
// string keeps addresses out of server logs, history and Referer headers.
const EMAIL_KEY = 'fluxboard.auth_email';

export function rememberEmail(email: string): void {
  try {
    const v = email.trim();
    if (v) sessionStorage.setItem(EMAIL_KEY, v);
    else sessionStorage.removeItem(EMAIL_KEY);
  } catch {
    // storage blocked — the user just retypes
  }
}

export function recallEmail(): string {
  try {
    return sessionStorage.getItem(EMAIL_KEY) ?? '';
  } catch {
    return '';
  }
}

const noSubscribe = () => () => {};

/**
 * The remembered email as a render-safe value: '' on the server and during
 * hydration, the stored address on the client. Use it as the default for an
 * email input whose edits live in state.
 */
export function useRememberedEmail(): string {
  return useSyncExternalStore(noSubscribe, recallEmail, () => '');
}
