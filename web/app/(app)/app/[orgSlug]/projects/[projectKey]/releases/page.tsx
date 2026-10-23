'use client';

import { use, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Copy, FileText, Package, Pencil, Plus, RotateCcw, Rocket, Trash2 } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { getBoard } from '@/lib/api/board';
import {
  createRelease,
  deleteRelease,
  listReleases,
  updateRelease,
  type Release,
  type ReleaseInput,
} from '@/lib/api/releases';
import { doneTasksOf, releaseNotesMarkdown } from '@/lib/board/release-notes';
import { ApiError } from '@/lib/api/client';
import { cn } from '@/lib/utils';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useToast } from '@/components/ui/toast';

const inputClass =
  'w-full rounded-lg border border-input bg-card px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

function formatDay(day: string): string {
  return new Date(`${day.slice(0, 10)}T00:00:00Z`).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    timeZone: 'UTC',
  });
}

function statusOf(r: Release): { label: string; className: string } {
  if (r.released_at) return { label: `Released ${formatDay(r.released_at)}`, className: 'bg-success/15 text-success' };
  if (r.release_date && r.release_date < new Date().toISOString().slice(0, 10))
    return { label: `Overdue · ${formatDay(r.release_date)}`, className: 'bg-warning/20 text-foreground' };
  if (r.release_date) return { label: `Planned ${formatDay(r.release_date)}`, className: 'bg-cobalt/12 text-cobalt' };
  return { label: 'Unreleased', className: 'bg-secondary text-muted-foreground' };
}

// The create / edit form: name, planned date, description.
function ReleaseForm({
  initial,
  pending,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  initial?: Release;
  pending: boolean;
  submitLabel: string;
  onSubmit: (input: ReleaseInput) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? '');
  const [day, setDay] = useState(initial?.release_date ?? '');
  const [description, setDescription] = useState(initial?.description ?? '');
  const submit = () =>
    name.trim() &&
    onSubmit({ name: name.trim(), description: description.trim(), release_date: day || null, released: Boolean(initial?.released_at) });
  return (
    <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
      <label className="text-xs font-medium text-muted-foreground">
        Name
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') submit();
          }}
          maxLength={100}
          placeholder="e.g. v1.4"
          className={`mt-1 ${inputClass}`}
        />
      </label>
      <label className="text-xs font-medium text-muted-foreground">
        Planned date
        <input type="date" value={day} onChange={(e) => setDay(e.target.value)} className={`mt-1 ${inputClass}`} />
      </label>
      <label className="text-xs font-medium text-muted-foreground sm:col-span-2">
        Description
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          maxLength={2000}
          rows={2}
          placeholder="What this release is about (optional)"
          className={`mt-1 ${inputClass}`}
        />
      </label>
      <div className="flex gap-2 sm:col-span-2">
        <Button size="sm" disabled={!name.trim() || pending} onClick={submit}>
          {pending ? 'Saving…' : submitLabel}
        </Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

