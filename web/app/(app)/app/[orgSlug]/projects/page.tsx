'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Plus, Archive, ArrowUpRight } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { listProjects } from '@/lib/api/projects';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

type Filter = 'active' | 'archived';

export default function ProjectsPage() {
  const { orgId, slug } = useOrg();
  const [filter, setFilter] = useState<Filter>('active');

  const { data, isLoading, isError } = useQuery({
    queryKey: ['projects', orgId, filter],
    queryFn: () => listProjects(orgId, { archived: filter === 'archived' }),
  });

  // The archived list includes active projects too; when viewing "archived",
  // keep only the ones actually archived.
  const projects = filter === 'archived' ? (data ?? []).filter((p) => p.archived_at) : (data ?? []);

  return (
    <div className="mx-auto max-w-[1320px] px-5 py-8 lg:px-10 lg:py-10">
      <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="kicker">Workspace / projects</p>
          <h1 className="mt-3 animate-[slide-up_0.7s_var(--ease-out-expo)_backwards] text-4xl font-semibold tracking-[-0.05em] sm:text-5xl">
            Projects
            {data ? <sup className="ml-2 align-top font-mono text-sm font-normal tracking-normal text-muted-foreground">{projects.length}</sup> : null}
          </h1>
        </div>
        <Link
          href={`/app/${slug}/projects/new`}
          className="group inline-flex h-10 items-center gap-2 rounded-md bg-foreground px-4 text-sm font-medium text-background transition-[background-color,color,transform] duration-300 hover:bg-signal hover:text-signal-foreground active:scale-[0.97]"
        >
          <Plus className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:rotate-90" /> New project
        </Link>
      </div>

      {/* Segmented filter with a sliding thumb */}
      <div className="relative mb-8 inline-grid grid-cols-2 rounded-md border border-border bg-card p-1 text-sm">
        <span
          aria-hidden
          className="absolute bottom-1 left-1 top-1 w-[calc(50%-4px)] rounded-sm bg-foreground transition-transform duration-500 ease-spring"
          style={{ transform: filter === 'archived' ? 'translateX(100%)' : 'translateX(0)' }}
        />
        {(['active', 'archived'] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            aria-pressed={filter === f}
            className={cn(
              'relative z-[1] px-5 py-1.5 capitalize transition-colors duration-300',
              filter === f ? 'text-background' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {f}
          </button>
        ))}
      </div>

      {isLoading ? (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-[136px] rounded-lg" />
          ))}
        </div>
      ) : isError ? (
        <p className="text-sm text-destructive">Couldn&apos;t load projects.</p>
      ) : projects.length === 0 ? (
        <div className="flex animate-fade-in flex-col items-center rounded-lg border border-dashed border-foreground/20 px-6 py-20 text-center">
          <div className="flex h-12 items-end gap-1.5" aria-hidden>
            {[0.4, 0.75, 0.3, 0.6].map((h, i) => (
              <span
                key={i}
                className={cn('w-3 origin-bottom animate-bar-bounce rounded-[2px]', i === 3 ? 'bg-signal' : 'bg-foreground/20')}
                style={{ height: `${h * 100}%`, animationDelay: `${i * 140}ms` }}
              />
            ))}
          </div>
          <p className="mt-6 text-sm text-muted-foreground">{filter === 'archived' ? 'No archived projects.' : 'No projects yet.'}</p>
          {filter === 'active' ? (
            <Link
              href={`/app/${slug}/projects/new`}
              className="link-underline mt-3 font-mono text-xs uppercase tracking-[0.14em] text-foreground"
            >
              Create your first project →
            </Link>
          ) : null}
        </div>
      ) : (
        <div key={filter} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {projects.map((p, i) => (
            <Link
              key={p.id}
              href={`/app/${slug}/projects/${p.key}`}
              className="group relative flex min-h-[136px] animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] flex-col overflow-hidden rounded-lg border border-border bg-card p-5 transition-[border-color,transform,box-shadow] duration-300 ease-out-expo hover:-translate-y-1 hover:border-foreground/30 hover:shadow-[0_18px_40px_-24px_hsl(var(--ink)/0.5)]"
              style={{ animationDelay: `${Math.min(i, 12) * 45}ms` }}
            >
              <span
                className="absolute inset-y-0 left-0 w-[3px] origin-top scale-y-[0.3] transition-transform duration-500 ease-out-expo group-hover:scale-y-100"
                style={{ backgroundColor: p.color || 'hsl(var(--muted-foreground))' }}
                aria-hidden
              />
              <div className="flex items-center justify-between">
                <span className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">{p.key}</span>
                {p.archived_at ? (
                  <span className="inline-flex items-center gap-1 font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                    <Archive className="h-3 w-3" /> Archived
                  </span>
                ) : (
                  <ArrowUpRight className="h-4 w-4 text-muted-foreground transition-[transform,color] duration-500 ease-spring group-hover:rotate-45 group-hover:text-foreground" />
                )}
              </div>
              <span className="mt-auto pt-6 font-display text-xl font-semibold tracking-[-0.025em]">{p.name}</span>
              {p.description ? <p className="mt-1 line-clamp-1 text-sm text-muted-foreground">{p.description}</p> : null}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
