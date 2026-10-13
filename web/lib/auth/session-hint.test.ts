import { afterEach, describe, expect, it } from 'vitest';

import { setAccessToken } from '@/lib/api/client';
import { clearSession, hasSessionHint, markSession, shouldRestoreSession } from './session-hint';

describe('session hint', () => {
  afterEach(() => clearSession());

  it('follows the access token', () => {
    setAccessToken('a.b.c');
    expect(hasSessionHint()).toBe(true);
    setAccessToken(null);
    expect(hasSessionHint()).toBe(false);
  });

  it('skips the refresh on public pages for signed-out visitors', () => {
    expect(shouldRestoreSession('/')).toBe(false);
    expect(shouldRestoreSession('/login')).toBe(false);
    expect(shouldRestoreSession('/verify-email/confirm')).toBe(false);
    markSession();
    expect(shouldRestoreSession('/')).toBe(true);
  });

  it('always tries on pages that need a session', () => {
    for (const p of ['/app', '/app/acme/projects', '/account/security', '/admin', '/verify-email', '/invite/tok']) {
      expect(shouldRestoreSession(p)).toBe(true);
    }
    expect(shouldRestoreSession('/application')).toBe(false);
  });
});
