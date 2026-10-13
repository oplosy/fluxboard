'use client';

import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { Suspense, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { MailCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/ui/password-input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, keepFocus, FormError, FormNotice } from '@/components/auth/field';
import { OAuthButtons } from '@/components/auth/oauth-buttons';
import { PasswordChecklist } from '@/components/auth/password-checklist';
import { register, requestEmailVerification } from '@/lib/api/auth';
import { ApiError } from '@/lib/api/client';
import { apiFieldErrors, authErrorMessage, isFieldError, retryAfterSeconds } from '@/lib/auth/errors';
import { rememberEmail } from '@/lib/auth/pending';
import { useCooldown } from '@/lib/auth/use-cooldown';
import { validateEmail, validateName, validateNewPassword } from '@/lib/auth/validation';
import { sanitizeNext } from '@/lib/nav/safe-next';

type FieldName = 'name' | 'email' | 'password';

function RegisterForm() {
  const params = useSearchParams();
  const next = sanitizeNext(params.get('next'));
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [touched, setTouched] = useState<Partial<Record<FieldName, boolean>>>({});
  const [submitted, setSubmitted] = useState(false);
  // Values as sent, so a server-side field error disappears once that field is edited.
  const [sent, setSent] = useState({ name: '', email: '', password: '' });

  const mutation = useMutation({
    mutationFn: (input: { name: string; email: string; password: string }) => register(input),
  });

  const serverFields = apiFieldErrors(mutation.error);
  const current = { name, email, password };
  const fieldError = (f: FieldName, local: string | undefined): string | undefined => {
    if (serverFields[f] && current[f] === sent[f]) return serverFields[f];
    return submitted || touched[f] ? local : undefined;
  };
  const nameError = fieldError('name', validateName(name));
  const emailError = fieldError('email', validateEmail(email));
  const passwordError = fieldError('password', validateNewPassword(password, email));
  const emailTaken =
    mutation.error instanceof ApiError && mutation.error.code === 'email_taken' && email === sent.email;

  const banner = mutation.error && !isFieldError(mutation.error) ? authErrorMessage(mutation.error) : null;

  if (mutation.isSuccess) {
    return <CheckInbox email={sent.email} onRestart={() => mutation.reset()} />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create your account</CardTitle>
        <CardDescription>Start managing projects in minutes.</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          noValidate
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            setSubmitted(true);
            if (validateName(name) || validateEmail(email) || validateNewPassword(password, email)) return;
            const input = { name: name.trim(), email: email.trim(), password };
            setSent({ name, email, password });
            mutation.mutate(input);
          }}
        >
          <FormError message={banner} />
          <Field id="name" label="Name" error={nameError}>
            <Input
              id="name"
              autoComplete="name"
              autoFocus
              placeholder="Ada Lovelace"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={(e) => e.target.value && setTouched((t) => ({ ...t, name: true }))}
            />
          </Field>
          <Field
            id="email"
            label="Email"
            error={emailError}
            errorAction={
              emailTaken ? (
                <span className="flex flex-wrap gap-x-3 gap-y-1">
                  <Link href="/login" onClick={() => rememberEmail(email)} className="underline underline-offset-2">
                    Sign in instead
                  </Link>
                  <Link href="/forgot-password" onClick={() => rememberEmail(email)} className="underline underline-offset-2">
                    Reset password
                  </Link>
                </span>
              ) : undefined
            }
          >
            <Input
              id="email"
              type="email"
              inputMode="email"
              autoComplete="email"
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
            hint={<PasswordChecklist password={password} email={email} />}
          >
            <PasswordInput
              id="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onBlur={(e) => e.target.value && setTouched((t) => ({ ...t, password: true }))}
            />
          </Field>
          <Button type="submit" className="w-full" isLoading={mutation.isPending} onMouseDown={keepFocus}>
            {mutation.isPending ? 'Creating account…' : 'Create account'}
          </Button>
        </form>

        <OAuthButtons next={next} />

        <p className="mt-6 text-center text-sm text-muted-foreground">
          Already have an account?{' '}
          <Link href="/login" onClick={() => rememberEmail(email)} className="text-primary hover:underline">
            Sign in
          </Link>
        </p>
      </CardContent>
    </Card>
  );
}

// Post-signup screen: where the link went, how to get another, and a way back
// if the address was mistyped.
function CheckInbox({ email, onRestart }: { email: string; onRestart: () => void }) {
  const { remaining, start } = useCooldown();
  const [resent, setResent] = useState(false);
  const resend = useMutation({
    mutationFn: () => requestEmailVerification(email.trim()),
    onSuccess: () => {
      setResent(true);
      start(60);
    },
    onError: (err) => start(retryAfterSeconds(err) ?? 30),
  });

  return (
    <Card className="text-center">
      <CardHeader>
        <div className="mx-auto flex h-14 w-14 animate-scale-in items-center justify-center rounded-full bg-success/10">
          <MailCheck className="h-7 w-7 text-success" />
        </div>
        <CardTitle className="mt-2">Check your inbox</CardTitle>
        <CardDescription>
          We sent a verification link to <span className="font-medium text-foreground">{email.trim()}</span>. Open it to
          activate your account — it expires in 24 hours.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 text-left">
        <FormNotice message={resent ? 'A new link is on its way. Check spam if you don’t see it.' : null} variant="success" />
        <FormError message={resend.error ? authErrorMessage(resend.error) : null} />
        <Link href="/login" onClick={() => rememberEmail(email)}>
          <Button className="w-full">Go to sign in</Button>
        </Link>
        <Button
          variant="outline"
          className="w-full"
          isLoading={resend.isPending}
          disabled={remaining > 0}
          onClick={() => resend.mutate()}
        >
          {remaining > 0 ? `Resend in ${remaining}s` : 'Resend verification email'}
        </Button>
        <p className="pt-1 text-center text-[13px] text-muted-foreground">
          Wrong address?{' '}
          <button type="button" onClick={onRestart} className="text-primary hover:underline">
            Use a different email
          </button>
        </p>
      </CardContent>
    </Card>
  );
}

export default function RegisterPage() {
  return (
    <Suspense>
      <RegisterForm />
    </Suspense>
  );
}
