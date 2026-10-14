'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type ReactNode,
} from 'react';
import { createPortal } from 'react-dom';
import { ArrowRight, CornerDownLeft, FolderKanban, History, ListChecks, Plus, Search } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useProjectsMap } from '@/lib/board/use-projects';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { Highlight } from './highlight';
import { SearchResultRow, taskHref } from './result-row';
import { pushRecent, readRecents, useDebouncedValue, useTaskSearch, type RecentItem } from './use-task-search';

interface RowLinkProps {
  tabIndex: number;
  onClick: (e: ReactMouseEvent<HTMLAnchorElement>) => void;
}

interface Entry {
  id: string;
  group: string;
  href: string;
  recent?: RecentItem;
  render: (active: boolean, linkProps: RowLinkProps) => ReactNode;
}

const MAX_TASKS = 8;
const MAX_PROJECTS = 4;
const TASK_KEY_RE = /^([a-z0-9_]+)-(\d+)$/i;

function GenericRow({
  icon,
  children,
  hint,
  active,
  href,
  linkProps,
}: {
  icon: ReactNode;
  children: ReactNode;
  hint?: ReactNode;
  active: boolean;
  href: string;
  linkProps: RowLinkProps;
}) {
  return (
    <Link
      href={href}
      {...linkProps}
      data-active={active ? 'true' : undefined}
      className="flex items-center gap-3 px-3 py-2.5 text-sm outline-none transition-colors hover:bg-secondary data-[active=true]:bg-secondary"
    >
      <span className="flex h-5 w-5 shrink-0 items-center justify-center text-muted-foreground [&>svg]:h-4 [&>svg]:w-4">
        {icon}
      </span>
      <span className="min-w-0 flex-1 truncate">{children}</span>
      {hint ? <span className="shrink-0 text-xs text-muted-foreground">{hint}</span> : null}
    </Link>
  );
}

