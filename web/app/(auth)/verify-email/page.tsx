'use client';

import { useRouter } from 'next/navigation';
import { useEffect, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { MailCheck } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { FormError, FormNotice } from '@/components/auth/field';
import { getAuthAccount, resendMyVerification } from '@/lib/api/auth';
import { RequireAuth } from '@/lib/auth/guards';
import { useAuth } from '@/lib/auth/context';
import { authErrorMessage, retryAfterSeconds } from '@/lib/auth/errors';
import { useCooldown } from '@/lib/auth/use-cooldown';

function VerifyEmailInner() {
  const router = useRouter();
  const { logout, refresh, verified } = useAuth();
  const me = useQuery({ queryKey: ['auth', 'account'], queryFn: getAuthAccount });
  const email = me.data?.email ?? '';
  const { remaining, start } = useCooldown();
  const [resent, setResent] = useState(false);
  const [notYet, setNotYet] = useState(false);

  // Verified (here, after a refresh, or in another tab) → straight into the app.
  useEffect(() => {
    if (verified) router.replace('/app');
  }, [verified, router]);

  // Coming back from the inbox tab: re-mint the token to pick up `ver`.
  useEffect(() => {
    const onFocus = () => void refresh();
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, [refresh]);

  const resend = useMutation({
    mutationFn: resendMyVerification,
    onSuccess: () => {
      setResent(true);
      start(60);
    },
    onError: (err) => start(retryAfterSeconds(err) ?? 30),
  });

  const check = useMutation({
    mutationFn: () => refresh(),
    // `verified` flips via the token if it worked; otherwise say so.
    onSuccess: () => setNotYet(true),
  });

  return (
    <Card>
      <CardHeader>
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/5">
          <MailCheck className="h-6 w-6 text-primary" />
        </div>
        <CardTitle>Verify your email</CardTitle>
        <CardDescription>
          {email ? (
            <>
              We sent a link to <span className="font-medium text-foreground">{email}</span>. Open it to unlock your
              workspace — this page continues on its own once you have.
            </>
          ) : (
            'Open the link we emailed you to unlock your workspace.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <FormNotice message={resent ? 'A new link is on its way. Check spam if you don’t see it.' : null} variant="success" />
        <FormNotice
          message={notYet && !verified ? 'Not verified yet. Click the link in the email, then try again.' : null}
        />
        <FormError message={resend.error ? authErrorMessage(resend.error) : null} />
        <Button className="w-full" isLoading={check.isPending} onClick={() => check.mutate()}>
          I&apos;ve verified my email
        </Button>
        <Button
          variant="outline"
          className="w-full"
          isLoading={resend.isPending}
          disabled={remaining > 0}
          onClick={() => resend.mutate()}
        >
          {remaining > 0 ? `Resend in ${remaining}s` : 'Resend verification email'}
        </Button>
        <button
          type="button"
          onClick={() => void logout().then(() => router.replace('/login?notice=signed_out'))}
          className="w-full pt-2 text-center text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          Sign out
        </button>
      </CardContent>
    </Card>
  );
}

export default function VerifyEmailPage() {
  return (
    <RequireAuth>
      <VerifyEmailInner />
    </RequireAuth>
  );
}
