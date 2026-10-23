'use client';

import { use, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Layers, Pencil, SquareCheck, Trash2, X } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useAuth } from '@/lib/auth/context';
import { deleteGoal, getGoal, linkGoalItem, updateGoal, type GoalInput, type GoalItem } from '@/lib/api/goals';
import { listEpics } from '@/lib/api/epics';
import { searchTasks } from '@/lib/api/tasks';
import { ApiError } from '@/lib/api/client';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useProjectsMap } from '@/lib/board/use-projects';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useToast } from '@/components/ui/toast';
import { GoalForm, goalInputClass } from '@/components/goals/goal-form';
import { GoalProgress, GoalStatusBadge, formatDay } from '@/components/goals/goal-progress';

// Picker for the work a goal links: pick a project, then one of its epics or a
// task found by text. Only projects the caller can see are listed by the API.
function LinkPicker({ onLink, linked, pending }: { onLink: (kind: GoalItem['kind'], id: string) => void; linked: Set<string>; pending: boolean }) {
  const { orgId } = useOrg();
  const { projects } = useProjectsMap();
  const [projectId, setProjectId] = useState('');
  const [text, setText] = useState('');
  const open = projects.filter((p) => !p.archived_at);

  const epicsQ = useQuery({
    queryKey: ['epics', projectId],
    queryFn: () => listEpics(orgId, projectId),
    enabled: Boolean(projectId),
  });
  const tasksQ = useQuery({
    queryKey: ['goal-task-search', projectId, text],
    queryFn: () => searchTasks(orgId, { project_id: projectId, q: text.trim(), limit: 10 }),
    enabled: Boolean(projectId),
  });

  return (
    <div className="space-y-3">
      <label className="block text-xs font-medium text-muted-foreground">
        Project
        <select value={projectId} onChange={(e) => setProjectId(e.target.value)} className={`mt-1 ${goalInputClass}`}>
          <option value="">Choose a project…</option>
          {open.map((p) => (
            <option key={p.id} value={p.id}>
              {p.key} · {p.name}
            </option>
          ))}
        </select>
      </label>

      {projectId ? (
        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <p className="mb-1 text-xs font-medium text-muted-foreground">Epics</p>
            <ul className="space-y-1">
              {(epicsQ.data ?? []).map((e) => (
                <li key={e.id} className="flex items-center justify-between gap-2 text-sm">
                  <span className="truncate">{e.name}</span>
                  <Button size="sm" variant="outline" disabled={linked.has(e.id) || pending} onClick={() => onLink('epic', e.id)}>
                    {linked.has(e.id) ? 'Linked' : 'Link'}
                  </Button>
                </li>
              ))}
              {epicsQ.isSuccess && (epicsQ.data ?? []).length === 0 ? (
                <li className="text-xs text-muted-foreground">No epics in this project.</li>
              ) : null}
            </ul>
          </div>
          <div>
            <label className="block text-xs font-medium text-muted-foreground">
              Tasks
              <input
                value={text}
                onChange={(e) => setText(e.target.value)}
                placeholder="Search tasks by title"
                className={`mt-1 ${goalInputClass}`}
              />
            </label>
            <ul className="mt-2 space-y-1">
              {(tasksQ.data?.tasks ?? []).map((t) => (
                <li key={t.id} className="flex items-center justify-between gap-2 text-sm">
                  <span className="truncate">
                    <span className="font-mono text-xs text-muted-foreground">#{t.number}</span> {t.title}
                  </span>
                  <Button size="sm" variant="outline" disabled={linked.has(t.id) || pending} onClick={() => onLink('task', t.id)}>
                    {linked.has(t.id) ? 'Linked' : 'Link'}
                  </Button>
                </li>
              ))}
              {tasksQ.isSuccess && (tasksQ.data?.tasks ?? []).length === 0 ? (
                <li className="text-xs text-muted-foreground">No matching tasks.</li>
              ) : null}
            </ul>
          </div>
        </div>
      ) : null}
    </div>
  );
}

