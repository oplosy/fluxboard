'use client';

import { useQuery } from '@tanstack/react-query';

import { listAgents } from '@/lib/api/agents';
import { useOrg } from '@/lib/org/context';
import type { Agent } from '@/lib/api/types';

// The org's agents (ADR-033), shared by assignee pickers and anywhere a user id
// may be an agent's. Same cache key as Settings → Agents, so edits there show
// up here. Guests cannot list agents: the query stays off for them.
export function useOrgAgents(): { agents: Agent[]; names: Map<string, string> } {
  const { orgId, role } = useOrg();
  const { data } = useQuery({
    queryKey: ['agents', orgId],
    queryFn: () => listAgents(orgId),
    enabled: role !== 'GUEST',
    staleTime: 60 * 1000,
    retry: false,
  });
  const agents = data?.agents ?? [];
  return { agents, names: new Map(agents.map((a) => [a.id, a.name])) };
}

/** True while an agent's claim on a task is live. */
export function claimLive(task: { claimed_by?: string | null; lease_expires_at?: string | null }): boolean {
  return Boolean(task.claimed_by && task.lease_expires_at && new Date(task.lease_expires_at).getTime() > Date.now());
}
