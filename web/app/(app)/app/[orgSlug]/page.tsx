'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { ArrowUpRight } from 'lucide-react';
import type { ReactNode } from 'react';

import { Skeleton } from '@/components/ui/skeleton';
import { DueStrip } from '@/components/canvas/due-strip';
import { CountUp } from '@/components/motion/effects';
import { PriorityBars } from '@/components/board/priority-bars';
import { useAuth } from '@/lib/auth/context';
import { useOrg } from '@/lib/org/context';
import { searchTasks } from '@/lib/api/tasks';
import { listProjects } from '@/lib/api/projects';
import { listNotifications, getUnreadCount } from '@/lib/api/notifications';
import { getMe } from '@/lib/api/user';
import type { Priority } from '@/lib/api/types';
import { cn } from '@/lib/utils';

function greeting(d = new Date()) {
  const h = d.getHours();
  if (h < 5) return 'Working late';
  if (h < 12) return 'Good morning';
  if (h < 18) return 'Good afternoon';
  return 'Good evening';
}

export default function OrgHomePage() {
  const { org, orgId, slug } = useOrg();
  const { userId } = useAuth();
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: getMe });

  const assigned = useQuery({
    queryKey: ['home-assigned', orgId, userId],
    queryFn: () => searchTasks(orgId, { assignee_id: userId ?? '', limit: 50 }),
    enabled: Boolean(userId),
  });
  const projects = useQuery({ queryKey: ['home-projects', orgId], queryFn: () => listProjects(orgId, {}) });
  const unread = useQuery({ queryKey: ['unread-count', orgId], queryFn: () => getUnreadCount(orgId) });

  const firstName = me?.name?.split(' ')[0];
  const dueItems =
    assigned.data?.tasks
      .filter((t) => t.due_date)
      .map((t) => ({ id: t.id, title: t.title, due: t.due_date as string, priority: t.priority })) ?? [];

  return (
    <div className="mx-auto max-w-[1320px] px-5 py-8 lg:px-10 lg:py-10">
      <header className="mb-10 flex flex-wrap items-end justify-between gap-6">
        <div>
          <p className="kicker animate-fade-in">
            {org.name} / overview
          </p>
          <h1 className="mt-3 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] text-4xl font-semibold tracking-[-0.05em] sm:text-5xl">
            {greeting()}
            {firstName ? <span className="text-muted-foreground">, {firstName}</span> : null}
            <span className="text-signal">.</span>
          </h1>
        </div>
        <Link
          href={`/app/${slug}/projects/new`}
          className="group inline-flex h-10 items-center gap-2 rounded-md bg-foreground px-4 text-sm font-medium text-background transition-[background-color,color,transform] duration-300 hover:bg-signal hover:text-signal-foreground active:scale-[0.97]"
        >
          New project
          <ArrowUpRight className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-45" />
        </Link>
      </header>

      {/* Stat strip */}
      <div className="mb-8 grid grid-cols-1 gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-3">
        <Stat label="Assigned to you" value={assigned.data?.total} loading={assigned.isLoading} index={0} />
        <Stat label="Projects" value={projects.data?.length} loading={projects.isLoading} index={1} />
        <Stat label="Unread" value={unread.data} loading={unread.isLoading} index={2} accent={(unread.data ?? 0) > 0} />
      </div>

      {/* Due strip (canvas) */}
      <section className="mb-10 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] rounded-lg border border-border bg-card [animation-delay:200ms]">
        <div className="flex items-center justify-between border-b border-border px-5 py-3">
          <p className="kicker">Your next two weeks</p>
          <div className="flex items-center gap-4 font-mono text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 bg-signal" /> high
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 bg-destructive" /> overdue
            </span>
          </div>
        </div>
        {assigned.isLoading ? (
          <Skeleton className="m-5 h-[150px]" />
        ) : dueItems.length === 0 ? (
          <p className="px-5 py-12 text-center text-sm text-muted-foreground">Nothing with a due date on your plate.</p>
        ) : (
          <>
            <DueStrip items={dueItems} className="h-[190px]" />
            <ul className="sr-only">
              {dueItems.map((d) => (
                <li key={d.id}>
                  {d.title}, due {new Date(d.due).toLocaleDateString()}
                </li>
              ))}
            </ul>
          </>
        )}
      </section>

      <div className="grid gap-x-10 gap-y-10 lg:grid-cols-[1.3fr_0.9fr]">
        <Panel title="Assigned to me" index={0}>
          {assigned.isLoading ? (
            <LoadingRows />
          ) : assigned.isError ? (
            <Failed />
          ) : !assigned.data || assigned.data.tasks.length === 0 ? (
            <Empty text="Nothing assigned to you right now." />
          ) : (
            <ul>
              {assigned.data.tasks.slice(0, 8).map((t, i) => (
                <li
                  key={t.id}
                  className="group flex animate-[slide-up_0.5s_var(--ease-out-expo)_backwards] items-center gap-3 border-b border-border py-3 text-sm"
                  style={{ animationDelay: `${300 + i * 40}ms` }}
                >
                  <span className="font-mono text-[11px] text-muted-foreground">#{t.number}</span>
                  <span className="truncate transition-transform duration-300 ease-out-expo group-hover:translate-x-1">{t.title}</span>
                  <span className="ml-auto shrink-0">
                    <PriorityBars priority={t.priority as Priority} />
                  </span>
                </li>
              ))}
            </ul>
          )}
          {assigned.data && assigned.data.total > 8 ? (
            <p className="mt-3 font-mono text-[11px] text-muted-foreground">
              Showing 8 of {assigned.data.total}
            </p>
          ) : null}
        </Panel>

        <RecentActivity />

        <div className="lg:col-span-2">
          <Panel
            title="Projects"
            index={2}
            action={
              <Link href={`/app/${slug}/projects`} className="link-underline font-mono text-[11px] uppercase tracking-[0.14em] text-muted-foreground hover:text-foreground">
                All projects →
              </Link>
            }
          >
            {projects.isLoading ? (
              <LoadingRows />
            ) : projects.isError ? (
              <Failed />
            ) : !projects.data || projects.data.length === 0 ? (
              <Empty text="No projects yet." />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {projects.data.slice(0, 6).map((p, i) => (
                  <Link
                    key={p.id}
                    href={`/app/${slug}/projects/${p.key}`}
                    className="group relative flex animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] flex-col overflow-hidden rounded-lg border border-border bg-card p-4 transition-[border-color,transform,box-shadow] duration-300 ease-out-expo hover:-translate-y-1 hover:border-foreground/30 hover:shadow-[0_18px_40px_-24px_hsl(var(--ink)/0.5)]"
                    style={{ animationDelay: `${350 + i * 60}ms` }}
                  >
                    <span
                      className="absolute inset-x-0 top-0 h-[3px] origin-left scale-x-[0.18] transition-transform duration-500 ease-out-expo group-hover:scale-x-100"
                      style={{ backgroundColor: p.color || 'hsl(var(--muted-foreground))' }}
                    />
                    <span className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{p.key}</span>
                    <span className="mt-6 flex items-end justify-between gap-2">
                      <span className="truncate font-display text-lg font-semibold tracking-[-0.02em]">{p.name}</span>
                      <ArrowUpRight className="h-4 w-4 shrink-0 text-muted-foreground transition-[transform,color] duration-500 ease-spring group-hover:rotate-45 group-hover:text-foreground" />
                    </span>
                  </Link>
                ))}
              </div>
            )}
          </Panel>
        </div>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  loading,
  index,
  accent = false,
}: {
  label: string;
  value: number | undefined;
  loading: boolean;
  index: number;
  accent?: boolean;
}) {
  return (
    <div className="animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] bg-card p-5" style={{ animationDelay: `${index * 70}ms` }}>
      <p className="font-mono text-[10px] uppercase tracking-[0.16em] text-muted-foreground">{label}</p>
      <div className={cn('mt-3 font-display text-4xl font-semibold tracking-[-0.04em]', accent && 'text-signal')}>
        {loading ? <Skeleton className="h-9 w-16" /> : <CountUp value={value ?? 0} duration={1100} />}
      </div>
    </div>
  );
}

