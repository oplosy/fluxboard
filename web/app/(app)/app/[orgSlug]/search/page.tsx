'use client';

import { Suspense, useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import { Search as SearchIcon, X } from 'lucide-react';

import { listLabels } from '@/lib/api/labels';
import { useOrg } from '@/lib/org/context';
import { useOrgMembers } from '@/lib/org/use-members';
import { useProjectsMap } from '@/lib/board/use-projects';
import { PRIORITIES, priorityLabel } from '@/lib/board/priority';
import type { Priority } from '@/lib/api/types';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { SearchResultRow } from '@/components/search/result-row';
import { useDebouncedValue, useTaskSearch } from '@/components/search/use-task-search';

// Full task search (FR-TASK-007). The query and every filter live in the URL so
// a result set is shareable and survives reloads; results show immediately and
// the filters only narrow them.
export default function SearchPage() {
  // useSearchParams requires a Suspense boundary during prerender.
  return (
    <Suspense fallback={<p className="px-6 py-8 text-sm text-muted-foreground">Loading…</p>}>
      <SearchInner />
    </Suspense>
  );
}

const FILTER_KEYS = ['project', 'assignee', 'priority', 'label'] as const;
type FilterKey = (typeof FILTER_KEYS)[number];

function SearchInner() {
  const { orgId, slug } = useOrg();
  const router = useRouter();
  const searchParams = useSearchParams();

  const q = searchParams.get('q') ?? '';
  const filters: Record<FilterKey, string> = {
    project: searchParams.get('project') ?? '',
    assignee: searchParams.get('assignee') ?? '',
    priority: searchParams.get('priority') ?? '',
    label: searchParams.get('label') ?? '',
  };
  const activeFilters = FILTER_KEYS.filter((k) => filters[k]).length;

  const { projects, byId } = useProjectsMap();
  const { members } = useOrgMembers();
  const { data: labels } = useQuery({ queryKey: ['labels', orgId], queryFn: () => listLabels(orgId) });

  // The input is local state so typing is instant; the URL follows after a pause.
  const [input, setInput] = useState(q);
  const debouncedInput = useDebouncedValue(input.trim(), 300);

  function setParams(changes: Partial<Record<'q' | FilterKey, string>>) {
    const next = new URLSearchParams(searchParams.toString());
    for (const [k, v] of Object.entries(changes)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    const qs = next.toString();
    router.replace(`/app/${slug}/search${qs ? `?${qs}` : ''}`, { scroll: false });
  }

  useEffect(() => {
    if (debouncedInput !== q.trim()) setParams({ q: debouncedInput });
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only react to the debounced input
  }, [debouncedInput]);

  // Follow external URL changes (back/forward, the palette's "Advanced search" link).
  const [seenQ, setSeenQ] = useState(q);
  if (seenQ !== q) {
    setSeenQ(q);
    if (input.trim() !== q.trim()) setInput(q);
  }

  const params = {
    q: q.trim(),
    project_id: filters.project,
    assignee_id: filters.assignee,
    priority: filters.priority,
    label_id: filters.label,
    limit: 50,
  };
  const { data, isLoading, isError, isFetching, refetch } = useTaskSearch(orgId, params);

  const results = data?.tasks ?? [];
  const total = data?.total ?? results.length;
  const hasCriteria = Boolean(params.q || activeFilters);

  return (
    <div className="mx-auto w-full max-w-4xl px-4 py-6 sm:px-6">
      <h1 className="sr-only">Search</h1>

      <form
        role="search"
        onSubmit={(e) => {
          e.preventDefault();
          setParams({ q: input.trim() });
        }}
        className="relative"
      >
        <SearchIcon aria-hidden className="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          autoFocus
          type="search"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Search tasks…"
          aria-label="Search tasks"
          className="h-12 w-full rounded-md border border-input bg-card pl-10 pr-10 text-base outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] [&::-webkit-search-cancel-button]:hidden"
        />
        {input ? (
          <button
            type="button"
            aria-label="Clear search"
            onClick={() => {
              setInput('');
              setParams({ q: '' });
            }}
            className="absolute right-2.5 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground"
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </form>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <FilterSelect
          label="Project"
          value={filters.project}
          onChange={(v) => setParams({ project: v })}
          any="Any project"
          options={projects.map((p) => ({ value: p.id, label: p.name }))}
        />
        <FilterSelect
          label="Assignee"
          value={filters.assignee}
          onChange={(v) => setParams({ assignee: v })}
          any="Any assignee"
          options={members.map((m) => ({ value: m.user_id, label: m.name || m.email }))}
        />
        <FilterSelect
          label="Priority"
          value={filters.priority}
          onChange={(v) => setParams({ priority: v })}
          any="Any priority"
          options={PRIORITIES.map((p) => ({ value: p, label: priorityLabel(p as Priority) }))}
        />
        <FilterSelect
          label="Label"
          value={filters.label}
          onChange={(v) => setParams({ label: v })}
          any="Any label"
          options={(labels ?? []).map((l) => ({ value: l.id, label: l.name }))}
        />
        {activeFilters > 0 ? (
          <button
            type="button"
            onClick={() => setParams({ project: '', assignee: '', priority: '', label: '' })}
            className="inline-flex h-8 items-center gap-1 rounded-md px-2 text-[13px] text-muted-foreground hover:bg-secondary hover:text-foreground"
          >
            <X className="h-3.5 w-3.5" /> Clear filters
          </button>
        ) : null}
      </div>

      <div className="mt-5" aria-live="polite">
        {isLoading ? (
          <ResultsSkeleton />
        ) : isError ? (
          <div role="alert" className="rounded-md border border-border px-4 py-8 text-center text-sm">
            <p className="text-destructive">Search failed.</p>
            <button type="button" onClick={() => refetch()} className="mt-1 text-xs underline underline-offset-2">
              Try again
            </button>
          </div>
        ) : results.length === 0 ? (
          <div className="rounded-md border border-dashed border-border px-4 py-12 text-center text-sm text-muted-foreground">
            {hasCriteria ? (
              <>
                No tasks match
                {params.q ? (
                  <>
                    {' '}
                    <span className="font-medium text-foreground">&ldquo;{params.q}&rdquo;</span>
                  </>
                ) : null}
                {activeFilters ? ' with these filters' : ''}.
              </>
            ) : (
              'No tasks yet.'
            )}
          </div>
        ) : (
          <>
            <p className="eyebrow mb-2">
              {total} result{total === 1 ? '' : 's'}
              {total > results.length ? ` · showing ${results.length}` : ''}
            </p>
            <ul
              className={`divide-y divide-border overflow-hidden rounded-md border border-border bg-card transition-opacity ${isFetching ? 'opacity-60' : ''}`}
            >
              {results.map((t) => (
                <li key={t.id}>
                  <SearchResultRow task={t} project={byId.get(t.project_id)} orgSlug={slug} query={params.q} />
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </div>
  );
}

function FilterSelect({
  label,
  value,
  onChange,
  any,
  options,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  any: string;
  options: Array<{ value: string; label: string }>;
}) {
  return (
    <div className="w-40 max-w-full">
      <Select
        size="sm"
        aria-label={label}
        value={value}
        onValueChange={onChange}
        options={[{ value: '', label: any }, ...options]}
        className={value ? 'border-foreground/50' : undefined}
      />
    </div>
  );
}

function ResultsSkeleton() {
  return (
    <div className="divide-y divide-border rounded-md border border-border bg-card" aria-label="Loading results">
      {[0, 1, 2, 3, 4].map((i) => (
        <div key={i} className="flex items-center gap-3 px-3 py-3">
          <Skeleton className="h-3 w-14" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-6 w-6 rounded-full" />
        </div>
      ))}
    </div>
  );
}