// Releases (FR-PROJ-010): plan what ships together, follow its progress and
// copy release notes from its done tasks. LEADs manage releases; tasks are
// filed under one from the task detail.
export default function ProjectReleasesPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId } = useOrg();
  const qc = useQueryClient();
  const { toast } = useToast();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);

  const releasesKey = ['releases', project?.id] as const;
  const releasesQ = useQuery({
    queryKey: releasesKey,
    queryFn: () => listReleases(orgId, project!.id),
    enabled: Boolean(project),
  });
  const boardQ = useQuery({
    queryKey: ['board', project?.id],
    queryFn: () => getBoard(orgId, project!.id),
    enabled: Boolean(project),
  });
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: releasesKey });
    qc.invalidateQueries({ queryKey: ['board', project?.id] });
  };
  const onError = (err: unknown, fallback: string) =>
    toast({
      title: err instanceof ApiError && err.status === 403 ? 'Only project leads can manage releases' : fallback,
      variant: 'error',
    });

  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [notesOpen, setNotesOpen] = useState<Set<string>>(new Set());

  const createMutation = useMutation({
    mutationFn: (input: ReleaseInput) => createRelease(orgId, project!.id, input),
    onSuccess: () => {
      invalidate();
      setShowForm(false);
    },
    onError: (e) => onError(e, 'Could not create the release'),
  });
  const updateMutation = useMutation({
    mutationFn: (v: { id: string; input: ReleaseInput }) => updateRelease(orgId, project!.id, v.id, v.input),
    onSuccess: () => {
      invalidate();
      setEditingId(null);
    },
    onError: (e) => onError(e, 'Could not update the release'),
  });
  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteRelease(orgId, project!.id, id),
    onSuccess: invalidate,
    onError: (e) => onError(e, 'Could not delete the release'),
  });

  async function copyNotes(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      toast({ title: 'Release notes copied' });
    } catch {
      toast({ title: 'Couldn’t copy — select the text instead', variant: 'error' });
    }
  }

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading releases…</p>;
  if (isError || !project)
    return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;

  const releases = releasesQ.data ?? [];

  return (
    <ProjectFrame project={project}>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <p className="min-w-0 flex-1 text-sm text-muted-foreground">
          Plan what ships together, follow its progress and copy release notes from its done tasks. File a
          task under a release from its detail panel.
        </p>
        <Button size="sm" className="shrink-0" onClick={() => setShowForm((v) => !v)}>
          <Plus className="h-4 w-4" /> New release
        </Button>
      </div>

      {showForm ? (
        <Card className="mb-6">
          <CardContent className="py-4">
            <ReleaseForm
              pending={createMutation.isPending}
              submitLabel="Create release"
              onSubmit={(input) => createMutation.mutate(input)}
              onCancel={() => setShowForm(false)}
            />
          </CardContent>
        </Card>
      ) : null}

      {releases.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed py-12 text-center">
          <Package className="h-6 w-6 text-muted-foreground" aria-hidden />
          <p className="text-sm font-medium">No releases yet</p>
          <p className="text-xs text-muted-foreground">Create one to plan what ships together.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {releases.map((r) => {
            const pct = r.total ? Math.round((r.done / r.total) * 100) : 0;
            const status = statusOf(r);
            const showNotes = notesOpen.has(r.id);
            const notes =
              showNotes && boardQ.data ? releaseNotesMarkdown(r, doneTasksOf(boardQ.data, r.id), project.key) : '';
            return (
              <Card key={r.id}>
                <CardContent className="space-y-3 py-4">
                  {editingId === r.id ? (
                    <ReleaseForm
                      initial={r}
                      pending={updateMutation.isPending}
                      submitLabel="Save"
                      onSubmit={(input) => updateMutation.mutate({ id: r.id, input })}
                      onCancel={() => setEditingId(null)}
                    />
                  ) : (
                    <div className="flex flex-wrap items-start gap-3">
                      <div className="min-w-0 flex-1 basis-56">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="truncate text-sm font-semibold">{r.name}</p>
                          <span className={cn('whitespace-nowrap rounded-full px-2 py-0.5 text-[11px] font-medium', status.className)}>
                            {status.label}
                          </span>
                        </div>
                        {r.description ? (
                          <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{r.description}</p>
                        ) : null}
                      </div>
                      <div className="flex items-center gap-1">
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={updateMutation.isPending}
                          onClick={() =>
                            updateMutation.mutate({
                              id: r.id,
                              input: {
                                name: r.name,
                                description: r.description,
                                release_date: r.release_date,
                                released: !r.released_at,
                              },
                            })
                          }
                        >
                          {r.released_at ? (
                            <>
                              <RotateCcw className="h-3.5 w-3.5" /> Reopen
                            </>
                          ) : (
                            <>
                              <Rocket className="h-3.5 w-3.5" /> Mark released
                            </>
                          )}
                        </Button>
                        <button
                          type="button"
                          onClick={() => setEditingId(r.id)}
                          aria-label={`Edit ${r.name}`}
                          className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            if (window.confirm(`Delete "${r.name}"? Its tasks stay, without a release.`))
                              deleteMutation.mutate(r.id);
                          }}
                          aria-label={`Delete ${r.name}`}
                          className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-destructive"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  )}

                  <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                    <div
                      className="h-1.5 min-w-32 flex-1 overflow-hidden rounded-full bg-secondary"
                      role="progressbar"
                      aria-valuenow={pct}
                      aria-valuemin={0}
                      aria-valuemax={100}
                      aria-label={`${r.name} progress`}
                    >
                      <div className="h-full rounded-full bg-success transition-[width] duration-500" style={{ width: `${pct}%` }} />
                    </div>
                    <span className="font-mono text-[11px] text-muted-foreground">
                      {r.done}/{r.total} done
                      {r.points > 0 ? ` · ${r.done_points}/${r.points} pts` : ''}
                    </span>
                    <Button
                      size="sm"
                      variant="ghost"
                      aria-expanded={showNotes}
                      onClick={() =>
                        setNotesOpen((prev) => {
                          const next = new Set(prev);
                          if (next.has(r.id)) next.delete(r.id);
                          else next.add(r.id);
                          return next;
                        })
                      }
                    >
                      <FileText className="h-3.5 w-3.5" /> Release notes
                    </Button>
                  </div>

                  {showNotes ? (
                    <div className="rounded-md border bg-secondary/40">
                      <div className="flex items-center justify-between border-b px-3 py-1.5">
                        <span className="text-[11px] font-medium text-muted-foreground">
                          Markdown · {r.total - r.done} unfinished {r.total - r.done === 1 ? 'task is' : 'tasks are'} left out
                        </span>
                        <Button size="sm" variant="ghost" disabled={!notes} onClick={() => copyNotes(notes)}>
                          <Copy className="h-3.5 w-3.5" /> Copy
                        </Button>
                      </div>
                      <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words px-3 py-2 font-mono text-xs">
                        {boardQ.isLoading ? 'Loading…' : notes}
                      </pre>
                    </div>
                  ) : null}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </ProjectFrame>
  );
}
