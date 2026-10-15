'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';

import { listWaitingApprovals } from '@/lib/api/approvals';
import { searchTasks } from '@/lib/api/tasks';
import { useOrg } from '@/lib/org/context';
import { useAuth } from '@/lib/auth/context';
import { useProjectsMap } from '@/lib/board/use-projects';
import { TaskRow } from '@/components/task/task-row';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useNow } from '@/lib/use-now';
import type { Task } from '@/lib/api/types';

// Cross-project "assigned to me", grouped by project (7.6.5). There is no server
// assignee=me alias, so the caller's own id is passed to search (ADR-016).
export default function MyTasksPage() {
  const { orgId, slug } = useOrg();
  const { userId } = useAuth();
  const { byId } = useProjectsMap();
  const { byId: people } = useOrgMembers();
  const [waitingOnly, setWaitingOnly] = useState(false);
  const now = useNow(false);

  const { data, isLoading, isError } = useQuery({
    queryKey: ['my-tasks', orgId, userId],
    queryFn: () => searchTasks(orgId, { assignee_id: userId as string, limit: 100 }),
    enabled: Boolean(userId),
  });

  const { data: waiting, isError: waitingError } = useQuery({
    queryKey: ['waiting-approvals', orgId],
    queryFn: () => listWaitingApprovals(orgId),
  });
  const waitingList = waiting ?? [];

  const tasks = data?.tasks ?? [];

  // Group by project_id, preserving first-seen order.
  const groups: { projectId: string; tasks: Task[] }[] = [];
  const index = new Map<string, number>();
  for (const t of tasks) {
    let i = index.get(t.project_id);
    if (i === undefined) {
      i = groups.length;
      index.set(t.project_id, i);
      groups.push({ projectId: t.project_id, tasks: [] });
    }
    groups[i]!.tasks.push(t);
  }

  return (
    <div className="px-6 py-6">
      <h1 className="mb-4 text-xl font-bold tracking-tight">My tasks</h1>

      <div className="mb-4 flex gap-2" role="group" aria-label="Filter">
        <button
          type="button"
          aria-pressed={!waitingOnly}
          onClick={() => setWaitingOnly(false)}
          className={`rounded-md border px-3 py-1 text-sm ${!waitingOnly ? 'border-foreground bg-foreground text-background' : 'border-input hover:border-foreground/40'}`}
        >
          Assigned to me
        </button>
        <button
          type="button"
          aria-pressed={waitingOnly}
          onClick={() => setWaitingOnly(true)}
          className={`rounded-md border px-3 py-1 text-sm ${waitingOnly ? 'border-foreground bg-foreground text-background' : 'border-input hover:border-foreground/40'}`}
        >
          Waiting for my approval{waitingList.length ? ` (${waitingList.length})` : ''}
        </button>
      </div>

      {waitingOnly ? (
        waitingError ? (
          <p className="text-sm text-destructive">Couldn’t load your approvals.</p>
        ) : waitingList.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nothing is waiting for your approval.</p>
        ) : (
          <ul className="space-y-2">
            {waitingList.map((w) => {
              const project = byId.get(w.project_id);
              const overdue = w.deadline !== null && new Date(w.deadline).getTime() < now;
              const label = (
                <>
                  <span className="font-medium">{w.task_title}</span>
                  <span className="block text-xs text-muted-foreground">
                    {project ? `${project.name} · ` : ''}requested by {memberName(people, w.requested_by)}
                    {w.deadline ? (
                      <span className={overdue ? ' font-medium text-destructive' : ''}>
                        {' '}
                        · {overdue ? 'overdue ' : 'due '}
                        {new Date(w.deadline).toLocaleDateString()}
                      </span>
                    ) : null}
                  </span>
                </>
              );
              return (
                <li key={w.approval_id} className="rounded-md border border-border bg-card px-3 py-2 text-sm">
                  {project ? (
                    <Link href={`/app/${slug}/projects/${project.key}/tasks/${w.task_number}`} className="block hover:underline">
                      {label}
                    </Link>
                  ) : (
                    label
                  )}
                </li>
              );
            })}
          </ul>
        )
      ) : isLoading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : isError ? (
        <p className="text-sm text-destructive">Couldn’t load your tasks.</p>
      ) : tasks.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nothing assigned to you right now.</p>
      ) : (
        <div className="space-y-6">
          {groups.map((g) => {
            const project = byId.get(g.projectId);
            return (
              <section key={g.projectId}>
                <h2 className="mb-2 text-sm font-semibold text-muted-foreground">
                  {project ? project.name : 'Unknown project'}
                </h2>
                <div className="space-y-2">
                  {g.tasks.map((t) => (
                    <TaskRow key={t.id} task={t} project={project} orgSlug={slug} />
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
