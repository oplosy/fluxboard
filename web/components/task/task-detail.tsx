'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, Trash2 } from 'lucide-react';

import { conflictCurrent, getTask, listTaskLabels, attachLabel, detachLabel, trashTask, updateTask } from '@/lib/api/tasks';
import { listLabels } from '@/lib/api/labels';
import { getBoard } from '@/lib/api/board';
import { statusTone } from '@/lib/board/status';
import { cn } from '@/lib/utils';
import { useOrg } from '@/lib/org/context';
import { useOrgMembers } from '@/lib/org/use-members';
import { claimLive, useOrgAgents } from '@/lib/org/use-agents';
import { useToast } from '@/components/ui/toast';
import { PRIORITIES, priorityLabel } from '@/lib/board/priority';
import type { Priority, Task, UpdateTaskInput } from '@/lib/api/types';
import { Subtasks } from './subtasks';
import { Comments } from './comments';
import { TaskLinks } from './links';
import { TaskRecurrence } from './recurrence';
import { TaskApprovals } from './approvals';
import { TaskWatchers } from './watchers';
import { TaskEpic } from './epic-field';
import { TimeTracker } from './time-tracker';
import { TaskCustomValues } from './custom-values';
import { Attachments } from './attachments';
import { ActivityTimeline } from './activity-timeline';
import { ReviewPanel } from './review-panel';
import { Select } from '@/components/ui/select';
import { DescriptionEditor } from '@/components/editor/description-editor';

// Full task detail (FR-TASK-001/002). Inline-edits every field; UpdateTask
// replaces the whole field set, so each change is merged onto the current task
// and sent as a unit, with the version it was read at: if someone (an agent,
// say) changed the task meanwhile, the server refuses it and we show the
// latest instead of overwriting their work (FR-AGT-005). Subtasks/comments/
// attachments/activity manage their own queries. A GUEST sees a read-only view
// (the server enforces the same).
function toInput(t: Task, patch: Partial<UpdateTaskInput>): UpdateTaskInput {
  return {
    title: t.title,
    description: t.description,
    assignee_id: t.assignee_id ?? null,
    priority: t.priority as Priority,
    start_date: t.start_date ?? null,
    due_date: t.due_date ?? null,
    version: t.version,
    ...patch,
  };
}


function dateInputValue(iso?: string): string {
  if (!iso) return '';
  return new Date(iso).toISOString().slice(0, 10);
}

