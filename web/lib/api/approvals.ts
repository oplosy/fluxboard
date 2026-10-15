import { apiFetch } from './client';

export type ApprovalState = 'pending' | 'approved' | 'changes_requested' | 'cancelled';
export type ApprovalDecision = 'approved' | 'changes_requested';

export interface Approver {
  user_id: string;
  decision: ApprovalDecision | ''; // '' while waiting
  comment: string;
  decided_at: string | null;
}

/** One approval request on a task (ADR-037). */
export interface Approval {
  id: string;
  task_id: string;
  requested_by: string;
  note: string;
  deadline: string | null;
  state: ApprovalState;
  /** Approved, but the task was reopened or edited after completion. */
  stale: boolean;
  decided_at: string | null;
  created_at: string;
  approvers: Approver[];
}

/** An open request awaiting the caller (GET /orgs/{id}/approvals/waiting). */
export interface WaitingApproval {
  approval_id: string;
  task_id: string;
  project_id: string;
  task_number: number;
  task_title: string;
  requested_by: string;
  deadline: string | null;
  created_at: string;
}

export interface RequestApprovalInput {
  approver_ids: string[];
  deadline?: string;
  note?: string;
}

/** The task's requests, newest first (VIEWER). */
export async function listApprovals(orgId: string, taskId: string): Promise<Approval[]> {
  const res = await apiFetch<{ approvals: Approval[] }>(`/orgs/${orgId}/tasks/${taskId}/approvals`);
  return res.approvals;
}

/** Ask approvers to sign off the task (CONTRIBUTOR). */
export function requestApproval(orgId: string, taskId: string, input: RequestApprovalInput): Promise<Approval> {
  return apiFetch<Approval>(`/orgs/${orgId}/tasks/${taskId}/approvals`, { method: 'POST', body: input });
}

/** Record the caller's decision; `comment` is required for changes_requested. */
export function decideApproval(
  orgId: string,
  taskId: string,
  approvalId: string,
  decision: ApprovalDecision,
  comment?: string,
): Promise<Approval> {
  return apiFetch<Approval>(`/orgs/${orgId}/tasks/${taskId}/approvals/${approvalId}/decision`, {
    method: 'POST',
    body: { decision, comment },
  });
}

/** Withdraw an open request (204). */
export function cancelApproval(orgId: string, taskId: string, approvalId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/tasks/${taskId}/approvals/${approvalId}/cancel`, { method: 'POST' });
}

/** Open requests the caller has yet to answer, nearest deadline first. */
export async function listWaitingApprovals(orgId: string): Promise<WaitingApproval[]> {
  const res = await apiFetch<{ approvals: WaitingApproval[] }>(`/orgs/${orgId}/approvals/waiting`);
  return res.approvals;
}
