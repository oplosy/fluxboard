'use client';

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

import {
  apiFetch,
  getAccessToken,
  onAccessTokenChange,
  refreshSession,
  setAccessToken,
} from '@/lib/api/client';
import type { TokenResponse } from '@/lib/api/types';
import { decodeAccessClaims, type AccessClaims } from './jwt';

interface AuthState {
  /** True until the initial cookie → refresh bootstrap resolves. */
  loading: boolean;
  token: string | null;
  claims: AccessClaims | null;
  userId: string | null;
  /** Email verified at token-issue time (JWT `ver`). */
  verified: boolean;
  /** Impersonated org id when a read-only admin session is active (JWT `imp`). */
  impersonating: string | null;
  isAuthenticated: boolean;
}

interface AuthContextValue extends AuthState {
  /** Adopt a freshly minted access token (login / 2fa / oauth). */
  adopt: (tok: TokenResponse) => void;
  /** Revoke the session server-side and clear local state. */
  logout: () => Promise<void>;
  /** Force a silent refresh; returns whether a session is now active. */
  refresh: () => Promise<boolean>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/** How long before access-token expiry the proactive refresh fires. */
const REFRESH_LEAD_MS = 60_000;

function deriveState(token: string | null, loading: boolean): AuthState {
  const claims = decodeAccessClaims(token);
  return {
    loading,
    token,
    claims,
    userId: claims?.sub ?? null,
    verified: Boolean(claims?.ver),
    impersonating: claims?.imp ?? null,
    isAuthenticated: Boolean(token),
  };
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => getAccessToken());
  const [loading, setLoading] = useState(true);

  // Mirror the module-level token store into React state so components re-render.
  useEffect(() => onAccessTokenChange(setToken), []);

  // Bootstrap: exchange the refresh cookie for an access token on first mount.
  useEffect(() => {
    let active = true;
    void refreshSession().finally(() => {
      if (active) setLoading(false);
    });
    return () => {
      active = false;
    };
  }, []);

  // Refresh shortly before the access token expires, so requests (and the SSE
  // stream) never hit a 401 first. A tab that slept past expiry refreshes as
  // soon as it becomes visible again.
  useEffect(() => {
    const exp = decodeAccessClaims(token)?.exp;
    if (!exp) return;
    const delay = Math.max(5_000, exp * 1000 - Date.now() - REFRESH_LEAD_MS);
    const timer = window.setTimeout(() => void refreshSession(), delay);
    const onVisible = () => {
      if (document.visibilityState === 'visible' && exp * 1000 - Date.now() < REFRESH_LEAD_MS) {
        void refreshSession();
      }
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener('visibilitychange', onVisible);
    };
  }, [token]);

  const adopt = useCallback((tok: TokenResponse) => {
    setAccessToken(tok.access_token);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiFetch('/auth/logout', { method: 'POST', skipAuthRetry: true });
    } catch {
      // best effort — clear locally regardless
    }
    setAccessToken(null);
  }, []);

  const refresh = useCallback(() => refreshSession(), []);

  const value = useMemo<AuthContextValue>(
    () => ({ ...deriveState(token, loading), adopt, logout, refresh }),
    [token, loading, adopt, logout, refresh],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>');
  return ctx;
}