// Goal page (FR-PROJ-013): progress, the linked epics and tasks, and — for the
// owner or an org admin — editing and linking.
export default function GoalPage(props: { params: Promise<{ goalId: string }> }) {
  const { goalId } = use(props.params);
  const { orgId, slug, isAdmin } = useOrg();
  const { userId } = useAuth();
  const router = useRouter();
  const qc = useQueryClient();
  const { toast } = useToast();
  const { byId: people } = useOrgMembers();
  const [editing, setEditing] = useState(false);

  const goalKey = ['goal', orgId, goalId] as const;
  const goalQ = useQuery({ queryKey: goalKey, queryFn: () => getGoal(orgId, goalId) });
  const invalidate = () => {
    qc.invalidateQueries({ queryKey: goalKey });
    qc.invalidateQueries({ queryKey: ['goals', orgId] });
  };
  const onError = (err: unknown, fallback: string) =>
    toast({
      title: err instanceof ApiError && err.status === 403 ? 'Only the goal owner or an admin can change this goal' : fallback,
      variant: 'error',
    });

  const updateMutation = useMutation({
    mutationFn: (input: GoalInput) => updateGoal(orgId, goalId, input),
    onSuccess: () => {
      invalidate();
      setEditing(false);
    },
    onError: (e) => onError(e, 'Could not update the goal'),
  });
  const deleteMutation = useMutation({
    mutationFn: () => deleteGoal(orgId, goalId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['goals', orgId] });
      router.push(`/app/${slug}/goals`);
    },
    onError: (e) => onError(e, 'Could not delete the goal'),
  });
  const linkMutation = useMutation({
    mutationFn: (v: { kind: GoalItem['kind']; id: string; link: boolean }) => linkGoalItem(orgId, goalId, v.kind, v.id, v.link),
    onSuccess: invalidate,
    onError: (e) => onError(e, 'Could not change the link'),
  });

  if (goalQ.isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading goal…</p>;
  if (goalQ.isError || !goalQ.data) return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this goal.</p>;

  const g = goalQ.data;
  const canManage = isAdmin || g.owner_id === userId;
  const linked = new Set(g.items.map((i) => i.id));

  return (
    <div className="px-6 py-6">
      <Link href={`/app/${slug}/goals`} className="mb-4 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-3.5 w-3.5" /> All goals
      </Link>

      <Card className="mb-6">
        <CardContent className="space-y-3 py-4">
          {editing ? (
            <GoalForm
              initial={g}
              pending={updateMutation.isPending}
              submitLabel="Save"
              onSubmit={(input) => updateMutation.mutate(input)}
              onCancel={() => setEditing(false)}
            />
          ) : (
            <div className="flex flex-wrap items-start gap-3">
              <div className="min-w-0 flex-1 basis-56">
                <div className="flex flex-wrap items-center gap-2">
                  <h1 className="text-lg font-bold tracking-tight">{g.title}</h1>
                  <GoalStatusBadge status={g.status} />
                </div>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  {memberName(people, g.owner_id)}
                  {g.target_date ? ` · Target ${formatDay(g.target_date)}` : ''}
                </p>
                {g.description ? <p className="mt-2 whitespace-pre-wrap text-sm">{g.description}</p> : null}
              </div>
              {canManage ? (
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setEditing(true)}
                    aria-label={`Edit ${g.title}`}
                    className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      if (window.confirm(`Delete "${g.title}"? The linked epics and tasks stay.`)) deleteMutation.mutate();
                    }}
                    aria-label={`Delete ${g.title}`}
                    className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-destructive"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </button>
                </div>
              ) : null}
            </div>
          )}
          <GoalProgress goal={g} />
        </CardContent>
      </Card>

      <h2 className="mb-2 text-sm font-semibold">Linked work</h2>
      {g.items.length === 0 ? (
        <p className="mb-6 rounded-lg border border-dashed py-8 text-center text-xs text-muted-foreground">
          Nothing linked yet. Progress comes from the tasks of linked epics and tasks.
        </p>
      ) : (
        <ul className="mb-6 divide-y rounded-lg border">
          {g.items.map((it) => {
            const href =
              it.kind === 'epic'
                ? `/app/${slug}/projects/${it.project_key}/epics`
                : `/app/${slug}/projects/${it.project_key}/tasks/${it.number}`;
            const Icon = it.kind === 'epic' ? Layers : SquareCheck;
            return (
              <li key={`${it.kind}-${it.id}`} className="flex items-center gap-3 px-3 py-2">
                <Icon className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
                <Link href={href} className="min-w-0 flex-1 truncate text-sm hover:underline">
                  <span className="font-mono text-xs text-muted-foreground">
                    {it.kind === 'task' ? `${it.project_key}-${it.number}` : it.project_key}
                  </span>{' '}
                  {it.title}
                </Link>
                <span className="font-mono text-[11px] text-muted-foreground">
                  {it.done}/{it.total} done
                </span>
                {canManage ? (
                  <button
                    type="button"
                    onClick={() => linkMutation.mutate({ kind: it.kind, id: it.id, link: false })}
                    aria-label={`Unlink ${it.title}`}
                    className="rounded-md p-1 text-muted-foreground hover:bg-secondary hover:text-destructive"
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}

      {canManage ? (
        <Card>
          <CardContent className="space-y-2 py-4">
            <h2 className="text-sm font-semibold">Link work</h2>
            <LinkPicker
              linked={linked}
              pending={linkMutation.isPending}
              onLink={(kind, id) => linkMutation.mutate({ kind, id, link: true })}
            />
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}
