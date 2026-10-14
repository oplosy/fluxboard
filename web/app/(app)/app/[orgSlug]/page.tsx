'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import {
  ArrowUpRight,
  Bell,
  CalendarDays,
  CheckCircle2,
  FolderKanban,
  ListChecks,
  type LucideIcon,
} from 'lucide-react';
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

  const startOfToday = new Date().setHours(0, 0, 0, 0);
  const overdueCount = dueItems.filter((d) => new Date(d.due).getTime() < startOfToday).length;
  const projectById = new Map((projects.data ?? []).map((p) => [p.id, p]));
  const myCountByProject = new Map<string, number>();
  for (const t of assigned.data?.tasks ?? []) {
    myCountByProject.set(t.project_id, (myCountByProject.get(t.project_id) ?? 0) + 1);
  }

  return (
    <div className="mx-auto max-w-[1320px] px-5 py-8 lg:px-10 lg:py-10">
      <header className="mb-10 flex flex-wrap items-end justify-between gap-6">
        <div>
          <p className="eyebrow animate-fade-in">{org.name} / overview</p>
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

      {/* Stat tiles */}
      <div className="mb-8 grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Stat
          label="Assigned to you"
          icon={ListChecks}
          href={`/app/${slug}/my-tasks`}
          value={assigned.data?.total}
          loading={assigned.isLoading}
          index={0}
          hint={
            overdueCount > 0
              ? `${overdueCount} overdue`
              : assigned.data && assigned.data.total > 0
                ? 'Nothing overdue'
                : 'All clear'
          }
          hintTone={overdueCount > 0 ? 'bad' : 'muted'}
        />
        <Stat
          label="Projects"
          icon={FolderKanban}
          href={`/app/${slug}/projects`}
          value={projects.data?.length}
          loading={projects.isLoading}
          index={1}
          hint={projects.data && projects.data.length > 0 ? 'In this workspace' : 'Create your first project'}
        />
        <Stat
          label="Unread"
          icon={Bell}
          href={`/app/${slug}/notifications`}
          value={unread.data}
          loading={unread.isLoading}
          index={2}
          accent={(unread.data ?? 0) > 0}
          hint={(unread.data ?? 0) > 0 ? 'Needs your attention' : 'Inbox zero'}
        />
      </div>

      {/* Due strip (canvas) */}
      <section className="mb-10 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] overflow-hidden rounded-lg border border-border bg-card [animation-delay:200ms]">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border px-5 py-3">
          <div className="flex items-center gap-2">
            <CalendarDays className="h-4 w-4 text-muted-foreground" />
            <p className="eyebrow">Your next two weeks</p>
          </div>
          <div className="flex items-center gap-4 text-xs font-medium text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-foreground/60" /> upcoming
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-signal" /> high priority
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-destructive" /> overdue
            </span>
          </div>
        </div>
        {assigned.isLoading ? (
          <Skeleton className="m-5 h-[150px]" />
        ) : dueItems.length === 0 ? (
          <div className="flex flex-col items-center gap-2 px-5 py-12 text-center">
            <span className="flex h-10 w-10 items-center justify-center rounded-full bg-secondary text-muted-foreground">
              <CalendarDays className="h-5 w-5" />
            </span>
            <p className="text-sm font-medium">A clear fortnight</p>
            <p className="max-w-sm text-sm text-muted-foreground">
              Tasks assigned to you with a due date will drop onto this timeline.
            </p>
          </div>
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
        <Panel title="Assigned to me" icon={ListChecks} index={0}>
          {assigned.isLoading ? (
            <LoadingRows />
          ) : assigned.isError ? (
            <Failed />
          ) : !assigned.data || assigned.data.tasks.length === 0 ? (
            <Empty icon={CheckCircle2} title="Nothing assigned to you" text="New tasks assigned to you will show up here." />
          ) : (
            <ul className="overflow-hidden rounded-lg border border-border bg-card">
              {assigned.data.tasks.slice(0, 8).map((t, i) => {
                const project = projectById.get(t.project_id);
                const overdue = t.due_date ? new Date(t.due_date).getTime() < startOfToday : false;
                return (
                  <li
                    key={t.id}
                    className="animate-[slide-up_0.5s_var(--ease-out-expo)_backwards] border-b border-border last:border-b-0"
                    style={{ animationDelay: `${300 + i * 40}ms` }}
                  >
                    <Link
                      href={project ? `/app/${slug}/projects/${project.key}` : `/app/${slug}/projects`}
                      className="group flex items-center gap-3 px-4 py-3 text-sm transition-colors hover:bg-secondary/60"
                    >
                      <span className="shrink-0 font-mono text-[11px] text-muted-foreground">
                        {project ? `${project.key}-` : '#'}
                        {t.number}
                      </span>
                      <span className="min-w-0 flex-1 truncate">{t.title}</span>
                      {t.due_date ? (
                        <span
                          className={cn(
                            'hidden shrink-0 text-xs sm:inline',
                            overdue ? 'font-medium text-destructive' : 'text-muted-foreground',
                          )}
                        >
                          {new Date(t.due_date).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                        </span>
                      ) : null}
                      <span className="shrink-0">
                        <PriorityBars priority={t.priority as Priority} />
                      </span>
                    </Link>
                  </li>
                );
              })}
            </ul>
          )}
          {assigned.data && assigned.data.total > 8 ? (
            <p className="mt-3 font-mono text-[11px] text-muted-foreground">Showing 8 of {assigned.data.total}</p>
          ) : null}
        </Panel>

        <RecentActivity />

        <div className="lg:col-span-2">
          <Panel
            title="Projects"
            icon={FolderKanban}
            index={2}
            action={
              <Link
                href={`/app/${slug}/projects`}
                className="link-underline text-xs font-medium text-muted-foreground hover:text-foreground"
              >
                All projects →
              </Link>
            }
          >
            {projects.isLoading ? (
              <LoadingRows />
            ) : projects.isError ? (
              <Failed />
            ) : !projects.data || projects.data.length === 0 ? (
              <Empty
                icon={FolderKanban}
                title="No projects yet"
                text="Projects hold your boards and tasks."
                action={
                  <Link href={`/app/${slug}/projects/new`} className="link-underline text-sm font-medium text-foreground">
                    Create a project →
                  </Link>
                }
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                {projects.data.slice(0, 6).map((p, i) => {
                  const mine = myCountByProject.get(p.id) ?? 0;
                  const total = assigned.data?.tasks.length ?? 0;
                  const share = total > 0 ? Math.round((mine / total) * 100) : 0;
                  const dot = p.color || 'hsl(var(--muted-foreground))';
                  return (
                    <Link
                      key={p.id}
                      href={`/app/${slug}/projects/${p.key}`}
                      className="group relative flex animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] flex-col overflow-hidden rounded-lg border border-border bg-card p-4 transition-[border-color,transform,box-shadow] duration-300 ease-out-expo hover:-translate-y-1 hover:border-foreground/30 hover:shadow-[0_18px_40px_-24px_hsl(var(--ink)/0.5)]"
                      style={{ animationDelay: `${350 + i * 60}ms` }}
                    >
                      <span
                        className="absolute inset-x-0 top-0 h-[3px] origin-left scale-x-[0.18] transition-transform duration-500 ease-out-expo group-hover:scale-x-100"
                        style={{ backgroundColor: dot }}
                      />
                      <span className="flex items-center justify-between gap-2">
                        <span className="inline-flex items-center gap-1.5 rounded-sm bg-secondary px-1.5 py-0.5 font-mono text-[11px] font-medium text-secondary-foreground">
                          <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: dot }} />
                          {p.key}
                        </span>
                        <ArrowUpRight className="h-4 w-4 shrink-0 text-muted-foreground transition-[transform,color] duration-500 ease-spring group-hover:rotate-45 group-hover:text-foreground" />
                      </span>
                      <span className="mt-4 truncate font-display text-lg font-semibold tracking-[-0.02em]">{p.name}</span>
                      <span className="mt-1 line-clamp-2 min-h-[2rem] text-xs text-muted-foreground">
                        {p.description || 'No description yet.'}
                      </span>
                      <span className="mt-4 flex items-center justify-between text-[11px] text-muted-foreground">
                        <span>{mine > 0 ? `${mine} assigned to you` : 'Nothing assigned to you'}</span>
                        <span className="tabular-nums">{share}%</span>
                      </span>
                      <span className="mt-1.5 h-1 overflow-hidden rounded-full bg-secondary" aria-hidden>
                        <span
                          className="block h-full rounded-full bg-foreground/70 transition-[width,background-color] duration-700 ease-out-expo group-hover:bg-signal"
                          style={{ width: `${share}%` }}
                        />
                      </span>
                    </Link>
                  );
                })}
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
  icon: Icon,
  href,
  hint,
  hintTone = 'muted',
  accent = false,
}: {
  label: string;
  value: number | undefined;
  loading: boolean;
  index: number;
  icon: LucideIcon;
  href: string;
  hint: string;
  hintTone?: 'muted' | 'bad';
  accent?: boolean;
}) {
  return (
    <Link
      href={href}
      className="group animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] rounded-lg border border-border bg-card p-5 transition-[border-color,transform,box-shadow] duration-300 ease-out-expo hover:-translate-y-0.5 hover:border-foreground/30 hover:shadow-[0_14px_30px_-22px_hsl(var(--ink)/0.5)]"
      style={{ animationDelay: `${index * 70}ms` }}
    >
      <div className="flex items-center justify-between">
        <p className="text-xs font-medium text-muted-foreground">{label}</p>
        <Icon
          className={cn('h-4 w-4 text-muted-foreground transition-colors group-hover:text-foreground', accent && 'text-signal')}
        />
      </div>
      <div className={cn('mt-3 font-display text-4xl font-semibold tracking-[-0.04em]', accent && 'text-signal')}>
        {loading ? <Skeleton className="h-9 w-16" /> : <CountUp value={value ?? 0} duration={1100} />}
      </div>
      <p className={cn('mt-2 text-xs', hintTone === 'bad' ? 'font-medium text-destructive' : 'text-muted-foreground')}>
        {loading ? ' ' : hint}
      </p>
    </Link>
  );
}

function Panel({
  title,
  icon: Icon,
  index,
  action,
  children,
}: {
  title: string;
  icon?: LucideIcon;
  index: number;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="animate-[slide-up_0.6s_var(--ease-out-expo)_backwards]" style={{ animationDelay: `${250 + index * 80}ms` }}>
      <div className="mb-3 flex items-center justify-between border-b border-border pb-2">
        <h2 className="flex items-center gap-2 text-sm font-medium">
          {Icon ? <Icon className="h-4 w-4 text-muted-foreground" /> : null}
          {title}
        </h2>
        {action}
      </div>
      {children}
    </section>
  );
}

function LoadingRows() {
  return (
    <div className="space-y-3 pt-2">
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
    </div>
  );
}
function Failed() {
  return <p className="py-3 text-sm text-destructive">Couldn&apos;t load.</p>;
}
function Empty({ icon: Icon, title, text, action }: { icon: LucideIcon; title: string; text: string; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border px-5 py-10 text-center">
      <span className="flex h-10 w-10 items-center justify-center rounded-full bg-secondary text-muted-foreground">
        <Icon className="h-5 w-5" />
      </span>
      <p className="text-sm font-medium">{title}</p>
      <p className="max-w-xs text-sm text-muted-foreground">{text}</p>
      {action}
    </div>
  );
}

function RecentActivity() {
  const { orgId, slug } = useOrg();
  const { data, isLoading, isError } = useQuery({
    queryKey: ['home-activity', orgId],
    queryFn: () => listNotifications(orgId, { limit: 8 }),
  });

  return (
    <Panel
      title="Recent activity"
      icon={Bell}
      index={1}
      action={
        <Link
          href={`/app/${slug}/notifications`}
          className="link-underline text-xs font-medium text-muted-foreground hover:text-foreground"
        >
          View all →
        </Link>
      }
    >
      {isLoading ? (
        <LoadingRows />
      ) : isError ? (
        <Failed />
      ) : !data || data.length === 0 ? (
        <Empty icon={Bell} title="No recent activity" text="Mentions, assignments and comments will appear here." />
      ) : (
        <ul className="overflow-hidden rounded-lg border border-border bg-card">
          {data.map((n, i) => (
            <li
              key={n.id}
              className="flex animate-[slide-up_0.5s_var(--ease-out-expo)_backwards] gap-3 border-b border-border px-4 py-3 text-sm last:border-b-0"
              style={{ animationDelay: `${350 + i * 45}ms` }}
            >
              <span
                className={cn('mt-1.5 h-2 w-2 shrink-0 rounded-full', n.read_at ? 'bg-foreground/20' : 'bg-signal')}
                aria-hidden
              />
              <div className="min-w-0 flex-1">
                <p className={cn('truncate', n.read_at ? 'text-muted-foreground' : 'font-medium')}>{n.title}</p>
                {n.body ? <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{n.body}</p> : null}
              </div>
              <time
                className="shrink-0 text-[11px] text-muted-foreground"
                dateTime={n.created_at}
                title={new Date(n.created_at).toLocaleString()}
              >
                {new Date(n.created_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
              </time>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}
