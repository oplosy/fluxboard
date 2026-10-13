'use client';

import Link from 'next/link';
import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { MailCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, FormError, FormNotice, keepFocus } from '@/components/auth/field';
import { forgotPassword } from '@/lib/api/auth';
import { authErrorMessage, retryAfterSeconds } from '@/lib/auth/errors';
import { rememberEmail, useRememberedEmail } from '@/lib/auth/pending';
import { useCooldown } from '@/lib/auth/use-cooldown';
import { validateEmail } from '@/lib/auth/validation';

export default function ForgotPasswordPage() {
  // Prefilled from an earlier auth screen until the user types.
  const remembered = useRememberedEmail();
  const [typedEmail, setEmail] = useState<string | null>(null);
  const email = typedEmail ?? remembered;
  const [touched, setTouched] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [sentCount, setSentCount] = useState(0);
  const { remaining, start } = useCooldown();

  const mutation = useMutation({
    mutationFn: () => forgotPassword(email.trim()),
    onSuccess: () => {
      rememberEmail(email);
      setSentCount((n) => n + 1);
      start(60);
    },
    onError: (err) => {
      const secs = retryAfterSeconds(err);
      if (secs) start(secs);
    },
  });

  const emailError = submitted || touched ? validateEmail(email) : undefined;
  const send = () => {
    setSubmitted(true);
    if (validateEmail(email)) return;
    mutation.mutate();
  };

  // The API answers the same whether or not the address has an account (no
  // enumeration), so a resolved call always shows this confirmation.
  if (sentCount > 0) {
    return (
      <Card className="text-center">
        <CardHeader>
          <div className="mx-auto flex h-14 w-14 animate-scale-in items-center justify-center rounded-full bg-success/10">
            <MailCheck className="h-7 w-7 text-success" />
          </div>
          <CardTitle className="mt-2">Check your inbox</CardTitle>
          <CardDescription>
            If an account exists for <span className="font-medium text-foreground">{email.trim()}</span>, a reset link is
            on its way. It expires in 1 hour.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-left">
          <FormNotice message={sentCount > 1 ? 'Sent again. Only the newest link matters once you use it.' : null} variant="success" />
          <FormError message={mutation.error ? authErrorMessage(mutation.error) : null} />
          <p className="text-[13px] text-muted-foreground">
            Nothing after a few minutes? Check your spam folder, or make sure you used the address you signed up with.
          </p>
          <Button variant="outline" className="w-full" isLoading={mutation.isPending} disabled={remaining > 0} onClick={send}>
            {remaining > 0 ? `Resend in ${remaining}s` : 'Resend link'}
          </Button>
          <Link href="/login">
            <Button variant="ghost" className="w-full">
              Back to sign in
            </Button>
          </Link>
          <p className="text-center text-[13px] text-muted-foreground">
            Wrong address?{' '}
            <button type="button" onClick={() => setSentCount(0)} className="text-primary hover:underline">
              Change email
            </button>
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Forgot your password?</CardTitle>
        <CardDescription>
          Enter your account email and we&apos;ll send you a link to choose a new one. Signed up with Google or GitHub?
          This lets you add a password too.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          noValidate
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            send();
          }}
        >
          <FormError message={authErrorMessage(mutation.error)} />
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
              onBlur={(e) => e.target.value && setTouched(true)}
            />
          </Field>
          <Button type="submit" className="w-full" isLoading={mutation.isPending} disabled={remaining > 0} onMouseDown={keepFocus}>
            {mutation.isPending ? 'Sending…' : remaining > 0 ? `Try again in ${remaining}s` : 'Send reset link'}
          </Button>
        </form>
        <p className="mt-6 text-center text-sm text-muted-foreground">
          Remembered it?{' '}
          <Link href="/login" onClick={() => rememberEmail(email)} className="text-primary hover:underline">
            Back to sign in
          </Link>
        </p>
      </CardContent>
    </Card>
  );
}
