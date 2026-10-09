'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Plus, ArrowRight, LogOut, User } from 'lucide-react';

import { Skeleton } from '@/components/ui/skeleton';
import { Logo } from '@/components/brand/logo';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { listMyOrgs } from '@/lib/api/orgs';

// Org switcher hub (docs/02 §3, FR-TEN-001). NOTE: the sitemap's "auto-redirect
// when exactly one org" lands the user in the org shell (/app/{slug}), which is
// built in a later Phase-7 section; until then we always render the picker so the
// user is never bounced into a not-yet-existing route.
export default function OrgSwitcherPage() {
  const { data: orgs, isLoading, isError } = useQuery({
    queryKey: ['orgs'],
    queryFn: listMyOrgs,
  });

  return (
    <div className="blueprint min-h-screen">
      <div className="mx-auto flex min-h-screen max-w-xl flex-col px-5 py-8">
        <div className="flex items-center justify-between">
          <Logo href="/app" />
          <div className="flex items-center gap-1">
            <Link
              href="/account/profile"
              className="flex h-9 items-center gap-2 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            >
              <User className="h-4 w-4" /> Account
            </Link>
            <ThemeToggle compact />
          </div>
        </div>

        <div className="my-auto py-12">
          <p className="kicker animate-fade-in">Choose a workspace</p>
          <h1 className="mt-3 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] text-5xl font-semibold tracking-[-0.05em]">
            Your organizations
          </h1>

          <div className="mt-10">
            {isLoading ? (
              <div className="space-y-2">
                {[1, 2, 3].map((i) => (
                  <Skeleton key={i} className="h-[72px] w-full rounded-lg" />
                ))}
              </div>
            ) : isError ? (
              <p className="rounded-lg border border-destructive/30 bg-destructive/[0.06] p-6 text-center text-sm text-destructive">
                Couldn&apos;t load your organizations.
              </p>
            ) : orgs && orgs.length > 0 ? (
              <ul className="overflow-hidden rounded-lg border border-border bg-card">
                {orgs.map((o, i) => (
                  <li
                    key={o.org_id}
                    className="animate-[slide-up_0.55s_var(--ease-out-expo)_backwards] border-b border-border last:border-0"
                    style={{ animationDelay: `${120 + i * 60}ms` }}
                  >
                    <Link href={`/app/${o.slug}`} className="group relative flex items-center gap-4 p-4 transition-colors hover:bg-secondary/50">
                      <span className="absolute inset-y-0 left-0 w-[3px] origin-bottom scale-y-0 bg-signal transition-transform duration-400 ease-out-expo group-hover:scale-y-100" />
                      <span className="flex h-11 w-11 items-center justify-center rounded-md bg-foreground font-display text-lg font-semibold text-background transition-transform duration-500 ease-spring group-hover:-rotate-6 group-hover:scale-105">
                        {o.name.slice(0, 1).toUpperCase()}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium">{o.name}</span>
                        <span className="block font-mono text-[11px] text-muted-foreground">
                          {o.slug} · {o.role.toLowerCase()}
                        </span>
                      </span>
                      <ArrowRight className="h-4 w-4 text-muted-foreground transition-[transform,color] duration-500 ease-spring group-hover:translate-x-1 group-hover:text-foreground" />
                    </Link>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="rounded-lg border border-dashed border-foreground/20 p-10 text-center">
                <p className="font-medium">No organizations yet</p>
                <p className="mt-1 text-sm text-muted-foreground">Create your first organization to get started.</p>
              </div>
            )}
          </div>

          <Link
            href="/app/new-organization"
            className="group mt-4 flex h-12 items-center justify-center gap-2 rounded-lg border border-dashed border-foreground/25 text-sm font-medium transition-[border-color,background-color] duration-300 hover:border-foreground hover:bg-card"
          >
            <Plus className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-90" /> New organization
          </Link>
        </div>

        <Link
          href="/logout"
          className="flex items-center justify-center gap-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
        >
          <LogOut className="h-3.5 w-3.5" />
          Sign out
        </Link>
      </div>
    </div>
  );
}
