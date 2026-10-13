'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useRef } from 'react';
import { Loader2 } from 'lucide-react';

import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuth } from '@/lib/auth/context';
import { PENDING_2FA_KEY } from '@/lib/auth/pending';
import { sanitizeNext } from '@/lib/nav/safe-next';

// The backend OAuth callback (Google or GitHub) redirects here as
//   /oauth/callback?next=<path>                       — session cookie set, or
//   /oauth/callback?next=<path>#pending_token=<jwt>   — the account needs 2FA.
// No access token ever travels in the URL: we mint one from the refresh cookie.
// Failures never land here; the backend sends them to /login?oauth_error=<code>.
export default function OAuthCallbackPage() {
  const router = useRouter();
  const { refresh } = useAuth();
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return; // StrictMode re-runs effects; this must run once
    ran.current = true;

    const next = sanitizeNext(new URLSearchParams(window.location.search).get('next'));
    const hash = window.location.hash.startsWith('#') ? window.location.hash.slice(1) : '';
    const pending = new URLSearchParams(hash).get('pending_token');
    // Drop the fragment from history before anything else.
    window.history.replaceState(null, '', window.location.pathname + window.location.search);

    if (pending) {
      try {
        sessionStorage.setItem(PENDING_2FA_KEY, pending);
      } catch {
        router.replace('/login?oauth_error=oauth_failed');
        return;
      }
      router.replace(`/login/2fa?next=${encodeURIComponent(next)}`);
      return;
    }

    void refresh().then((ok) => {
      router.replace(ok ? next : '/login?oauth_error=oauth_failed');
    });
  }, [refresh, router]);

  return (
    <Card>
      <CardHeader>
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        <CardTitle>Signing you in…</CardTitle>
        <CardDescription>Finishing up with your provider.</CardDescription>
      </CardHeader>
    </Card>
  );
}
