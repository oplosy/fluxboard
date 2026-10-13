'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Loader2, XCircle } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { PasswordInput } from '@/components/ui/password-input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, keepFocus, FormError } from '@/components/auth/field';
import { PasswordChecklist } from '@/components/auth/password-checklist';
import { resetPassword, validateResetToken } from '@/lib/api/auth';
import { ApiError } from '@/lib/api/client';
import { apiFieldErrors, authErrorMessage, isFieldError } from '@/lib/auth/errors';
import { validateConfirm, validateNewPassword } from '@/lib/auth/validation';

function InvalidLink({ reason }: { reason: string }) {
  return (
    <Card>
      <CardHeader>
        <XCircle className="h-8 w-8 text-destructive" />
        <CardTitle>This link can&apos;t be used</CardTitle>
        <CardDescription>{reason} Request a new one — it only takes a moment.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <Link href="/forgot-password">
          <Button className="w-full">Request a new link</Button>
        </Link>
        <Link href="/login">
          <Button variant="ghost" className="w-full">
            Back to sign in
          </Button>
        </Link>
      </CardContent>
    </Card>
  );
}

function ResetForm() {
  const router = useRouter();
  const params = useSearchParams();
  const token = params.get('token') ?? '';

  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [touched, setTouched] = useState<{ password?: boolean; confirm?: boolean }>({});
  const [submitted, setSubmitted] = useState(false);
  const [sentPassword, setSentPassword] = useState('');

  // Check the link up front so nobody types a new password into a dead link.
  const check = useQuery({
    queryKey: ['auth', 'reset-token', token],
    queryFn: () => validateResetToken(token).then(() => true),
    enabled: Boolean(token),
    retry: false,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

  const mutation = useMutation({
    mutationFn: () => resetPassword({ token, password }),
    onSuccess: () => router.replace('/login?notice=password_reset'),
  });

  if (!token) return <InvalidLink reason="This reset link is missing its token." />;
  if (check.isPending) {
    return (
      <Card>
        <CardHeader>
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          <CardTitle>Checking your link…</CardTitle>
        </CardHeader>
      </Card>
    );
  }
  const linkDead =
    (check.error instanceof ApiError && check.error.code === 'token_invalid') ||
    (mutation.error instanceof ApiError && mutation.error.code === 'token_invalid');
  if (linkDead) return <InvalidLink reason="It has expired, was already used, or a newer link replaced it." />;

  const serverPassword = password === sentPassword ? apiFieldErrors(mutation.error).password : undefined;
  const passwordError = serverPassword ?? (submitted || touched.password ? validateNewPassword(password) : undefined);
  const confirmError = submitted || touched.confirm ? validateConfirm(password, confirm) : undefined;
  const banner =
    (check.error ? authErrorMessage(check.error) : null) ??
    (mutation.error && !isFieldError(mutation.error) ? authErrorMessage(mutation.error) : null);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Set a new password</CardTitle>
        <CardDescription>Choose a strong password you don&apos;t use anywhere else. Every signed-in device will be signed out.</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          noValidate
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            setSubmitted(true);
            if (validateNewPassword(password) || validateConfirm(password, confirm)) return;
            setSentPassword(password);
            mutation.mutate();
          }}
        >
          <FormError message={banner} />
          <Field id="password" label="New password" error={passwordError} hint={<PasswordChecklist password={password} />}>
            <PasswordInput
              id="password"
              autoComplete="new-password"
              autoFocus
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onBlur={(e) => e.target.value && setTouched((t) => ({ ...t, password: true }))}
            />
          </Field>
          <Field id="confirm" label="Confirm new password" error={confirmError}>
            <PasswordInput
              id="confirm"
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              onBlur={(e) => e.target.value && setTouched((t) => ({ ...t, confirm: true }))}
            />
          </Field>
          <Button type="submit" className="w-full" isLoading={mutation.isPending} onMouseDown={keepFocus}>
            {mutation.isPending ? 'Saving…' : 'Update password'}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense>
      <ResetForm />
    </Suspense>
  );
}
