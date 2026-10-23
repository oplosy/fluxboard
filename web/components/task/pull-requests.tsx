'use client';

import { useQuery } from '@tanstack/react-query';
import { GitMerge, GitPullRequest, GitPullRequestClosed } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { listTaskPullRequests, type LinkedPullRequest } from '@/lib/api/github';

const STATE: Record<
  LinkedPullRequest['state'],
  { label: string; icon: typeof GitPullRequest; className: string }
> = {
  open: {
    label: 'Open',
    icon: GitPullRequest,
    className: 'text-emerald-600 dark:text-emerald-400',
  },
  merged: { label: 'Merged', icon: GitMerge, className: 'text-violet-600 dark:text-violet-400' },
  closed: { label: 'Closed', icon: GitPullRequestClosed, className: 'text-destructive' },
};

// Pull requests linked to a task by key in the title or branch (FR-TASK-016).
export function TaskPullRequests({ taskId }: { taskId: string }) {
  const { orgId } = useOrg();
  const { data: prs = [] } = useQuery({
    queryKey: ['task-pull-requests', taskId],
    queryFn: () => listTaskPullRequests(orgId, taskId),
  });
  if (prs.length === 0) return <span className="text-sm text-muted-foreground">None</span>;
  return (
    <ul className="space-y-1.5" data-testid="task-pull-requests">
      {prs.map((pr) => {
        const s = STATE[pr.state];
        const Icon = s.icon;
        return (
          <li key={`${pr.repo}#${pr.number}`} className="flex items-start gap-2 text-sm">
            <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${s.className}`} aria-label={s.label} />
            <a
              href={pr.url}
              target="_blank"
              rel="noopener noreferrer"
              className="min-w-0 underline-offset-2 hover:underline"
            >
              <span className="break-words">{pr.title}</span>{' '}
              <span className="text-muted-foreground">
                {pr.repo}#{pr.number}
              </span>
            </a>
          </li>
        );
      })}
    </ul>
  );
}
