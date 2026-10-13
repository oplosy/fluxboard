'use client';

import { useEffect, useState, type ComponentType } from 'react';
import { useQuery } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { getOAuthProviders, oauthStartUrl, type OAuthProvider } from '@/lib/api/auth';

function GoogleIcon() {
  return (
    <svg className="h-4 w-4" viewBox="0 0 24 24" aria-hidden>
      <path
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
        fill="#4285F4"
      />
      <path
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
        fill="#34A853"
      />
      <path
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
        fill="#FBBC05"
      />
      <path
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
        fill="#EA4335"
      />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M12 .5C5.73.5.75 5.48.75 11.75c0 4.97 3.22 9.18 7.69 10.67.56.1.77-.24.77-.54v-1.9c-3.13.68-3.79-1.51-3.79-1.51-.51-1.3-1.25-1.65-1.25-1.65-1.02-.7.08-.69.08-.69 1.13.08 1.72 1.16 1.72 1.16 1 1.72 2.63 1.22 3.27.93.1-.73.39-1.22.71-1.5-2.5-.28-5.13-1.25-5.13-5.57 0-1.23.44-2.24 1.16-3.03-.12-.28-.5-1.43.11-2.98 0 0 .95-.3 3.1 1.16a10.8 10.8 0 015.64 0c2.15-1.46 3.1-1.16 3.1-1.16.61 1.55.23 2.7.11 2.98.72.79 1.16 1.8 1.16 3.03 0 4.33-2.64 5.28-5.15 5.56.4.35.76 1.03.76 2.08v3.08c0 .3.2.65.78.54 4.46-1.49 7.68-5.7 7.68-10.67C23.25 5.48 18.27.5 12 .5z" />
    </svg>
  );
}

const META: Record<OAuthProvider, { label: string; Icon: ComponentType }> = {
  google: { label: 'Continue with Google', Icon: GoogleIcon },
  github: { label: 'Continue with GitHub', Icon: GitHubIcon },
};

const ORDER: OAuthProvider[] = ['google', 'github'];

// "Continue with …" buttons for the providers the server has configured, plus
// the "or" divider. Renders nothing when none are configured, so the form never
// offers a button that would only bounce back with an error.
export function OAuthButtons({ next }: { next: string }) {
  const { data } = useQuery({
    queryKey: ['auth', 'oauth-providers'],
    queryFn: getOAuthProviders,
    staleTime: 5 * 60_000,
    retry: 1,
  });
  const [leaving, setLeaving] = useState<OAuthProvider | null>(null);
  // Back-button from the provider restores this page from the bfcache with the
  // spinner still on; clear it so the buttons work again.
  useEffect(() => {
    const onShow = (e: PageTransitionEvent) => {
      if (e.persisted) setLeaving(null);
    };
    window.addEventListener('pageshow', onShow);
    return () => window.removeEventListener('pageshow', onShow);
  }, []);
  const providers = ORDER.filter((p) => data?.providers.includes(p));
  if (providers.length === 0) return null;

  return (
    <div className="animate-slide-down">
      <div className="my-5 flex items-center gap-3 font-mono text-[10.5px] uppercase tracking-[0.14em] text-muted-foreground">
        <span className="h-px flex-1 bg-border" />
        or
        <span className="h-px flex-1 bg-border" />
      </div>
      <div className="grid gap-2.5">
        {providers.map((p) => {
          const { label, Icon } = META[p];
          return (
            <Button
              key={p}
              type="button"
              variant="outline"
              className="w-full gap-2"
              isLoading={leaving === p}
              disabled={leaving !== null}
              onClick={() => {
                // Full-page navigation: the provider flow needs top-level redirects.
                setLeaving(p);
                window.location.assign(oauthStartUrl(p, next));
              }}
            >
              <Icon />
              {label}
            </Button>
          );
        })}
      </div>
    </div>
  );
}
