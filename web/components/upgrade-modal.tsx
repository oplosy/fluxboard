'use client';

// App-wide mount point for the 402 `plan_limit_exceeded` upgrade modal
// (docs/02 §9, FR-BILL-009). Any mutation that catches a 402 calls
// `useUpgradeModal().open(limitKey)` to raise it. The modal lives above the org
// context (it wraps the whole app in providers.tsx), so it derives the org slug
// from the pathname to link into that org's plans page.

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { createContext, useContext, useState, type ReactNode } from 'react';

import { usePresence } from '@/lib/motion/hooks';

interface UpgradeModalValue {
  /** Raise the upgrade prompt; `limit` is the offending entitlement key. */
  open: (limit?: string) => void;
}

const UpgradeModalContext = createContext<UpgradeModalValue | null>(null);

// Friendly copy for the entitlement keys the API reports on a 402.
const LIMIT_COPY: Record<string, string> = {
  max_members: 'You’ve reached the member limit for your current plan.',
  max_projects: 'You’ve reached the project limit for your current plan.',
  max_storage_bytes: 'You’ve reached the storage limit for your current plan.',
  members: 'You’ve reached the member limit for your current plan.',
  seats: 'You’ve reached the seat limit for your current plan.',
  plan_limit: 'You’ve reached a limit on your current plan.',
};

function slugFromPath(pathname: string): string | null {
  const m = pathname.match(/^\/app\/([^/]+)/);
  const slug = m?.[1];
  if (!slug || slug === 'new-organization') return null;
  return slug;
}

export function UpgradeModalProvider({ children }: { children: ReactNode }) {
  const [limit, setLimit] = useState<string | null>(null);
  // Keep the last limit around so the copy doesn't vanish during the exit animation.
  const [shown, setShown] = useState<string>('plan_limit');
  const pathname = usePathname();
  const slug = slugFromPath(pathname);
  const { mounted, state } = usePresence(limit !== null, 260);

  const close = () => setLimit(null);

  return (
    <UpgradeModalContext.Provider
      value={{
        open: (l) => {
          setShown(l ?? 'plan_limit');
          setLimit(l ?? 'plan_limit');
        },
      }}
    >
      {children}
      {mounted ? (
        <div
          data-state={state}
          className="fixed inset-0 z-[60] flex items-center justify-center bg-ink/55 p-4 backdrop-blur-[3px] transition-opacity duration-300 data-[state=closed]:opacity-0"
          onClick={close}
          role="dialog"
          aria-modal="true"
          aria-labelledby="upgrade-title"
        >
          <div
            data-state={state}
            className="relative w-full max-w-md overflow-hidden rounded-lg border border-border bg-card p-6 shadow-[0_40px_80px_-30px_hsl(var(--ink)/0.6)] transition-[transform,opacity] duration-400 ease-spring data-[state=closed]:translate-y-4 data-[state=closed]:scale-95 data-[state=closed]:opacity-0"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="absolute inset-x-0 top-0 h-[3px] bg-signal" aria-hidden />
            <p className="font-mono text-[10px] uppercase tracking-[0.18em] text-muted-foreground">Plan limit reached</p>
            <h2 id="upgrade-title" className="mt-3 text-2xl font-semibold tracking-[-0.03em]">
              Upgrade your plan
            </h2>
            <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
              {LIMIT_COPY[shown] ?? LIMIT_COPY.plan_limit} Upgrade to raise your limits and keep going.
            </p>
            <div className="mt-6 flex justify-end gap-2">
              <button
                className="inline-flex h-10 items-center justify-center rounded-md px-4 text-sm font-medium transition-colors hover:bg-secondary"
                onClick={close}
              >
                Not now
              </button>
              {slug ? (
                <Link
                  href={`/app/${slug}/billing/plans`}
                  onClick={close}
                  className="inline-flex h-10 items-center justify-center rounded-md bg-foreground px-4 text-sm font-medium text-background transition-colors hover:bg-signal hover:text-signal-foreground"
                >
                  View plans
                </Link>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </UpgradeModalContext.Provider>
  );
}

export function useUpgradeModal(): UpgradeModalValue {
  const ctx = useContext(UpgradeModalContext);
  if (!ctx) throw new Error('useUpgradeModal must be used within <UpgradeModalProvider>');
  return ctx;
}
