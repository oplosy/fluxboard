'use client';

import { use, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Layers, Pencil, Plus, Trash2 } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { createEpic, deleteEpic, listEpics, updateEpic, type Epic } from '@/lib/api/epics';
import { ApiError } from '@/lib/api/client';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useToast } from '@/components/ui/toast';

const inputClass =
  'w-full rounded-lg border border-input bg-card px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

// Epics (FR-PROJ-008): group related tasks and follow their progress. LEADs
// manage epics; tasks are filed under one from the task detail.
export default function ProjectEpicsPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId } = useOrg();
  const qc = useQueryClient();
  const { toast } = useToast();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);

  const epicsKey = ['epics', project?.id] as const;
  const epicsQ = useQuery({
    queryKey: epicsKey,
    queryFn: () => listEpics(orgId, project!.id),
    enabled: Boolean(project),
  });
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: epicsKey });
    qc.invalidateQueries({ queryKey: ['board', project?.id] });
  };
  const onError = (err: unknown, fallback: string) =>
    toast({
      title: err instanceof ApiError && err.status === 403 ? 'Only project leads can manage epics' : fallback,
      variant: 'error',
    });

  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [color, setColor] = useState('#3b82f6');
  const [editing, setEditing] = useState<{ id: string; name: string } | null>(null);

  const createMutation = useMutation({
    mutationFn: () => createEpic(orgId, project!.id, { name: name.trim(), color }),
    onSuccess: () => {
      invalidate();
      setShowForm(false);
      setName('');
    },
    onError: (e) => onError(e, 'Could not create the epic'),
  });
  const renameMutation = useMutation({
    mutationFn: (v: { epic: Epic; name: string }) =>
      updateEpic(orgId, project!.id, v.epic.id, { name: v.name, color: v.epic.color }),
    onSuccess: () => {
      invalidate();
      setEditing(null);
    },
    onError: (e) => onError(e, 'Could not rename the epic'),
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteEpic(orgId, project!.id, id),
    onSuccess: invalidate,
    onError: (e) => onError(e, 'Could not delete the epic'),
  });

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading epics…</p>;
  if (isError || !project)
    return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;

  const epics = epicsQ.data ?? [];

  return (
    <ProjectFrame project={project}>
      <div className="mb-6 flex items-center justify-between gap-4">
        <p className="text-sm text-muted-foreground">
          Group related tasks under an epic and follow how much of it is done. File a task under an
          epic from its detail panel.
        </p>
        <Button size="sm" className="shrink-0" onClick={() => setShowForm((v) => !v)}>
          <Plus className="h-4 w-4" /> New epic
        </Button>
      </div>

      {showForm ? (
        <Card className="mb-6">
          <CardContent className="flex flex-wrap items-end gap-3 py-4">
            <label className="min-w-48 flex-1 text-xs font-medium text-muted-foreground">
              Name
              <input
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && name.trim()) createMutation.mutate();
                }}
                maxLength={100}
                placeholder="e.g. Checkout revamp"
                className={`mt-1 ${inputClass}`}
              />
            </label>
            <label className="text-xs font-medium text-muted-foreground">
              Color
              <input
                type="color"
                value={color}
                onChange={(e) => setColor(e.target.value)}
                className="mt-1 block h-9 w-12 cursor-pointer rounded-lg border border-input bg-card p-1"
              />
            </label>
            <Button size="sm" disabled={!name.trim() || createMutation.isPending} onClick={() => createMutation.mutate()}>
              {createMutation.isPending ? 'Creating…' : 'Create epic'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setShowForm(false)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}

      {epics.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed py-12 text-center">
          <Layers className="h-6 w-6 text-muted-foreground" aria-hidden />
          <p className="text-sm font-medium">No epics yet</p>
          <p className="text-xs text-muted-foreground">Create one to group related tasks.</p>
        </div>
      ) : (
        <ul className="divide-y rounded-lg border bg-card">
          {epics.map((e) => {
            const pct = e.total ? Math.round((e.done / e.total) * 100) : 0;
            return (
              <li key={e.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
                <span
                  className="h-2.5 w-2.5 shrink-0 rounded-full"
                  style={{ backgroundColor: e.color || 'hsl(var(--muted-foreground))' }}
                  aria-hidden
                />
                <div className="min-w-0 flex-1">
                  {editing?.id === e.id ? (
                    <input
                      autoFocus
                      value={editing.name}
                      onChange={(ev) => setEditing({ id: e.id, name: ev.target.value })}
                      onKeyDown={(ev) => {
                        if (ev.key === 'Enter' && editing.name.trim()) renameMutation.mutate({ epic: e, name: editing.name });
                        if (ev.key === 'Escape') setEditing(null);
                      }}
                      onBlur={() => setEditing(null)}
                      maxLength={100}
                      aria-label={`Rename ${e.name}`}
                      className={inputClass}
                    />
                  ) : (
                    <p className="truncate text-sm font-medium">{e.name}</p>
                  )}
                </div>
                <div className="flex w-full items-center gap-2 sm:w-56">
                  <div
                    className="h-1.5 flex-1 overflow-hidden rounded-full bg-secondary"
                    role="progressbar"
                    aria-valuenow={pct}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-label={`${e.name} progress`}
                  >
                    <div
                      className="h-full rounded-full bg-success transition-[width] duration-500"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className="w-20 shrink-0 text-right font-mono text-[11px] text-muted-foreground">
                    {e.done}/{e.total} done
                  </span>
                  {e.points > 0 ? (
                    <span
                      className="w-20 shrink-0 text-right font-mono text-[11px] text-muted-foreground"
                      title="Story points done / total"
                    >
                      {e.done_points}/{e.points} pts
                    </span>
                  ) : null}
                </div>
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setEditing({ id: e.id, name: e.name })}
                    aria-label={`Rename ${e.name}`}
                    className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      if (window.confirm(`Delete "${e.name}"? Its tasks stay, without an epic.`)) deleteMutation.mutate(e.id);
                    }}
                    aria-label={`Delete ${e.name}`}
                    className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-destructive"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </ProjectFrame>
  );
}
