// A non-secret marker cookie on the web origin meaning "this browser probably
// holds a refresh cookie". The refresh cookie itself is httpOnly (and scoped to
// the API), so scripts cannot see it; without this marker every page view by a
// signed-out visitor would fire a doomed POST /auth/refresh (a 401 in the
// console and a wasted round trip). It carries no token and grants nothing —
// the server still decides on every request.

const NAME = 'fluxboard_session';
// Matches the 7-day sliding refresh TTL; renewed on every successful refresh.
const MAX_AGE_SECONDS = 7 * 24 * 60 * 60;

// Pages that need a session to render. They always try to restore one, so a
// browser that has a refresh cookie but no marker (signed in before the marker
// existed, or site data partly cleared) is not sent to /login by mistake.
const NEEDS_SESSION = [/^\/app(\/|$)/, /^\/account(\/|$)/, /^\/admin(\/|$)/, /^\/verify-email\/?$/, /^\/invite\//];

export function hasSessionHint(): boolean {
  if (typeof document === 'undefined') return false;
  try {
    return document.cookie.split(';').some((c) => c.trim() === `${NAME}=1`);
  } catch {
    return false;
  }
}

export function markSession(): void {
  if (typeof document === 'undefined') return;
  const secure = window.location.protocol === 'https:' ? '; Secure' : '';
  try {
    document.cookie = `${NAME}=1; Path=/; Max-Age=${MAX_AGE_SECONDS}; SameSite=Lax${secure}`;
  } catch {
    // cookies blocked — we just keep probing, as before the marker existed
  }
}

export function clearSession(): void {
  if (typeof document === 'undefined') return;
  try {
    document.cookie = `${NAME}=; Path=/; Max-Age=0; SameSite=Lax`;
  } catch {
    // nothing to clear
  }
}

/** Whether the app should try POST /auth/refresh when it boots on `pathname`. */
export function shouldRestoreSession(pathname: string): boolean {
  return hasSessionHint() || NEEDS_SESSION.some((re) => re.test(pathname));
}
