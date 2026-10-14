'use client';

import { use, useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowDown, ArrowUp, Building2, Lock, MoreHorizontal, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Menu, MenuItem } from '@/components/ui/menu';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { useToast } from '@/components/ui/toast';
import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { archiveProject, unarchiveProject, updateProject } from '@/lib/api/projects';
import {
  createColumn,
  deleteColumn,
  getBoard,
  renameColumn,
  reorderColumn,
  setColumnRole,
} from '@/lib/api/board';
import { between } from '@/lib/board/rank';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { FieldsManager } from '@/components/board/fields-manager';
import { FormsManager } from '@/components/board/forms-manager';
import { TransferSection } from '@/components/board/transfer-section';
import {
  ConfirmStrip,
  EmptyState,
  RowList,
  SettingsSection,
} from '@/components/board/settings-section';
import type { BoardColumn, ColumnRole, Project, Visibility } from '@/lib/api/types';
import { cn } from '@/lib/utils';

export default function ProjectSettingsPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { slug, isAdmin } = useOrg();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading project…</p>;
  if (isError || !project)
    return (
      <div className="mx-auto max-w-md px-6 py-16 text-center">
        <h1 className="text-lg font-semibold">Project not found</h1>
        <Link
          href={`/app/${slug}/projects`}
          className="mt-4 inline-block text-sm text-primary hover:underline"
        >
          Back to projects
        </Link>
      </div>
    );

  return (
    <ProjectFrame project={project} narrow>
      <div className="space-y-5">
        {isAdmin ? (
          <>
            <GeneralSection project={project} />
            <ColumnsSection project={project} />
            <FieldsManager projectId={project.id} />
            <FormsManager projectId={project.id} />
          </>
        ) : (
          <p className="rounded-md border border-border bg-secondary/40 p-5 text-sm text-muted-foreground">
            Only project leads and organization admins can change project settings.
          </p>
        )}
        {/* Project-role gated server-side: VIEWER exports, CONTRIBUTOR imports. */}
        <TransferSection project={project} />
        {isAdmin ? <DangerSection project={project} /> : null}
      </div>
    </ProjectFrame>
  );
}

const VISIBILITY: { v: Visibility; label: string; hint: string; icon: React.ReactNode }[] = [
  { v: 'org', label: 'Organization', hint: 'All org members can see it.', icon: <Building2 /> },
  { v: 'private', label: 'Private', hint: 'Only project members.', icon: <Lock /> },
];

