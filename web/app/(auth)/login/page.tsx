'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useState } from 'react';
import { useMutation } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/ui/password-input';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { Field, keepFocus, FormError, FormNotice } from '@/components/auth/field';
import { OAuthButtons } from '@/components/auth/oauth-buttons';
import { login } from '@/lib/api/auth';
import { ApiError } from '@/lib/api/client';
import { isTwoFactorRequired } from '@/lib/api/types';
import { useAuth } from '@/lib/auth/context';
import { authErrorMessage, messageForCode } from '@/lib/auth/errors';
import { PENDING_2FA_KEY, rememberEmail, useRememberedEmail } from '@/lib/auth/pending';
import { validateEmail } from '@/lib/auth/validation';
import { sanitizeNext } from '@/lib/nav/safe-next';
import { useTransitionNav } from '@/components/motion/route-transition';

// One-off notices other auth screens hand over via ?notice=<key>.
const NOTICES: Record<string, string> = {
  password_reset: 'Your password was updated. Sign in with your new password.',
  password_changed: 'Your password was changed and every session was signed out. Sign in again.',
  email_verified: 'Your email is verified. Sign in to continue.',
  signed_out: 'You’re signed out.',
  session_expired: 'Your session expired. Sign in again to continue.',
};

function LoginForm() {
  const router = useRouter();
  const { go } = useTransitionNav();
  const params = useSearchParams();
  const next = sanitizeNext(params.get('next'));
  const notice = NOTICES[params.get('notice') ?? ''];
  const oauthError = messageForCode(params.get('oauth_error')) ?? (params.get('oauth_error') ? messageForCode('oauth_failed') : undefined);
  const { adopt } = useAuth();

  // Prefilled from an earlier auth screen until the user types.
  const remembered = useRememberedEmail();
  const [typedEmail, setEmail] = useState<string | null>(null);
  const email = typedEmail ?? remembered;
  const [password, setPassword] = useState('');
  const [touched, setTouched] = useState<{ email?: boolean; password?: boolean }>({});
  const [submitted, setSubmitted] = useState(false);

  const mutation = useMutation({
    mutationFn: () => login({ email: email.trim(), password }),
    onSuccess: (res) => {
      if (isTwoFactorRequired(res)) {
        sessionStorage.setItem(PENDING_2FA_KEY, res.pending_token);
        router.push(`/login/2fa?next=${encodeURIComponent(next)}`);
        return;
      }
      adopt(res);
      go(next, { replace: true });
    },
    onError: (err) => {
      // Keep the email, clear the password so the retry starts clean — and
      // don't flag the now-empty field; the banner already says what happened.
      if (err instanceof ApiError && err.status === 401) {
        setPassword('');
        setSubmitted(false);
        setTouched((t) => ({ ...t, password: false }));
      }
    },
  });

  const show = (f: 'email' | 'password') => submitted || touched[f];
  const emailError = show('email') ? validateEmail(email) : undefined;
  const passwordError = show('password') && !password ? 'Enter your password.' : undefined;

  const err = mutation.error;
  const banner = authErrorMessage(err) ?? (mutation.isIdle ? oauthError : null);
  const bannerAction =
    err instanceof ApiError && err.code === 'invalid_credentials' ? (
      <Link href="/forgot-password" onClick={() => rememberEmail(email)} className="underline underline-offset-2">
        Reset your password
      </Link>
    ) : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>Sign in</CardTitle>
        <CardDescription>Welcome back to Fluxboard.</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          noValidate
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            setSubmitted(true);
            if (validateEmail(email) || !password) return;
            mutation.mutate();
          }}
        >
          {banner ? <FormError message={banner} action={bannerAction} /> : <FormNotice message={notice} variant="success" />}
          <Field id="email" label="Email" error={emailError}>
            <Input
              id="email"
              type="email"
              inputMode="email"
              autoComplete="email"
              autoFocus
              placeholder="name@company.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onBlur={(e) => e.target.value && setTouched((t) => ({ ...t, email: true }))}
            />
          </Field>
          <Field
            id="password"
            label="Password"
            error={passwordError}
            labelAside={
              <Link
                href="/forgot-password"
                onClick={() => rememberEmail(email)}
                className="text-[13px] text-muted-foreground underline-offset-2 transition-colors hover:text-foreground hover:underline"
              >
                Forgot password?
              </Link>
            }
          >
            <PasswordInput
              id="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onBlur={(e) => e.target.value && setTouched((t) => ({ ...t, password: true }))}
            />
          </Field>
          <Button type="submit" className="w-full" isLoading={mutation.isPending} onMouseDown={keepFocus}>
            {mutation.isPending ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>

        <OAuthButtons next={next} />

        <p className="mt-6 text-center text-sm text-muted-foreground">
          No account?{' '}
          <Link href={`/register${next !== '/app' ? `?next=${encodeURIComponent(next)}` : ''}`} className="text-primary hover:underline">
            Create one
          </Link>
        </p>
      </CardContent>
    </Card>
  );
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}
