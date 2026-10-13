'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useRef, useState } from 'react';
import { CheckCircle2, XCircle, Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { confirmEmail } from '@/lib/api/auth';
import { ApiError } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import { authErrorMessage } from '@/lib/auth/errors';
import { hasSessionHint } from '@/lib/auth/session-hint';

type State = 'pending' | 'success' | 'invalid' | 'error';

function ConfirmInner() {
  const params = useSearchParams();
  const token = params.get('token') ?? '';
  const { loading, isAuthenticated, verified, refresh } = useAuth();
  const [state, setState] = useState<State>('pending');
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const fired = useRef(false);

  // Consume the token exactly once. The token is single-use, so the effect must
  // not re-run it: dev StrictMode mounts, cleans up and re-runs effects, and a
  // cleanup that discarded the first (successful) response let the second call
  // fail and show "Link expired". The ref survives that re-run; the response is
  // always applied (setState after a real unmount is a no-op).
  useEffect(() => {
    if (!token || fired.current) return;
    fired.current = true;
    const timeout = window.setTimeout(() => {
      setState((s) => (s === 'pending' ? 'error' : s));
      setErrorMessage('This is taking too long. Check your connection and reload the page.');
    }, 15_000);

    void confirmEmail(token)
      .then(() => {
        setState('success');
        // A signed-in tab gets a fresh token carrying ver=true right away.
        if (hasSessionHint()) void refresh();
      })
      .catch((err: unknown) => {
        if (err instanceof ApiError && err.code === 'token_invalid') setState('invalid');
        else {
          setState('error');
          setErrorMessage(authErrorMessage(err));
        }
      })
      .finally(() => window.clearTimeout(timeout));
  }, [token, refresh]);

  // An already-verified, signed-in user re-opening an old link isn't an error.
  const effective: State = state === 'invalid' && !loading && isAuthenticated && verified ? 'success' : state;
  const signedIn = !loading && isAuthenticated;

  let icon = <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />;
  let title = 'Verifying your email…';
  let desc = 'This only takes a second.';
  let cta: { href: string; label: string; primary: boolean } | null = null;

  if (!token) {
    icon = <XCircle className="h-8 w-8 text-destructive" />;
    title = 'Invalid link';
    desc = 'This verification link is missing its token. Use the button in the email, or request a new link.';
    cta = signedIn ? { href: '/verify-email', label: 'Request a new link', primary: true } : { href: '/login', label: 'Go to sign in', primary: true };
  } else if (effective === 'success') {
    icon = <CheckCircle2 className="h-8 w-8 animate-scale-in text-success" />;
    title = 'Email verified';
    desc = signedIn ? 'You’re all set. Your workspace is unlocked.' : 'You’re all set. Sign in to continue.';
    cta = signedIn ? { href: '/app', label: 'Continue to Fluxboard', primary: true } : { href: '/login?notice=email_verified', label: 'Sign in', primary: true };
  } else if (effective === 'invalid') {
    icon = <XCircle className="h-8 w-8 text-destructive" />;
    title = 'Link expired or already used';
    desc = signedIn
      ? 'Verification links work once and expire after 24 hours. Send yourself a fresh one.'
      : 'Verification links work once and expire after 24 hours. Sign in and we’ll help you send a fresh one.';
    cta = signedIn ? { href: '/verify-email', label: 'Send a new link', primary: true } : { href: '/login', label: 'Sign in', primary: true };
  } else if (effective === 'error') {
    icon = <XCircle className="h-8 w-8 text-destructive" />;
    title = 'We couldn’t verify your email';
    desc = errorMessage ?? 'Something went wrong. Reload the page to try again.';
    cta = { href: '/login', label: 'Go to sign in', primary: false };
  }

  return (
    <Card>
      <CardHeader>
        {icon}
        <CardTitle>{title}</CardTitle>
        <CardDescription>{desc}</CardDescription>
      </CardHeader>
      {cta ? (
        <CardContent>
          <Link href={cta.href}>
            <Button className="w-full" variant={cta.primary ? 'default' : 'outline'}>
              {cta.label}
            </Button>
          </Link>
        </CardContent>
      ) : null}
    </Card>
  );
}

export default function ConfirmEmailPage() {
  return (
    <Suspense>
      <ConfirmInner />
    </Suspense>
  );
}
