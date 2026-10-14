'use client';

import { useEffect, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { searchTasks, type TaskSearchParams } from '@/lib/api/tasks';

/** `value`, but only updated after it has been stable for `delay` ms. */
export function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(t);
  }, [value, delay]);
  return debounced;
}

/**
 * Org task search. The params are part of the query key, so a newer query
 * supersedes an older one and a stale response can never overwrite fresh
 * results. The previous results stay on screen while the next ones load.
 */
export function useTaskSearch(orgId: string, params: TaskSearchParams, enabled = true) {
  return useQuery({
    queryKey: ['search', orgId, params],
    queryFn: () => searchTasks(orgId, params),
    enabled,
    placeholderData: keepPreviousData,
    staleTime: 15_000,
  });
}

// ---- Recent items (per org, sessionStorage; best effort) --------------------

export interface RecentItem {
  href: string;
  title: string;
  /** Task key (`ABC-12`) or project key; mono. */
  key: string;
  kind: 'task' | 'project';
}

const MAX_RECENTS = 6;
const recentsKey = (orgId: string) => `fluxboard-recent-search:${orgId}`;

export function readRecents(orgId: string): RecentItem[] {
  try {
    const raw = sessionStorage.getItem(recentsKey(orgId));
    const parsed: unknown = raw ? JSON.parse(raw) : [];
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        (r): r is RecentItem =>
          !!r && typeof r.href === 'string' && typeof r.title === 'string' && typeof r.key === 'string',
      )
      .slice(0, MAX_RECENTS);
  } catch {
    return [];
  }
}

export function pushRecent(orgId: string, item: RecentItem): void {
  try {
    const next = [item, ...readRecents(orgId).filter((r) => r.href !== item.href)].slice(0, MAX_RECENTS);
    sessionStorage.setItem(recentsKey(orgId), JSON.stringify(next));
  } catch {
    // Storage unavailable — recents are a convenience only.
  }
}
