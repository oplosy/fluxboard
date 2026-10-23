'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bookmark, ChevronDown, Save, Trash2, Users } from 'lucide-react';

import { createView, deleteView, listViews, updateView, type SavedView } from '@/lib/api/views';
import { ApiError } from '@/lib/api/client';
import { fromViewFilters, sameView, toViewFilters, type BoardViewState } from '@/lib/board/saved-views';
import { cn } from '@/lib/utils';
import { Menu, MenuItem, MenuLabel, MenuSeparator } from '@/components/ui/menu';
import { useToast } from '@/components/ui/toast';

// Saved board views (FR-PROJ-011): apply a stored filter + grouping in one
// click, save the current one, update or delete the active view. Sharing and
// changing shared views is LEAD-only, enforced by the server.
export function SavedViewsMenu({
  orgId,
  projectId,
  state,
  onApply,
  activeId,
  onActiveChange,
}: {
  orgId: string;
  projectId: string;
  state: BoardViewState;
  onApply: (s: BoardViewState) => void;
  activeId: string | null;
  onActiveChange: (id: string | null) => void;
}) {
  const qc = useQueryClient();
  const { toast } = useToast();
  const viewsKey = ['views', projectId] as const;
  const { data: views = [] } = useQuery({ queryKey: viewsKey, queryFn: () => listViews(orgId, projectId) });
  const active = views.find((v) => v.id === activeId);
  const modified = active ? !sameView(state, active.filters) : false;

  const [saving, setSaving] = useState(false);
  const [name, setName] = useState('');
  const [shared, setShared] = useState(false);

  const onError = (err: unknown, fallback: string) =>
    toast({
      title: err instanceof ApiError && err.status === 403 ? 'Only project leads can share or change shared views' : fallback,
      variant: 'error',
    });

  const create = useMutation({
    mutationFn: () => createView(orgId, projectId, { name: name.trim(), shared, filters: toViewFilters(state) }),
    onSuccess: (v) => {
      qc.invalidateQueries({ queryKey: viewsKey });
      onActiveChange(v.id);
      setSaving(false);
      setName('');
      setShared(false);
    },
    onError: (e) => onError(e, 'Could not save the view'),
  });
  const update = useMutation({
    mutationFn: (v: SavedView) => updateView(orgId, projectId, v.id, { name: v.name, shared: v.shared, filters: toViewFilters(state) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: viewsKey }),
    onError: (e) => onError(e, 'Could not update the view'),
  });
  const remove = useMutation({
    mutationFn: (v: SavedView) => deleteView(orgId, projectId, v.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: viewsKey });
      onActiveChange(null);
    },
    onError: (e) => onError(e, 'Could not delete the view'),
  });

  const sharedViews = views.filter((v) => v.shared);
  const personalViews = views.filter((v) => !v.shared);
  const item = (v: SavedView) => (
    <MenuItem
      key={v.id}
      icon={v.shared ? <Users /> : <Bookmark />}
      onSelect={() => {
        onApply(fromViewFilters(v.filters));
        onActiveChange(v.id);
      }}
    >
      <span className={cn('truncate', v.id === activeId && 'font-semibold')}>{v.name}</span>
    </MenuItem>
  );

  return (
    <>
      <Menu
        aria-label="Saved views"
        trigger={
          <button
            type="button"
            aria-label={active ? `Saved views: ${active.name}${modified ? ' (modified)' : ''}` : 'Saved views'}
            className={cn(
              'inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-[13px] outline-none transition-colors duration-200 focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]',
              active
                ? 'border-foreground/50 bg-secondary text-foreground'
                : 'border-dashed border-input text-muted-foreground hover:border-foreground/40 hover:text-foreground',
            )}
          >
            <Bookmark className="h-3.5 w-3.5" aria-hidden />
            <span className="max-w-[10rem] truncate">{active ? active.name : 'Views'}</span>
            {modified ? <span className="h-1.5 w-1.5 rounded-full bg-signal" aria-hidden /> : null}
            <ChevronDown className="h-3.5 w-3.5 opacity-60" aria-hidden />
          </button>
        }
      >
        {sharedViews.length > 0 ? <MenuLabel>Shared with the project</MenuLabel> : null}
        {sharedViews.map(item)}
        {personalViews.length > 0 ? <MenuLabel>My views</MenuLabel> : null}
        {personalViews.map(item)}
        {views.length > 0 ? <MenuSeparator /> : null}
        <MenuItem icon={<Save />} onSelect={() => setSaving(true)}>
          Save current as view…
        </MenuItem>
        {active && modified ? (
          <MenuItem icon={<Save />} onSelect={() => update.mutate(active)}>
            Update “{active.name}”
          </MenuItem>
        ) : null}
        {active ? (
          <MenuItem
            icon={<Trash2 />}
            destructive
            onSelect={() => {
              if (window.confirm(`Delete the view "${active.name}"?`)) remove.mutate(active);
            }}
          >
            Delete “{active.name}”
          </MenuItem>
        ) : null}
      </Menu>

      {saving ? (
        <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && name.trim()) create.mutate();
              if (e.key === 'Escape') setSaving(false);
            }}
            maxLength={60}
            placeholder="View name"
            aria-label="View name"
            className="h-8 min-w-0 flex-1 rounded-md border border-input bg-card px-2.5 text-[13px] outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] sm:w-44 sm:flex-none"
          />
          <label className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
            <input type="checkbox" checked={shared} onChange={(e) => setShared(e.target.checked)} />
            Share with project
          </label>
          <button
            type="button"
            onClick={() => create.mutate()}
            disabled={!name.trim() || create.isPending}
            className="h-8 rounded-sm bg-foreground px-3 text-xs font-medium text-background transition-[background-color,transform] hover:bg-signal hover:text-signal-foreground active:scale-95 disabled:opacity-50"
          >
            {create.isPending ? 'Saving…' : 'Save'}
          </button>
          <button type="button" onClick={() => setSaving(false)} className="text-xs text-muted-foreground hover:text-foreground">
            Cancel
          </button>
        </div>
      ) : null}
    </>
  );
}
