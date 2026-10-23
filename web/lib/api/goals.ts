import { apiFetch } from './client';

export type GoalStatus = 'on_track' | 'at_risk' | 'done';

/** One goal with derived progress (GET /orgs/{id}/goals -> { goals }). */
export interface Goal {
  id: string;
  title: string;
  description: string;
  owner_id: string;
  /** Target day, YYYY-MM-DD. */
  target_date: string | null;
  status: GoalStatus;
  /** Distinct visible tasks linked directly or through a linked epic. */
  total: number;
  /** Of those, tasks in the done column. */
  done: number;
  /** Summed estimates of those tasks. */
  points: number;
  /** Of those, points in the done column. */
  done_points: number;
  created_at: string;
}

/** A linked epic or task the caller may see. For an epic total/done count its tasks. */
export interface GoalItem {
  kind: 'epic' | 'task';
  id: string;
  project_id: string;
  project_key: string;
  /** Task number in its project; 0 for an epic. */
  number: number;
  title: string;
  total: number;
  done: number;
}

export interface GoalDetail extends Goal {
  items: GoalItem[];
}

/** Payload for creating or updating a goal. */
export interface GoalInput {
  title: string;
  description?: string;
  target_date?: string | null;
}

/** List the org's goals oldest-first (FR-PROJ-014). */
export async function listGoals(orgId: string): Promise<Goal[]> {
  const res = await apiFetch<{ goals: Goal[] }>(`/orgs/${orgId}/goals`);
  return res.goals ?? [];
}

export function getGoal(orgId: string, goalId: string): Promise<GoalDetail> {
  return apiFetch(`/orgs/${orgId}/goals/${goalId}`);
}

/** Create a goal owned by the caller (201, MEMBER+). */
export function createGoal(orgId: string, input: GoalInput): Promise<GoalDetail> {
  return apiFetch(`/orgs/${orgId}/goals`, { method: 'POST', body: input });
}

/** Edit a goal (owner or ADMIN+). */
export function updateGoal(orgId: string, goalId: string, input: GoalInput): Promise<GoalDetail> {
  return apiFetch(`/orgs/${orgId}/goals/${goalId}`, { method: 'PATCH', body: input });
}

/** Delete a goal; the linked work stays (204, owner or ADMIN+). */
export function deleteGoal(orgId: string, goalId: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/goals/${goalId}`, { method: 'DELETE' });
}

/** Link (204, idempotent) or unlink an epic or task (owner or ADMIN+). */
export function linkGoalItem(orgId: string, goalId: string, kind: GoalItem['kind'], itemId: string, link: boolean): Promise<void> {
  return apiFetch(`/orgs/${orgId}/goals/${goalId}/${kind}s/${itemId}`, { method: link ? 'PUT' : 'DELETE' });
}
