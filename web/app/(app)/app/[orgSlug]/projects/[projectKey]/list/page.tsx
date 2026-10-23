'use client';

import { use, useMemo, useState } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { getBoard, moveTask } from '@/lib/api/board';
import { patchTask } from '@/lib/api/tasks';
import {
  clearTaskFieldValue,
  listCustomFields,
  listProjectFieldValues,
  setTaskFieldValue,
  type CustomValue,
  type CustomValueInput,
} from '@/lib/api/fields';
import { ApiError } from '@/lib/api/client';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { priorityClass, priorityLabel, PRIORITIES } from '@/lib/board/priority';
import { between } from '@/lib/board/rank';
import { statusTone, type StatusTone } from '@/lib/board/status';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useOrgAgents } from '@/lib/org/use-agents';
import type { Priority, TaskCard } from '@/lib/api/types';
import { Avatar } from '@/components/ui/avatar';
import { Select } from '@/components/ui/select';
import { useToast } from '@/components/ui/toast';
import { FieldRow } from '@/components/task/custom-values';
import { cn } from '@/lib/utils';

type SortKey = 'number' | 'title' | 'priority';

const PRIORITY_ORDER = new Map(PRIORITIES.map((p, i) => [p, i]));
// Rows drawn per status group before "Show more": every row carries several
// editors, so thousands of them make the page slow to paint.
const ROW_PAGE = 100;

const cellInput =
  'w-full rounded-md border border-transparent bg-transparent px-1.5 py-1 text-xs outline-none transition-[border-color,box-shadow] hover:border-input focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

interface Group {
  id: string;
  name: string;
  tone: StatusTone;
  tasks: TaskCard[];
}