export function TaskDetail({
  taskId,
  projectId,
  projectKey,
}: {
  taskId: string;
  projectId: string;
  projectKey: string;
}) {
  const { orgId, slug, role } = useOrg();
  const { members } = useOrgMembers();
  const { agents, names: agentNames } = useOrgAgents();
  const { toast } = useToast();
  const router = useRouter();
  const qc = useQueryClient();
  const canEdit = role !== 'GUEST';

  const taskKey = ['task', orgId, taskId] as const;
  const { data: task, isLoading, isError } = useQuery({ queryKey: taskKey, queryFn: () => getTask(orgId, taskId) });

  const labelKey = ['task-labels', orgId, taskId] as const;
  const { data: taskLabels } = useQuery({ queryKey: labelKey, queryFn: () => listTaskLabels(orgId, taskId) });
  const { data: orgLabels } = useQuery({ queryKey: ['labels', orgId], queryFn: () => listLabels(orgId) });
  // Same cache entry the board uses: gives the status (column) name and colour.
  const { data: board } = useQuery({ queryKey: ['board', projectId], queryFn: () => getBoard(orgId, projectId) });

  // In-progress edits, tied to the task they were typed for. Without one the
  // inputs show the saved text, so they follow refetches and task switches.
  const [titleEdit, setTitleEdit] = useState<{ taskId: string; text: string } | null>(null);

  const patch = useMutation({
    mutationFn: (p: Partial<UpdateTaskInput>) => updateTask(orgId, taskId, toInput(task as Task, p)),
    onSuccess: (updated) => {
      qc.setQueryData(taskKey, updated);
      qc.invalidateQueries({ queryKey: ['board', projectId] });
      qc.invalidateQueries({ queryKey: ['activity', orgId, taskId] });
    },
    onError: (err) => {
      const current = conflictCurrent(err);
      if (current) {
        qc.setQueryData(taskKey, current);
        qc.invalidateQueries({ queryKey: ['board', projectId] });
        qc.invalidateQueries({ queryKey: ['activity', orgId, taskId] });
        toast({
          title: 'This task just changed',
          description: 'Someone else updated it, so your edit wasn’t saved. The latest version is shown — apply your change again.',
          variant: 'error',
        });
        return;
      }
      toast({ title: 'Couldn’t save changes', variant: 'error' });
    },
  });

  const attach = useMutation({
    mutationFn: (labelId: string) => attachLabel(orgId, taskId, labelId),
    onSuccess: () => qc.invalidateQueries({ queryKey: labelKey }),
    onError: () => toast({ title: 'Couldn’t add label', variant: 'error' }),
  });
  const detach = useMutation({
    mutationFn: (labelId: string) => detachLabel(orgId, taskId, labelId),
    onSuccess: () => qc.invalidateQueries({ queryKey: labelKey }),
    onError: () => toast({ title: 'Couldn’t remove label', variant: 'error' }),
  });

  const trash = useMutation({
    mutationFn: () => trashTask(orgId, taskId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['board', projectId] });
      toast({ title: 'Task moved to trash', variant: 'success' });
      router.push(`/app/${slug}/projects/${projectKey}`);
    },
    onError: () => toast({ title: 'Couldn’t delete task', variant: 'error' }),
  });

  if (isLoading) return <p className="text-sm text-muted-foreground">Loading task…</p>;
  if (isError || !task) return <p className="text-sm text-destructive">Couldn’t load this task.</p>;

  const titleDraft = titleEdit?.taskId === task.id ? titleEdit.text : task.title;
  const column = board?.columns.find((c) => c.id === task.column_id);

  const attached = taskLabels ?? [];
  const attachedIds = new Set(attached.map((l) => l.id));
  const available = (orgLabels ?? []).filter((l) => !attachedIds.has(l.id));

  return (
    <div className="grid gap-8 lg:grid-cols-[1fr_19rem]">
      {/* Main column */}
      <div className="min-w-0 space-y-6">
        <div>
          <div className="flex flex-wrap items-center gap-2 pr-10 text-sm text-muted-foreground">
            <span className="font-mono text-xs font-medium">
              {projectKey}-{task.number}
            </span>
            {column && board ? (
              <span
                className={cn(
                  'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.04em]',
                  statusTone(board, column.id).pill,
                )}
              >
                {column.name}
              </span>
            ) : null}
          </div>
          {canEdit ? (
            <input
              value={titleDraft}
              onChange={(e) => setTitleEdit({ taskId: task.id, text: e.target.value })}
              onBlur={() => {
                const t = titleDraft.trim();
                if (t && t !== task.title) patch.mutate({ title: t }, { onSuccess: () => setTitleEdit(null) });
                else setTitleEdit(null);
              }}
              className="-mx-1.5 mt-2 w-[calc(100%+0.75rem)] rounded-md border-0 bg-transparent px-1.5 py-0.5 text-2xl font-semibold tracking-[-0.02em] outline-none hover:bg-secondary/50 focus:bg-secondary/50"
            />
          ) : (
            <h1 className="mt-2 text-2xl font-semibold tracking-[-0.02em]">{task.title}</h1>
          )}
        </div>

        <div>
          <h3 className="mb-2 text-sm font-semibold">Description</h3>
          <DescriptionEditor
            key={task.id}
            value={task.description}
            canEdit={canEdit}
            onSave={(description) => patch.mutateAsync({ description })}
          />
        </div>

        <Subtasks taskId={taskId} canEdit={canEdit} />
        <TaskLinks taskId={taskId} canEdit={canEdit} />
        <TaskCustomValues projectId={projectId} taskId={taskId} canEdit={canEdit} />
        <TimeTracker taskId={taskId} canEdit={canEdit} />
        <Attachments taskId={taskId} canEdit={canEdit} />
        <Comments taskId={taskId} canComment={canEdit} />
        <ActivityTimeline taskId={taskId} />
      </div>

      {/* Meta sidebar */}
      <aside className="space-y-4 lg:mt-9">
        <ReviewPanel taskId={taskId} projectId={projectId} columnId={task.column_id} canEdit={canEdit} />
        {/* Jira-style details panel: label | value rows. */}
        <div className="rounded-lg border border-border bg-card">
          <p className="border-b border-border px-4 py-2.5 text-[13px] font-semibold">Details</p>
          <div className="space-y-3 p-4">
            <Field label="Assignee">
              {canEdit ? (
                <Select
                  aria-label="Assignee"
                  value={task.assignee_id ?? ''}
                  onValueChange={(v) => patch.mutate({ assignee_id: v || null })}
                  options={[
                    { value: '', label: 'Unassigned' },
                    ...members.map((m) => ({ value: m.user_id, label: m.name || m.email })),
                    ...agents.map((a) => ({ value: a.id, label: a.name, description: 'Agent' })),
                  ]}
                />
              ) : (
                <span className="text-sm">
                  {members.find((m) => m.user_id === task.assignee_id)?.name ??
                    (task.assignee_id ? agentNames.get(task.assignee_id) : undefined) ??
                    'Unassigned'}
                </span>
              )}
              {claimLive(task) ? (
                <p className="mt-2 flex items-start gap-1.5 text-xs text-muted-foreground">
                  <Bot className="mt-px h-3.5 w-3.5 shrink-0 text-primary" aria-hidden />
                  <span>
                    {agentNames.get(task.claimed_by as string) ?? 'An agent'} is working on this until{' '}
                    {new Date(task.lease_expires_at as string).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}.
                  </span>
                </p>
              ) : null}
            </Field>

            <Field label="Priority">
              {canEdit ? (
                <Select
                  aria-label="Priority"
                  value={task.priority}
                  onValueChange={(v) => patch.mutate({ priority: v as Priority })}
                  options={PRIORITIES.map((p) => ({ value: p, label: priorityLabel(p) }))}
                />
              ) : (
                <span className="text-sm">{priorityLabel(task.priority as Priority)}</span>
              )}
            </Field>

            <Field label="Start date">
              {canEdit ? (
                <input
                  type="date"
                  value={dateInputValue(task.start_date)}
                  onChange={(e) =>
                    patch.mutate({ start_date: e.target.value ? new Date(e.target.value).toISOString() : null })
                  }
                    className="w-full rounded-md border border-input bg-card px-2.5 py-1.5 text-sm outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
                />
              ) : (
                <span className="text-sm">
                  {task.start_date ? new Date(task.start_date).toLocaleDateString() : '—'}
                </span>
              )}
            </Field>

            <Field label="Due date">
              {canEdit ? (
                <input
                  type="date"
                  value={dateInputValue(task.due_date)}
                  onChange={(e) =>
                    patch.mutate({ due_date: e.target.value ? new Date(e.target.value).toISOString() : null })
                  }
                    className="w-full rounded-md border border-input bg-card px-2.5 py-1.5 text-sm outline-none focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40"
                />
              ) : (
                <span className="text-sm">
                  {task.due_date ? new Date(task.due_date).toLocaleDateString() : '—'}
                </span>
              )}
            </Field>

            <Field label="Epic">
              <TaskEpic taskId={taskId} projectId={projectId} epicId={task.epic_id} canEdit={canEdit} />
            </Field>
            <Field label="Labels">
              <div className="flex flex-wrap gap-1.5">
                {attached.map((l) => (
                  <span
                    key={l.id}
                    className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs"
                    style={{ backgroundColor: `${l.color}22`, color: l.color }}
                  >
                    {l.name}
                    {canEdit ? (
                      <button
                        type="button"
                        onClick={() => detach.mutate(l.id)}
                        aria-label={`Remove ${l.name}`}
                        className="hover:opacity-70"
                      >
                        ×
                      </button>
                    ) : null}
                  </span>
                ))}
                {attached.length === 0 ? <span className="text-sm text-muted-foreground">None</span> : null}
              </div>
              {canEdit && available.length > 0 ? (
                <Select
                  className="mt-1.5"
                  aria-label="Add label"
                  value=""
                  placeholder="Add label…"
                  onValueChange={(v) => v && attach.mutate(v)}
                  options={available.map((l) => ({ value: l.id, label: l.name }))}
                />
              ) : null}
            </Field>
            <Field label="Repeat">
              <TaskRecurrence taskId={taskId} canEdit={canEdit} />
            </Field>
            <Field label="Watchers">
              <TaskWatchers taskId={taskId} />
            </Field>
            <Field label="Approval">
              <TaskApprovals taskId={taskId} canEdit={canEdit} />
            </Field>
          </div>
        </div>

        {canEdit ? (
          <div className="px-1">
            <button
              type="button"
              onClick={() => trash.mutate()}
              disabled={trash.isPending}
              className="inline-flex items-center gap-1.5 text-sm text-destructive hover:underline disabled:opacity-50"
            >
              <Trash2 className="h-4 w-4" /> Move to trash
            </button>
          </div>
        ) : null}
      </aside>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[5.5rem_minmax(0,1fr)] items-start gap-3">
      <p className="pt-2 text-xs font-medium text-muted-foreground">{label}</p>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