function Panel({ title, index, action, children }: { title: string; index: number; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="animate-[slide-up_0.6s_var(--ease-out-expo)_backwards]" style={{ animationDelay: `${250 + index * 80}ms` }}>
      <div className="mb-2 flex items-center justify-between border-b border-foreground pb-2">
        <h2 className="font-mono text-[11px] font-medium uppercase tracking-[0.16em]">{title}</h2>
        {action}
      </div>
      {children}
    </section>
  );
}

function LoadingRows() {
  return (
    <div className="space-y-3 pt-2">
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-4 w-5/6" />
    </div>
  );
}
function Failed() {
  return <p className="py-3 text-sm text-destructive">Couldn&apos;t load.</p>;
}
function Empty({ text }: { text: string }) {
  return <p className="py-6 text-sm text-muted-foreground">{text}</p>;
}

function RecentActivity() {
  const { orgId } = useOrg();
  const { data, isLoading, isError } = useQuery({
    queryKey: ['home-activity', orgId],
    queryFn: () => listNotifications(orgId, { limit: 8 }),
  });

  return (
    <Panel title="Recent activity" index={1}>
      {isLoading ? (
        <LoadingRows />
      ) : isError ? (
        <Failed />
      ) : !data || data.length === 0 ? (
        <Empty text="No recent activity." />
      ) : (
        <ol className="relative pl-4">
          <span className="absolute bottom-2 left-[3px] top-2 w-px bg-border" aria-hidden />
          {data.map((n, i) => (
            <li
              key={n.id}
              className="relative animate-[slide-up_0.5s_var(--ease-out-expo)_backwards] py-2.5 text-sm"
              style={{ animationDelay: `${350 + i * 45}ms` }}
            >
              <span
                className={cn('absolute -left-4 top-[15px] h-[7px] w-[7px] rounded-full border-2 border-background', n.read_at ? 'bg-foreground/30' : 'bg-signal')}
                aria-hidden
              />
              <p className={n.read_at ? 'text-muted-foreground' : 'font-medium'}>{n.title}</p>
              {n.body ? <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{n.body}</p> : null}
            </li>
          ))}
        </ol>
      )}
    </Panel>
  );
}
