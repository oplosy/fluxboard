'use client';

import { useRouter } from 'next/navigation';
import { useEffect, type ReactNode } from 'react';

import { restoreSession } from '@/lib/api/client';
import { useAuth } from './context';
import { Spinner } from '@/components/ui/spinner';

function Loading() {
  return (
    <div className="flex min-h-[40vh] items-center justify-center text-foreground">
      <Spinner size="lg" />
    </div>
  );
}

/**
 * Sends a signed-out visitor to /login (with a post-login `next`). If this page
 * load never tried to restore a session — the app booted on a public page
 * without the session marker and navigated here client-side — it tries once
 * first, so a valid refresh cookie is never ignored.
 */
function useLoginRedirect(loading: boolean, isAuthenticated: boolean): void {
  const router = useRouter();

  useEffect(() => {
    if (loading || isAuthenticated) return;
    let cancelled = false;
    const toLogin = () => {
      if (cancelled) return;
      const next = encodeURIComponent(window.location.pathname + window.location.search);
      router.replace(`/login?next=${next}`);
    };
    // Nested guards share one attempt; only a settled failure redirects.
    const pending = restoreSession();
    if (!pending) toLogin();
    else void pending.then((ok) => (ok ? undefined : toLogin()));
    return () => {
      cancelled = true;
    };
  }, [loading, isAuthenticated, router]);
}

/**
 * Gate for the `A` legend: requires a live session. Redirects to /login (with a
 * post-login `next`) once the bootstrap resolves without a token.
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { loading, isAuthenticated } = useAuth();
  useLoginRedirect(loading, isAuthenticated);

  if (loading) return <Loading />;
  if (!isAuthenticated) return <Loading />;
  return <>{children}</>;
}

/**
 * Gate for the `V` legend: requires a session AND a verified email. Unverified
 * users are sent to the blocking /verify-email screen.
 */
export function RequireVerified({ children }: { children: ReactNode }) {
  const { loading, isAuthenticated, verified } = useAuth();
  const router = useRouter();
  useLoginRedirect(loading, isAuthenticated);

  useEffect(() => {
    if (!loading && isAuthenticated && !verified) router.replace('/verify-email');
  }, [loading, isAuthenticated, verified, router]);

  if (loading || !isAuthenticated || !verified) return <Loading />;
  return <>{children}</>;
}

/**
 * Gate for the `PA` legend. platform_role is NOT in the access token — it comes
 * from GET /me (added in §B). Until that wiring lands this guard only ensures a
 * session; the admin surface itself is out of scope for this run.
 */
export function RequirePlatformAdmin({ children }: { children: ReactNode }) {
  return <RequireAuth>{children}</RequireAuth>;
}
