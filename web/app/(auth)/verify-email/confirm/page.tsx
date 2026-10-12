'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useRef, useState } from 'react';
import { CheckCircle2, XCircle, Loader2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { confirmEmail } from '@/lib/api/auth';

function ConfirmInner() {
  const params = useSearchParams();
  const token = params.get('token') ?? '';
  const [state, setState] = useState<'idle' | 'pending' | 'success' | 'error'>('idle');
  const fired = useRef(false);

  // Consume the token exactly once. The token is single-use, so the effect must
  // not re-run it: dev StrictMode mounts, cleans up and re-runs effects, and a
  // cleanup that discarded the first (successful) response let the second call
  // fail and show "Link expired". The ref survives that re-run; the response is
  // always applied (setState after a real unmount is a no-op).
  useEffect(() => {
    if (!token || fired.current) return;
    fired.current = true;
    const timeout = window.setTimeout(() => setState((s) => (s === 'pending' ? 'error' : s)), 10_000);

    setState('pending');
    void confirmEmail(token)
      .then(() => setState('success'))
      .catch(() => setState('error'))
      .finally(() => window.clearTimeout(timeout));
  }, [token]);

  let icon = <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />;
  let title = 'Verifying…';
  let desc = 'Confirming your email address.';

  if (!token) {
    icon = <XCircle className="h-8 w-8 text-destructive" />;
    title = 'Invalid link';
    desc = 'This verification link is missing its token.';
  } else if (state === 'success') {
    icon = <CheckCircle2 className="h-8 w-8 text-primary" />;
    title = 'Email verified';
    desc = 'Your email is confirmed. You can now sign in.';
  } else if (state === 'error') {
    icon = <XCircle className="h-8 w-8 text-destructive" />;
    title = 'Link expired';
    desc = 'This verification link is invalid or has already been used.';
  }

  return (
    <Card>
      <CardHeader>
        {icon}
        <CardTitle>{title}</CardTitle>
        <CardDescription>{desc}</CardDescription>
      </CardHeader>
      {(state === 'success' || state === 'error' || !token) && (
        <CardContent>
          <Link href="/login">
            <Button className="w-full" variant={state === 'success' ? 'default' : 'outline'}>
              Go to sign in
            </Button>
          </Link>
        </CardContent>
      )}
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