function GeneralSection({ project }: { project: Project }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description);
  const [color, setColor] = useState(project.color);
  const [visibility, setVisibility] = useState<Visibility>(project.visibility);
  const archived = Boolean(project.archived_at);

  const dirty =
    name !== project.name ||
    description !== project.description ||
    color !== project.color ||
    visibility !== project.visibility;

  const mutation = useMutation({
    mutationFn: () => updateProject(orgId, project.id, { name, description, color, visibility }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-by-key', orgId, project.key] });
      queryClient.invalidateQueries({ queryKey: ['projects', orgId] });
      toast({ title: 'Project updated', variant: 'success' });
    },
    onError: () => toast({ title: 'Update failed', variant: 'error' }),
  });

  function discard() {
    setName(project.name);
    setDescription(project.description);
    setColor(project.color);
    setVisibility(project.visibility);
  }

  return (
    <SettingsSection title="General" description="Name, description and who can see this project.">
      {archived ? (
        <p className="mb-4 rounded-md border border-warning/40 bg-warning/15 p-3 text-[13px]">
          This project is archived and read-only. Restore it below to edit.
        </p>
      ) : null}
      <form
        className="space-y-5"
        onSubmit={(e) => {
          e.preventDefault();
          if (dirty && name.trim()) mutation.mutate();
        }}
      >
        <Field label="Name">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={archived}
            required
          />
        </Field>
        <Field label="Description" optional>
          <Textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            disabled={archived}
            rows={3}
            autoResize
          />
        </Field>
        <Field label="Color">
          <div className="flex items-center gap-3">
            <input
              type="color"
              value={color || '#6366f1'}
              onChange={(e) => setColor(e.target.value)}
              disabled={archived}
              className="h-9 w-12 cursor-pointer rounded-md border border-input bg-card p-1 disabled:cursor-not-allowed disabled:opacity-50"
            />
            <span className="font-mono text-xs uppercase text-muted-foreground">
              {color || '#6366f1'}
            </span>
          </div>
        </Field>
        <div className="space-y-1.5">
          <span id="visibility-label" className="text-[13px] font-medium">
            Visibility
          </span>
          <div
            role="radiogroup"
            aria-labelledby="visibility-label"
            className="grid gap-2 sm:grid-cols-2"
          >
            {VISIBILITY.map(({ v, label, hint, icon }) => (
              <button
                key={v}
                type="button"
                role="radio"
                aria-checked={visibility === v}
                disabled={archived}
                onClick={() => setVisibility(v)}
                className={cn(
                  'flex items-start gap-3 rounded-md border p-3 text-left text-sm transition-colors disabled:opacity-50',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal focus-visible:ring-offset-2 focus-visible:ring-offset-background',
                  visibility === v
                    ? 'border-foreground bg-secondary/60'
                    : 'border-input bg-card hover:border-foreground/40',
                )}
              >
                <span className="mt-0.5 text-muted-foreground [&>svg]:h-4 [&>svg]:w-4">{icon}</span>
                <span>
                  <span className="block font-medium">{label}</span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">{hint}</span>
                </span>
              </button>
            ))}
          </div>
        </div>

        <div
          className={cn(
            'sticky bottom-4 z-10 flex items-center gap-3 rounded-md border px-3 py-2 transition-colors',
            dirty ? 'border-foreground/30 bg-card shadow-lg' : 'border-transparent',
          )}
        >
          <span className="flex-1 text-[13px] text-muted-foreground" aria-live="polite">
            {dirty ? 'You have unsaved changes.' : 'All changes saved.'}
          </span>
          {dirty ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={discard}
              disabled={mutation.isPending}
            >
              Discard
            </Button>
          ) : null}
          <Button
            type="submit"
            size="sm"
            disabled={archived || !dirty || !name.trim()}
            isLoading={mutation.isPending}
          >
            Save changes
          </Button>
        </div>
      </form>
    </SettingsSection>
  );
}

