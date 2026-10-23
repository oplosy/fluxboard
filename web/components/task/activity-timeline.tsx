'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';

import { listActivity } from '@/lib/api/tasks';
import { useOrg } from '@/lib/org/context';
import { useOrgMembers, memberName } from '@/lib/org/use-members';
import { useOrgAgents } from '@/lib/org/use-agents';
import type { Activity } from '@/lib/api/types';

// Task activity timeline (FR-TASK-002). Renders the server's field-change log as
// a plain chronological list.
export function ActivityTimeline({ taskId }: { taskId: string }) {
  const { orgId, slug } = useOrg();
  const { byId } = useOrgMembers();
  const { names: agentNames } = useOrgAgents();
  const { data: activity } = useQuery({
    queryKey: ['activity', orgId, taskId],
    queryFn: () => listActivity(orgId, taskId),
  });

  const list = activity ?? [];

  function describe(a: Activity): string {
    const field = a.field === 'estimate' ? 'story points' : a.field.replace(/_/g, ' ');
    if (a.old_value && a.new_value) return `changed ${field} from “${a.old_value}” to “${a.new_value}”`;
    if (a.new_value) return `set ${field} to “${a.new_value}”`;
    if (a.old_value) return `cleared ${field}`;
    return `updated ${field}`;
  }

  return (
    <section>
      <h3 className="mb-2 text-sm font-semibold">Activity</h3>
      <ul className="space-y-2">
        {list.map((a) => (
          <li key={a.id} className="text-sm text-muted-foreground">
            <span className="font-medium text-foreground">{memberName(byId, a.actor_id, agentNames)}</span>{' '}
            {describe(a)}{' '}
            {a.automation_rule_id ? (
              <Link
                href={`/app/${slug}/settings/automations#rule-${a.automation_rule_id}`}
                className="text-xs underline underline-offset-2 hover:text-foreground"
              >
                via automation “{a.automation_rule_name ?? 'deleted rule'}”
              </Link>
            ) : null}{' '}
            <span className="text-xs">· {new Date(a.created_at).toLocaleString()}</span>
          </li>
        ))}
        {list.length === 0 ? <li className="text-sm text-muted-foreground">No activity yet.</li> : null}
      </ul>
    </section>
  );
}
