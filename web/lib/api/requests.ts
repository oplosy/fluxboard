import { apiFetch } from './client';

export type RequestStatus = 'new' | 'accepted' | 'declined' | 'done';

/** One inbox request (GET .../projects/{id}/requests -> { requests: [...] }). */
export interface IntakeRequest {
  id: string;
  form_id: string;
  form_schema_version: number;
  title: string;
  description: string;
  priority: string;
  submitter_name: string;
  submitter_email: string;
  status: RequestStatus;
  owner_id: string;
  decline_reason: string;
  task_id: string;
  received_at: string;
  decided_at: string | null;
}

const base = (orgId: string, projectId: string) => `/orgs/${orgId}/projects/${projectId}`;

/** List a project's requests newest-first (CONTRIBUTOR). */
export async function listRequests(
  orgId: string,
  projectId: string,
  status?: RequestStatus,
): Promise<IntakeRequest[]> {
  const qs = status ? `?status=${status}` : '';
  const res = await apiFetch<{ requests: IntakeRequest[] }>(`${base(orgId, projectId)}/requests${qs}`);
  return res.requests ?? [];
}

/** Set owner ('' clears) and priority on a new request. */
export function triageRequest(
  orgId: string,
  projectId: string,
  requestId: string,
  input: { owner_id: string; priority: string },
): Promise<IntakeRequest> {
  return apiFetch(`${base(orgId, projectId)}/requests/${requestId}`, { method: 'PATCH', body: input });
}

/** Accept: creates the task in the form's column. Returns the new task. */
export function acceptRequest(
  orgId: string,
  projectId: string,
  requestId: string,
): Promise<{ task_id: string; number: number }> {
  return apiFetch(`${base(orgId, projectId)}/requests/${requestId}/accept`, { method: 'POST' });
}

/** Decline with a reason that is emailed to the submitter. */
export function declineRequest(
  orgId: string,
  projectId: string,
  requestId: string,
  reason: string,
): Promise<IntakeRequest> {
  return apiFetch(`${base(orgId, projectId)}/requests/${requestId}/decline`, {
    method: 'POST',
    body: { reason },
  });
}

/** A project's member ids (names come from the org member list). */
export async function listProjectMemberIds(orgId: string, projectId: string): Promise<string[]> {
  const res = await apiFetch<{ members: { user_id: string }[] }>(`${base(orgId, projectId)}/members`);
  return (res.members ?? []).map((m) => m.user_id);
}