function ColumnsSection({ project }: { project: Project }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const boardKey = ['board', project.id] as const;

  const board = useQuery({ queryKey: boardKey, queryFn: () => getBoard(orgId, project.id) });
  const columns = board.data?.columns ?? [];

  const [newName, setNewName] = useState('');

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: boardKey });
  }

  const add = useMutation({
    mutationFn: () => createColumn(orgId, project.id, { name: newName.trim() }),
    onSuccess: () => {
      setNewName('');
      invalidate();
    },
    onError: () => toast({ title: 'Couldn’t add column', variant: 'error' }),
  });

  const rename = useMutation({
    mutationFn: (v: { id: string; name: string; wip_limit: number | null }) =>
      renameColumn(orgId, project.id, v.id, { name: v.name, wip_limit: v.wip_limit }),
    onSuccess: invalidate,
    onError: () => toast({ title: 'Couldn’t update column', variant: 'error' }),
  });

  const reorder = useMutation({
    mutationFn: (v: { id: string; rank: string }) => reorderColumn(orgId, project.id, v.id, v.rank),
    onSuccess: invalidate,
    onError: () => toast({ title: 'Couldn’t reorder column', variant: 'error' }),
  });

  const remove = useMutation({
    mutationFn: (v: { id: string; target: string }) =>
      deleteColumn(orgId, project.id, v.id, v.target),
    onSuccess: invalidate,
    onError: () => toast({ title: 'Couldn’t delete column', variant: 'error' }),
  });

  const setRole = useMutation({
    mutationFn: (v: { id: string; role: ColumnRole | null }) =>
      setColumnRole(orgId, project.id, v.id, v.role),
    onSuccess: invalidate,
    onError: () => toast({ title: 'Couldn’t change the column’s stage', variant: 'error' }),
  });

  function move(i: number, dir: -1 | 1) {
    const j = i + dir;
    if (j < 0 || j >= columns.length) return;
    const col = columns[i] as BoardColumn;
    // Compute a rank that lands the column on the other side of its neighbour.
    let rank: string;
    try {
      if (dir === -1) {
        const before = columns[j - 1]?.rank ?? '';
        rank = between(before, (columns[j] as BoardColumn).rank);
      } else {
        const after = columns[j + 1]?.rank ?? '';
        rank = between((columns[j] as BoardColumn).rank, after);
      }
    } catch {
      return;
    }
    reorder.mutate({ id: col.id, rank });
  }

  const busy = rename.isPending || reorder.isPending || remove.isPending || setRole.isPending;

  return (
    <SettingsSection
      title="Columns"
      description="Each column can stand for a workflow stage. Agents move work by stage and can take it as far as Review; only a person moves it to Done. Stages left unset are inferred from column names."
    >
      <div className="space-y-4">
        {board.isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : columns.length === 0 ? (
          <EmptyState title="No columns yet">
            Add a first column to start shaping the workflow.
          </EmptyState>
        ) : (
          <RowList>
            {columns.map((c, i) => (
              <ColumnRow
                // Remount on a saved change so the drafts start from the new values.
                key={`${c.id}:${c.name}:${c.wip_limit ?? ''}`}
                column={c}
                index={i}
                total={columns.length}
                others={columns.filter((o) => o.id !== c.id)}
                onRename={(name, wip) => rename.mutate({ id: c.id, name, wip_limit: wip })}
                onMove={(dir) => move(i, dir)}
                onDelete={(target) => remove.mutate({ id: c.id, target })}
                onRole={(role) => setRole.mutate({ id: c.id, role })}
                busy={busy}
              />
            ))}
          </RowList>
        )}

        <form
          className="flex flex-wrap items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (newName.trim()) add.mutate();
          }}
        >
          <Input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="New column name"
            aria-label="New column name"
            className="h-9 min-w-0 flex-1 sm:max-w-xs"
          />
          <Button
            type="submit"
            size="sm"
            variant="outline"
            disabled={add.isPending || !newName.trim()}
          >
            Add column
          </Button>
        </form>
      </div>
    </SettingsSection>
  );
}

const STAGES: { value: ColumnRole; label: string }[] = [
  { value: 'backlog', label: 'Backlog' },
  { value: 'todo', label: 'To do' },
  { value: 'in_progress', label: 'In progress' },
  { value: 'review', label: 'Review' },
  { value: 'done', label: 'Done' },
];

