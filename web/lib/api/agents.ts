import { apiFetch } from './client';
import type { Agent, AgentLimit, AgentRole, AgentSecretResult, AgentStatus } from './types';

// Org agents (ADR-033, FR-AGT-002/003). Members may list them; everything else
// is ADMIN+. A key's secret is returned exactly once, by createAgent and
// createAgentKey.

export function listAgents(orgId: string): Promise<{ agents: Agent[]; limit: AgentLimit }> {
  return apiFetch(`/orgs/${orgId}/agents`);
}

/** Create an agent with its first key. Over the plan cap → 402 (limit max_agents). */
export function createAgent(orgId: string, input: { name: string; role: AgentRole }): Promise<AgentSecretResult> {
  return apiFetch(`/orgs/${orgId}/agents`, { method: 'POST', body: input });
}

export async function updateAgent(
  orgId: string,
  id: string,
  input: { name?: string; role?: AgentRole; status?: AgentStatus },
): Promise<Agent> {
  const res = await apiFetch<{ agent: Agent }>(`/orgs/${orgId}/agents/${id}`, { method: 'PATCH', body: input });
  return res.agent;
}

/** Delete an agent (204): its keys stop working and its plan slot frees up. */
export function deleteAgent(orgId: string, id: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/agents/${id}`, { method: 'DELETE' });
}

/** Add a key (rotation). At most 3 active keys per agent → 409 agent_key_limit. */
export function createAgentKey(orgId: string, id: string): Promise<AgentSecretResult> {
  return apiFetch(`/orgs/${orgId}/agents/${id}/keys`, { method: 'POST' });
}

export function revokeAgentKey(orgId: string, id: string, keyId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/agents/${id}/keys/${keyId}`, { method: 'DELETE' });
}
