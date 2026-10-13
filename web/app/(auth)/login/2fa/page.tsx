'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState, useSyncExternalStore } from 'react';
import { useMutation } from '@tanstack/react-query';
import { ShieldCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, FormError } from '@/components/auth/field';
import { verify2FA } from '@/lib/api/auth';
import { ApiError } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import { authErrorMessage } from '@/lib/auth/errors';
import { PENDING_2FA_KEY } from '@/lib/auth/pending';
import { sanitizeNext } from '@/lib/nav/safe-next';
import { useTransitionNav } from '@/components/motion/route-transition';

// sessionStorage has no change events for this tab; read it once per render.
const noSubscribe = () => () => {};
const readPending = () => sessionStorage.getItem(PENDING_2FA_KEY);

function TwoFactorForm() {
  const router = useRouter();
  const { go } = useTransitionNav();
  const params = useSearchParams();
  const next = sanitizeNext(params.get('next'));
  const { adopt } = useAuth();

  const pending = useSyncExternalStore(noSubscribe, readPending, () => null);
  const [mode, setMode] = useState<'totp' | 'recovery'>('totp');
  const [code, setCode] = useState('');
  const [submitted, setSubmitted] = useState(false);

  // The pending token was stashed by /login (or the OAuth callback). Without it
  // there's nothing to verify.
  useEffect(() => {
    if (!readPending()) router.replace('/login');
  }, [router]);

  const mutation = useMutation({
    mutationFn: (value: string) => verify2FA({ pending_token: pending ?? '', code: value }),
    onSuccess: (tok) => {
      sessionStorage.removeItem(PENDING_2FA_KEY);
      adopt(tok);
      go(next, { replace: true });
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 'pending_2fa_expired') {
        sessionStorage.removeItem(PENDING_2FA_KEY);
        router.replace('/login?notice=session_expired');
        return;
      }
      setCode('');
    },
  });

  const clean = mode === 'totp' ? code.replace(/\D/g, '') : code.replace(/[\s-]/g, '').toUpperCase();
  const localError = !submitted
    ? undefined
    : mode === 'totp'
      ? clean.length !== 6
        ? 'Enter the 6-digit code from your authenticator app.'
        : undefined
      : clean.length !== 10
        ? 'Recovery codes are 10 characters long.'
        : undefined;
  const codeError = localError ?? (mutation.error && !mutation.isPending ? authErrorMessage(mutation.error) ?? undefined : undefined);

  const submit = (value: string) => {
    setSubmitted(true);
    const ok = mode === 'totp' ? /^\d{6}$/.test(value) : value.length === 10;
    if (!ok || mutation.isPending) return;
    mutation.mutate(value);
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/5">
          <ShieldCheck className="h-6 w-6 text-primary" />
        </div>
        <CardTitle>Two-factor authentication</CardTitle>
        <CardDescription>
          {mode === 'totp'
            ? 'Enter the 6-digit code from your authenticator app.'
            : 'Enter one of the 10-character recovery codes you saved when you turned on 2FA. Each code works once.'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          noValidate
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            submit(clean);
          }}
        >
          <FormError
            message={
              mutation.error instanceof ApiError && mutation.error.status === 429 ? authErrorMessage(mutation.error) : null
            }
          />
          <Field id="code" label={mode === 'totp' ? 'Authentication code' : 'Recovery code'} error={codeError}>
            <Input
              id="code"
              key={mode}
              inputMode={mode === 'totp' ? 'numeric' : 'text'}
              autoComplete="one-time-code"
              autoCapitalize={mode === 'recovery' ? 'characters' : undefined}
              spellCheck={false}
              maxLength={mode === 'totp' ? 7 : 12}
              autoFocus
              placeholder={mode === 'totp' ? '123456' : 'ABCDE12345'}
              className="text-center font-mono text-lg tracking-[0.35em]"
              value={code}
              onChange={(e) => {
                const v = e.target.value;
                setCode(v);
                // Submit as soon as a full TOTP code is typed or pasted.
                const digits = v.replace(/\D/g, '');
                if (mode === 'totp' && digits.length === 6 && !mutation.isPending) submit(digits);
              }}
            />
          </Field>
          <Button type="submit" className="w-full" isLoading={mutation.isPending} disabled={!pending}>
            {mutation.isPending ? 'Verifying…' : 'Verify'}
          </Button>
        </form>
        <div className="mt-6 flex flex-col items-center gap-2 text-sm">
          <button
            type="button"
            className="text-primary hover:underline"
            onClick={() => {
              setMode((m) => (m === 'totp' ? 'recovery' : 'totp'));
              setCode('');
              setSubmitted(false);
              mutation.reset();
            }}
          >
            {mode === 'totp' ? 'Use a recovery code instead' : 'Use your authenticator app instead'}
          </button>
          <Link
            href="/login"
            onClick={() => sessionStorage.removeItem(PENDING_2FA_KEY)}
            className="text-muted-foreground hover:text-foreground"
          >
            Back to sign in
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}

export default function TwoFactorPage() {
  return (
    <Suspense>
      <TwoFactorForm />
    </Suspense>
  );
}