function ColumnRow({
  column,
  index,
  total,
  others,
  onRename,
  onMove,
  onDelete,
  onRole,
  busy,
}: {
  column: BoardColumn;
  index: number;
  total: number;
  others: BoardColumn[];
  onRename: (name: string, wip: number | null) => void;
  onMove: (dir: -1 | 1) => void;
  onDelete: (target: string) => void;
  onRole: (role: ColumnRole | null) => void;
  busy: boolean;
}) {
  const [name, setName] = useState(column.name);
  const [wip, setWip] = useState<string>(column.wip_limit != null ? String(column.wip_limit) : '');
  const [confirming, setConfirming] = useState(false);
  const [target, setTarget] = useState(others[0]?.id ?? '');

  function commit() {
    const parsedWip = wip.trim() === '' ? null : Number.parseInt(wip, 10);
    const wipVal =
      parsedWip != null && Number.isFinite(parsedWip) && parsedWip > 0 ? parsedWip : null;
    if (name.trim() && (name.trim() !== column.name || wipVal !== (column.wip_limit ?? null))) {
      onRename(name.trim(), wipVal);
    }
  }

  return (
    <li>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 px-3 py-2.5">
        <span
          className="w-5 shrink-0 text-center font-mono text-xs text-muted-foreground"
          aria-hidden
        >
          {index + 1}
        </span>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
          aria-label={`Name of column ${index + 1}`}
          className="h-8 min-w-0 flex-1 basis-40 text-[13px]"
        />
        <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
          WIP
          <Input
            type="number"
            min={0}
            value={wip}
            onChange={(e) => setWip(e.target.value)}
            onBlur={commit}
            onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
            placeholder="∞"
            className="h-8 w-16 text-[13px]"
          />
        </label>
        <Select
          size="sm"
          className="w-36"
          value={column.role ?? ''}
          onValueChange={(v) => onRole((v || null) as ColumnRole | null)}
          disabled={busy}
          aria-label={`Stage of ${column.name}`}
          options={[{ value: '', label: 'No stage' }, ...STAGES]}
        />
        <Menu
          align="end"
          aria-label={`Actions for ${column.name}`}
          trigger={
            <Button
              size="icon"
              variant="ghost"
              className="h-8 w-8"
              aria-label={`Actions for ${column.name}`}
            >
              <MoreHorizontal className="h-4 w-4" />
            </Button>
          }
        >
          <MenuItem icon={<ArrowUp />} disabled={index === 0 || busy} onSelect={() => onMove(-1)}>
            Move up
          </MenuItem>
          <MenuItem
            icon={<ArrowDown />}
            disabled={index === total - 1 || busy}
            onSelect={() => onMove(1)}
          >
            Move down
          </MenuItem>
          <MenuItem
            destructive
            icon={<Trash2 />}
            disabled={others.length === 0 || busy}
            onSelect={() => setConfirming(true)}
          >
            Delete column
          </MenuItem>
        </Menu>
      </div>

      {confirming ? (
        <ConfirmStrip
          confirmLabel="Delete column"
          disabled={!target}
          onConfirm={() => {
            onDelete(target);
            setConfirming(false);
          }}
          onCancel={() => setConfirming(false)}
        >
          <span>Move its tasks to</span>
          <Select
            size="sm"
            className="w-44"
            aria-label="Move tasks to"
            value={target}
            onValueChange={setTarget}
            options={others.map((o) => ({ value: o.id, label: o.name }))}
          />
        </ConfirmStrip>
      ) : null}
    </li>
  );
}

function DangerSection({ project }: { project: Project }) {
  const { orgId, slug } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const archived = Boolean(project.archived_at);
  const [confirming, setConfirming] = useState(false);

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ['project-by-key', orgId, project.key] });
    queryClient.invalidateQueries({ queryKey: ['projects', orgId] });
  }

  const archive = useMutation({
    mutationFn: () => archiveProject(orgId, project.id),
    onSuccess: () => {
      setConfirming(false);
      invalidate();
      toast({ title: 'Project archived', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t archive', variant: 'error' }),
  });

  const unarchive = useMutation({
    mutationFn: () => unarchiveProject(orgId, project.id),
    onSuccess: () => {
      invalidate();
      toast({ title: 'Project restored', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t restore', variant: 'error' }),
  });

  return (
    <SettingsSection
      danger
      title="Danger zone"
      description="Actions here change who can use the project. They can be undone."
    >
      <div className="overflow-hidden rounded-md border border-destructive/30">
        <div className="flex flex-wrap items-center justify-between gap-3 p-4">
          <div className="min-w-0">
            <p className="text-sm font-medium">
              {archived ? 'Restore project' : 'Archive project'}
            </p>
            <p className="mt-0.5 text-[13px] text-muted-foreground">
              {archived
                ? 'Make this project active and editable again.'
                : 'Hide it from the active list and make it read-only. You can restore it later.'}
            </p>
          </div>
          {archived ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => unarchive.mutate()}
              isLoading={unarchive.isPending}
            >
              Restore
            </Button>
          ) : (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => setConfirming(true)}
              disabled={confirming}
            >
              Archive
            </Button>
          )}
        </div>
        {confirming && !archived ? (
          <ConfirmStrip
            confirmLabel="Archive project"
            disabled={archive.isPending}
            onConfirm={() => archive.mutate()}
            onCancel={() => setConfirming(false)}
          >
            <span>
              Archive <strong className="font-medium">{project.name}</strong>? It becomes read-only
              for everyone.
            </span>
          </ConfirmStrip>
        ) : null}
      </div>
      <p className="mt-4 text-xs text-muted-foreground">
        <Link href={`/app/${slug}/projects`} className="hover:underline">
          Back to projects
        </Link>
      </p>
    </SettingsSection>
  );
}