// Monday-style list: one group per board column (status), each with a coloured
// edge and count; rows sort within their group. Editors change status,
// priority, assignee, due date, points and custom fields in place (FR-PROJ-012);
// the server still gates every write by project role.
export default function ProjectListPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId, slug, role } = useOrg();
  const canEdit = role !== 'GUEST';
  const { toast } = useToast();
  const qc = useQueryClient();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);
  const { byId, members } = useOrgMembers();
  const { agents, names: agentNames } = useOrgAgents();

  const boardKey = ['board', project?.id] as const;
  const valuesKey = ['field-values', project?.id] as const;
  const board = useQuery({
    queryKey: boardKey,
    queryFn: () => getBoard(orgId, project!.id),
    enabled: Boolean(project),
  });
  const fieldsQ = useQuery({
    queryKey: ['custom-fields', project?.id],
    queryFn: () => listCustomFields(orgId, project!.id),
    enabled: Boolean(project),
  });
  const valuesQ = useQuery({
    queryKey: valuesKey,
    queryFn: () => listProjectFieldValues(orgId, project!.id),
    enabled: Boolean(project) && (fieldsQ.data?.length ?? 0) > 0,
  });
  const fields = fieldsQ.data ?? [];
  const values = useMemo(() => {
    const m = new Map<string, CustomValue>();
    for (const v of valuesQ.data ?? []) m.set(`${v.task_id}:${v.field_id}`, v);
    return m;
  }, [valuesQ.data]);

  // One mutation for every cell: run the write, then refresh what it touched.
  const edit = useMutation({
    mutationFn: (write: () => Promise<unknown>) => write(),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: boardKey });
      qc.invalidateQueries({ queryKey: valuesKey });
    },
    onError: (err) => {
      qc.invalidateQueries({ queryKey: boardKey });
      toast({
        title:
          err instanceof ApiError && err.status === 409
            ? 'This task changed meanwhile — the list has been refreshed'
            : err instanceof ApiError && err.status === 403
              ? 'You can’t edit tasks in this project'
              : 'Couldn’t save the change',
        variant: 'error',
      });
    },
  });

  const [sort, setSort] = useState<SortKey>('number');
  const [asc, setAsc] = useState(true);
  const [rowLimit, setRowLimit] = useState<Record<string, number>>({});

  const groups = useMemo<Group[]>(() => {
    const data = board.data;
    if (!data) return [];
    const dir = asc ? 1 : -1;
    const cmp = (a: TaskCard, b: TaskCard) => {
      switch (sort) {
        case 'title':
          return dir * a.title.localeCompare(b.title);
        case 'priority':
          return dir * ((PRIORITY_ORDER.get(a.priority) ?? 99) - (PRIORITY_ORDER.get(b.priority) ?? 99));
        default:
          return dir * (a.number - b.number);
      }
    };
    return data.columns.map((col) => ({
      id: col.id,
      name: col.name,
      tone: statusTone(data, col.id),
      tasks: [...col.tasks].sort(cmp),
    }));
  }, [board.data, sort, asc]);

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading project…</p>;
  if (isError || !project)
    return (
      <div className="mx-auto max-w-md px-6 py-16 text-center">
        <h1 className="text-lg font-semibold">Project not found</h1>
        <Link href={`/app/${slug}/projects`} className="mt-4 inline-block text-sm text-primary hover:underline">
          Back to projects
        </Link>
      </div>
    );

  function header(key: SortKey, label: string) {
    return (
      <button
        onClick={() => (sort === key ? setAsc((v) => !v) : (setSort(key), setAsc(true)))}
        className="flex items-center gap-1 font-medium hover:text-foreground"
      >
        {label}
        {sort === key ? <span className="text-[10px]">{asc ? '▲' : '▼'}</span> : null}
      </button>
    );
  }

  // Move a task to the end of another column (the list has no in-column order).
  function moveToColumn(t: TaskCard, columnId: string) {
    const dest = board.data?.columns.find((c) => c.id === columnId);
    if (!dest) return;
    const last = dest.tasks.filter((x) => x.id !== t.id).at(-1)?.rank ?? '';
    edit.mutate(() => moveTask(orgId, t.id, { column_id: columnId, rank: between(last, '') }));
  }

  function setField(taskId: string, fieldId: string, v: CustomValueInput | null) {
    edit.mutate(() => (v ? setTaskFieldValue(orgId, taskId, fieldId, v) : clearTaskFieldValue(orgId, taskId, fieldId)));
  }

  const statusOptions = (board.data?.columns ?? []).map((c) => ({ value: c.id, label: c.name }));
  const assigneeOptions = [
    { value: '', label: 'Unassigned' },
    ...members.map((m) => ({ value: m.user_id, label: m.name || m.email })),
    ...agents.map((a) => ({ value: a.id, label: `${a.name} (agent)` })),
  ];
  const total = groups.reduce((n, g) => n + g.tasks.length, 0);

  return (
    <ProjectFrame project={project}>
      {board.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading tasks…</p>
      ) : total === 0 ? (
        <div className="rounded-lg border border-dashed border-border px-6 py-12 text-center">
          <p className="text-sm font-medium">No tasks yet</p>
          <p className="mt-1 text-sm text-muted-foreground">Tasks you add on the board show up here, grouped by status.</p>
        </div>
      ) : (
        <div className="space-y-6">
          {groups.map((g) => (
            <section key={g.id}>
              <div className="mb-1.5 flex items-center gap-2 px-1">
                <span className={cn('h-2 w-2 rounded-full', g.tone.dot)} aria-hidden />
                <h2 className="font-sans text-[13px] font-semibold">{g.name}</h2>
                <span className="text-xs tabular-nums text-muted-foreground">{g.tasks.length}</span>
              </div>
              <div className="relative overflow-x-auto rounded-lg border border-border bg-card">
                <span className={cn('absolute inset-y-0 left-0 w-[3px]', g.tone.dot)} aria-hidden />
                <table className="w-full min-w-[760px] text-[13px]">
                  <thead className="text-left text-xs text-muted-foreground">
                    <tr className="border-b border-border">
                      <th className="py-2 pl-4 pr-3 font-medium">{header('title', 'Task')}</th>
                      <th className="w-36 px-2 py-2 font-medium">Status</th>
                      <th className="w-32 px-2 py-2">{header('priority', 'Priority')}</th>
                      <th className="w-44 px-2 py-2 font-medium">Assignee</th>
                      <th className="w-32 px-2 py-2 font-medium">Due</th>
                      <th className="w-16 px-2 py-2 font-medium">Pts</th>
                      {fields.map((f) => (
                        <th key={f.id} className="w-36 px-2 py-2 font-medium">
                          {f.name}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {g.tasks.length === 0 ? (
                      <tr>
                        <td colSpan={6 + fields.length} className="py-3 pl-4 text-xs text-muted-foreground">
                          No tasks in {g.name}
                        </td>
                      </tr>
                    ) : null}
                    {g.tasks.slice(0, rowLimit[g.id] ?? ROW_PAGE).map((t) => (
                      <tr key={t.id} className="border-b border-border/60 transition-colors last:border-0 hover:bg-secondary/40">
                        <td className="py-2 pl-4 pr-3">
                          <Link
                            href={`/app/${slug}/projects/${project.key}/tasks/${t.number}`}
                            className="flex min-w-0 items-baseline gap-2 hover:underline hover:underline-offset-2"
                          >
                            <span className="shrink-0 font-mono text-[11px] text-muted-foreground">
                              {project.key}-{t.number}
                            </span>
                            <span className="truncate">{t.title}</span>
                          </Link>
                        </td>
                        <td className="px-2 py-1.5">
                          {canEdit ? (
                            <Select
                              size="sm"
                              aria-label={`Status of ${project.key}-${t.number}`}
                              value={g.id}
                              onValueChange={(v) => v !== g.id && moveToColumn(t, v)}
                              options={statusOptions}
                            />
                          ) : (
                            <span className={cn('inline-block max-w-full truncate rounded-full px-2 py-0.5 align-middle text-[11px] font-medium', g.tone.pill)}>
                              {g.name}
                            </span>
                          )}
                        </td>
                        <td className="px-2 py-1.5">
                          {canEdit ? (
                            <Select
                              size="sm"
                              aria-label={`Priority of ${project.key}-${t.number}`}
                              value={t.priority}
                              onValueChange={(v) => v !== t.priority && edit.mutate(() => patchTask(orgId, t.id, { priority: v as Priority }))}
                              options={PRIORITIES.map((p) => ({ value: p, label: priorityLabel(p) }))}
                            />
                          ) : t.priority !== 'none' ? (
                            <span className={cn('rounded-full px-2 py-0.5 text-[11px] font-medium', priorityClass(t.priority))}>
                              {priorityLabel(t.priority)}
                            </span>
                          ) : (
                            <span className="text-xs text-muted-foreground">—</span>
                          )}
                        </td>
                        <td className="px-2 py-1.5">
                          {canEdit ? (
                            <Select
                              size="sm"
                              aria-label={`Assignee of ${project.key}-${t.number}`}
                              value={t.assignee_id ?? ''}
                              onValueChange={(v) =>
                                v !== (t.assignee_id ?? '') && edit.mutate(() => patchTask(orgId, t.id, { assignee_id: v || null }))
                              }
                              options={assigneeOptions}
                            />
                          ) : t.assignee_id ? (
                            <span className="flex min-w-0 items-center gap-2">
                              <Avatar name={memberName(byId, t.assignee_id, agentNames)} size="sm" />
                              <span className="truncate">{memberName(byId, t.assignee_id, agentNames)}</span>
                            </span>
                          ) : (
                            <span className="text-xs text-muted-foreground">Unassigned</span>
                          )}
                        </td>
                        <td className="px-2 py-1.5 text-xs text-muted-foreground">
                          {canEdit ? (
                            <input
                              type="date"
                              key={`${t.id}:${t.due_date ?? ''}`}
                              aria-label={`Due date of ${project.key}-${t.number}`}
                              defaultValue={t.due_date ? t.due_date.slice(0, 10) : ''}
                              onChange={(e) => {
                                const next = e.target.value ? new Date(e.target.value).toISOString() : null;
                                edit.mutate(() => patchTask(orgId, t.id, { due_date: next }));
                              }}
                              className={cellInput}
                            />
                          ) : t.due_date ? (
                            new Date(t.due_date).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
                          ) : (
                            '—'
                          )}
                        </td>
                        <td className="px-2 py-1.5 text-xs">
                          {canEdit ? (
                            <input
                              type="number"
                              inputMode="numeric"
                              min={0}
                              max={1000}
                              step={1}
                              placeholder="—"
                              key={`${t.id}:${t.estimate ?? ''}`}
                              aria-label={`Story points of ${project.key}-${t.number}`}
                              defaultValue={t.estimate ?? ''}
                              onKeyDown={(e) => {
                                if (e.key === 'Enter') e.currentTarget.blur();
                              }}
                              onBlur={(e) => {
                                const raw = e.target.value.trim();
                                const next = raw === '' ? null : Number(raw);
                                if (next !== null && (!Number.isInteger(next) || next < 0 || next > 1000)) {
                                  e.target.value = t.estimate == null ? '' : String(t.estimate);
                                  toast({ title: 'Story points must be a whole number from 0 to 1000', variant: 'error' });
                                  return;
                                }
                                if (next !== (t.estimate ?? null)) edit.mutate(() => patchTask(orgId, t.id, { estimate: next }));
                              }}
                              className={cn(cellInput, 'tabular-nums')}
                            />
                          ) : (
                            <span className="tabular-nums text-muted-foreground">{t.estimate ?? '—'}</span>
                          )}
                        </td>
                        {fields.map((f) => (
                          <td key={f.id} className="px-2 py-1.5 text-xs">
                            <FieldRow
                              bare
                              ariaLabel={`${f.name} of ${project.key}-${t.number}`}
                              field={f}
                              value={values.get(`${t.id}:${f.id}`)}
                              canEdit={canEdit}
                              onSet={(v) => setField(t.id, f.id, v)}
                              onClear={() => setField(t.id, f.id, null)}
                            />
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
                {g.tasks.length > (rowLimit[g.id] ?? ROW_PAGE) ? (
                  <button
                    type="button"
                    onClick={() => setRowLimit((m) => ({ ...m, [g.id]: (m[g.id] ?? ROW_PAGE) + ROW_PAGE }))}
                    className="w-full border-t border-border/60 px-4 py-2 text-left text-xs text-muted-foreground hover:bg-secondary/40 hover:text-foreground"
                  >
                    Show {Math.min(ROW_PAGE, g.tasks.length - (rowLimit[g.id] ?? ROW_PAGE))} more{' '}
                    <span className="tabular-nums">({g.tasks.length - (rowLimit[g.id] ?? ROW_PAGE)} hidden)</span>
                  </button>
                ) : null}
              </div>
            </section>
          ))}
        </div>
      )}
    </ProjectFrame>
  );
}