/** Command-palette search over the org's tasks and projects. Mount it only while open. */
export function SearchPalette({ onClose }: { onClose: () => void }) {
  const { orgId, slug } = useOrg();
  const router = useRouter();
  const uid = useId();
  const listId = `${uid}-list`;
  const dialogRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const [recents] = useState(() => readRecents(orgId));
  const typed = query.trim();
  const debounced = useDebouncedValue(typed, 200);

  const { projects, byId: projectsById } = useProjectsMap();
  const hasQuery = debounced.length > 0;
  const search = useTaskSearch(orgId, { q: debounced, limit: MAX_TASKS }, hasQuery);

  // Take focus on open and hand it back on close; lock page scroll meanwhile.
  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const overflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    inputRef.current?.focus();
    return () => {
      document.body.style.overflow = overflow;
      previous?.focus?.();
    };
  }, []);

  const select = useCallback(
    (entry: Entry, newTab: boolean) => {
      if (entry.recent) pushRecent(orgId, entry.recent);
      if (newTab) {
        window.open(entry.href, '_blank', 'noopener');
        return;
      }
      onClose();
      router.push(entry.href);
    },
    [orgId, onClose, router],
  );

  const entries = useMemo<Entry[]>(() => {
    const out: Entry[] = [];

    if (!typed) {
      for (const r of recents) {
        out.push({
          id: `recent:${r.href}`,
          group: 'Recent',
          href: r.href,
          recent: r,
          render: (a, lp) => (
            <GenericRow icon={<History />} href={r.href} active={a} linkProps={lp} hint={r.kind === 'project' ? 'Project' : undefined}>
              <span className="mr-2 font-mono text-xs text-muted-foreground">{r.key}</span>
              {r.title}
            </GenericRow>
          ),
        });
      }
      const actions: Array<[string, string, ReactNode]> = [
        ['My tasks', `/app/${slug}/my-tasks`, <ListChecks key="a" />],
        ['Projects', `/app/${slug}/projects`, <FolderKanban key="b" />],
        ['Create project', `/app/${slug}/projects/new`, <Plus key="c" />],
      ];
      for (const [label, href, icon] of actions) {
        out.push({
          id: `action:${href}`,
          group: 'Quick actions',
          href,
          render: (a, lp) => (
            <GenericRow icon={icon} href={href} active={a} linkProps={lp}>
              {label}
            </GenericRow>
          ),
        });
      }
      return out;
    }

    // "ABC-12" jumps straight to that task without a server round trip.
    const m = TASK_KEY_RE.exec(typed);
    const keyed = m ? projects.find((p) => p.key.toLowerCase() === m[1]!.toLowerCase()) : undefined;
    if (keyed && m) {
      const href = `/app/${slug}/projects/${keyed.key}/tasks/${m[2]}`;
      const label = `${keyed.key}-${m[2]}`;
      out.push({
        id: `jump:${href}`,
        group: 'Go to',
        href,
        recent: { href, title: keyed.name, key: label, kind: 'task' },
        render: (a, lp) => (
          <GenericRow icon={<ArrowRight />} href={href} active={a} linkProps={lp} hint={keyed.name}>
            Open <span className="font-mono">{label}</span>
          </GenericRow>
        ),
      });
    }

    for (const t of search.data?.tasks ?? []) {
      const project = projectsById.get(t.project_id);
      const href = taskHref(slug, project, t);
      if (!project || !href) continue;
      out.push({
        id: `task:${t.id}`,
        group: 'Tasks',
        href,
        recent: { href, title: t.title, key: `${project.key}-${t.number}`, kind: 'task' },
        render: (a, lp) => <SearchResultRow task={t} project={project} orgSlug={slug} query={typed} active={a} {...lp} />,
      });
    }

    const needle = typed.toLowerCase();
    projects
      .filter((p) => !p.archived_at && (p.name.toLowerCase().includes(needle) || p.key.toLowerCase().includes(needle)))
      .slice(0, MAX_PROJECTS)
      .forEach((p) => {
        const href = `/app/${slug}/projects/${p.key}`;
        out.push({
          id: `project:${p.id}`,
          group: 'Projects',
          href,
          recent: { href, title: p.name, key: p.key, kind: 'project' },
          render: (a, lp) => (
            <GenericRow icon={<FolderKanban />} href={href} active={a} linkProps={lp} hint={p.key}>
              <Highlight text={p.name} query={typed} />
            </GenericRow>
          ),
        });
      });
    return out;
  }, [typed, recents, slug, projects, projectsById, search.data]);

  const current = entries.length ? Math.min(active, entries.length - 1) : -1;
  const activeId = current >= 0 ? `${uid}-opt-${current}` : undefined;

  useEffect(() => {
    if (current < 0) return;
    listRef.current?.querySelector<HTMLElement>(`[data-index="${current}"]`)?.scrollIntoView?.({ block: 'nearest' });
  }, [current]);

  const hasTaskRows = entries.some((e) => e.group === 'Tasks');
  const loading = hasQuery && (typed !== debounced || search.isFetching);
  const showSkeleton = typed.length > 0 && loading && !hasTaskRows;
  const noResults = typed.length > 0 && !loading && !search.isError && entries.length === 0;
  const advancedHref = `/app/${slug}/search${typed ? `?q=${encodeURIComponent(typed)}` : ''}`;

  function onInputKeyDown(e: ReactKeyboardEvent<HTMLInputElement>) {
    if (e.nativeEvent.isComposing) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (!entries.length) return;
      const dir = e.key === 'ArrowDown' ? 1 : -1;
      setActive((current + dir + entries.length) % entries.length);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const entry = entries[current];
      if (entry) select(entry, e.metaKey || e.ctrlKey);
      else if (typed) {
        onClose();
        router.push(advancedHref);
      }
    }
  }

  function onDialogKeyDown(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      onClose();
      return;
    }
    if (e.key !== 'Tab') return;
    // Trap focus: only the input, the close button and the footer link are tabbable.
    const focusable = Array.from(
      dialogRef.current?.querySelectorAll<HTMLElement>('input, a[href]:not([tabindex="-1"]), button:not([tabindex="-1"])') ?? [],
    ).filter((el) => el.offsetParent !== null);
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (!first || !last) return;
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  const groups: Array<{ name: string; items: Array<{ entry: Entry; index: number }> }> = [];
  entries.forEach((entry, index) => {
    let g = groups[groups.length - 1];
    if (!g || g.name !== entry.group) {
      g = { name: entry.group, items: [] };
      groups.push(g);
    }
    g.items.push({ entry, index });
  });

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-stretch justify-center sm:items-start sm:px-4 sm:pt-[15vh]">
      <div className="absolute inset-0 animate-fade-in bg-foreground/40 backdrop-blur-sm" onClick={onClose} aria-hidden />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="Search"
        onKeyDown={onDialogKeyDown}
        className="relative flex h-full w-full animate-scale-in flex-col overflow-hidden bg-card shadow-2xl sm:h-auto sm:max-h-[70vh] sm:max-w-[640px] sm:rounded-xl sm:border sm:border-border"
      >
        <div className="flex items-center gap-3 border-b border-border px-4">
          <Search aria-hidden className="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            onKeyDown={onInputKeyDown}
            role="combobox"
            aria-expanded
            aria-controls={listId}
            aria-activedescendant={activeId}
            aria-autocomplete="list"
            aria-label="Search tasks and projects"
            autoComplete="off"
            spellCheck={false}
            placeholder="Search tasks and projects…"
            className="h-14 min-w-0 flex-1 bg-transparent text-base outline-none placeholder:text-muted-foreground/70"
          />
          <button
            type="button"
            onClick={onClose}
            className="rounded-sm border border-border bg-background px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground hover:text-foreground sm:hidden"
          >
            Esc
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto" aria-busy={loading}>
          <ul ref={listRef} id={listId} role="listbox" aria-label="Results" className="py-1">
            {groups.map((g) => (
              <li key={g.name} role="presentation">
                <p className="eyebrow px-3 pb-1 pt-2.5">{g.name}</p>
                <ul role="presentation">
                  {g.items.map(({ entry, index }) => (
                    <li
                      key={entry.id}
                      id={`${uid}-opt-${index}`}
                      role="option"
                      aria-selected={index === current}
                      data-index={index}
                      onPointerMove={() => index !== current && setActive(index)}
                    >
                      {entry.render(index === current, {
                        tabIndex: -1,
                        onClick: (e) => {
                          if (entry.recent) pushRecent(orgId, entry.recent);
                          // Modified clicks open elsewhere; keep the palette for those.
                          if (!(e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0)) onClose();
                        },
                      })}
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ul>

          {showSkeleton ? (
            <div className="space-y-1 px-3 pb-3" aria-label="Loading results">
              {[0, 1, 2].map((i) => (
                <div key={i} className="flex items-center gap-3 py-2">
                  <Skeleton className="h-3 w-14" />
                  <Skeleton className="h-4 flex-1" />
                  <Skeleton className="h-6 w-6 rounded-full" />
                </div>
              ))}
            </div>
          ) : null}

          {search.isError && hasQuery ? (
            <div role="alert" className="px-4 py-6 text-center text-sm">
              <p className="text-destructive">Search failed.</p>
              <button type="button" onClick={() => search.refetch()} className="mt-1 text-xs underline underline-offset-2">
                Try again
              </button>
            </div>
          ) : null}

          {noResults ? (
            <p className="px-4 py-8 text-center text-sm text-muted-foreground">
              No results for <span className="font-medium text-foreground">&ldquo;{typed}&rdquo;</span>
            </p>
          ) : null}
        </div>

        <div className="flex items-center gap-4 border-t border-border px-4 py-2 text-xs text-muted-foreground">
          <span className="hidden items-center gap-1.5 sm:flex">
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd> navigate
          </span>
          <span className="hidden items-center gap-1.5 sm:flex">
            <Kbd>
              <CornerDownLeft className="h-3 w-3" aria-label="Enter" />
            </Kbd>
            open
          </span>
          <span className="hidden items-center gap-1.5 md:flex">
            <Kbd>Ctrl</Kbd>
            <Kbd>
              <CornerDownLeft className="h-3 w-3" aria-label="Enter" />
            </Kbd>
            new tab
          </span>
          <span className="hidden items-center gap-1.5 sm:flex">
            <Kbd>Esc</Kbd> close
          </span>
          <Link
            href={advancedHref}
            onClick={onClose}
            className={cn(
              'ml-auto inline-flex items-center gap-1 rounded-sm text-foreground underline-offset-2 outline-none',
              'hover:underline focus-visible:underline',
            )}
          >
            Advanced search <ArrowRight className="h-3 w-3" />
          </Link>
        </div>
      </div>
    </div>,
    document.body,
  );
}

function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="inline-flex min-w-5 items-center justify-center rounded-sm border border-border bg-background px-1 py-0.5 font-mono text-[10px]">
      {children}
    </kbd>
  );
}
