'use client';

import { useEffect } from 'react';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { useAuth } from '@/lib/auth/context';
import { connectSse, type SseEvent } from '@/lib/sse';
import { getTask } from '@/lib/api/tasks';
import type { Board } from '@/lib/api/types';
import { moveCard, patchCard } from '@/lib/board/board-cache';
import { useLatest } from '@/lib/motion/hooks';

// Realtime cache bridge (FR-NTF-001/002, docs/09 §1). Opens the org SSE stream
// and applies *surgical* TanStack invalidations per event — e.g. a task.moved
// touches only that project's board, never a blanket refetch. Events the caller
// produced (actor_id === me) are skipped: the optimistic update already applied.
// Renders nothing.
export function Realtime() {
  const { orgId } = useOrg();
  const { userId, isAuthenticated } = useAuth();
  const qc = useQueryClient();

  // Keep the latest apply-fn in a ref so the stream connects once per org, not
  // on every render.
  const applyRef = useLatest((e: SseEvent) => applyEvent(qc, orgId, userId, e));

  useEffect(() => {
    if (!orgId || !isAuthenticated) return;
    const dispose = connectSse(`/orgs/${orgId}/events`, {
      onEvent: (e) => applyRef.current(e),
      onResync: () => {
        // Backlog expired — the safe move is a full refetch of this org's data.
        void qc.invalidateQueries();
      },
    });
    return dispose;
  }, [orgId, isAuthenticated, qc, applyRef]);

  return null;
}

// Board refetches still needed (create / delete / restore) are coalesced per
// project: a burst of events within the window costs one refetch, not one each.
const BOARD_REFETCH_DEBOUNCE_MS = 300;
const pendingBoardRefetch = new Map<string, ReturnType<typeof setTimeout>>();

function refetchBoardSoon(qc: QueryClient, projectId: string): void {
  if (pendingBoardRefetch.has(projectId)) return;
  pendingBoardRefetch.set(
    projectId,
    setTimeout(() => {
      pendingBoardRefetch.delete(projectId);
      void qc.invalidateQueries({ queryKey: ['board', projectId] });
    }, BOARD_REFETCH_DEBOUNCE_MS),
  );
}

function applyEvent(
  qc: QueryClient,
  orgId: string,
  me: string | null,
  e: SseEvent,
): void {
  const d = e.data;
  const actor = typeof d.actor_id === 'string' ? d.actor_id : '';
  const isSelf = me != null && actor === me;
  const projectId = typeof d.project_id === 'string' ? d.project_id : undefined;
  const taskId = typeof d.task_id === 'string' ? d.task_id : undefined;

  const inv = (key: readonly unknown[]) => void qc.invalidateQueries({ queryKey: key });

  switch (e.name) {
    case 'task.moved': {
      if (isSelf || !projectId) return;
      // The event carries the destination: move the cached card, no refetch.
      const board = qc.getQueryData<Board>(['board', projectId]);
      const next =
        board && taskId && typeof d.to_column === 'string' && typeof d.rank === 'string'
          ? moveCard(board, taskId, d.to_column, d.rank)
          : null;
      if (next) qc.setQueryData(['board', projectId], next);
      else if (board) refetchBoardSoon(qc, projectId);
      return;
    }
    case 'task.created':
    case 'task.deleted':
    case 'task.restored': {
      if (isSelf) return;
      if (projectId) refetchBoardSoon(qc, projectId);
      return;
    }
    case 'task.updated': {
      if (isSelf || !taskId) return;
      inv(['activity', orgId, taskId]);
      const board = projectId ? qc.getQueryData<Board>(['board', projectId]) : undefined;
      if (!board && !qc.getQueryData(['task', orgId, taskId])) return; // nothing on screen shows it
      // One small task read refreshes both the detail cache and its board card.
      getTask(orgId, taskId)
        .then((task) => {
          qc.setQueryData(['task', orgId, taskId], task);
          if (!projectId) return;
          const current = qc.getQueryData<Board>(['board', projectId]);
          const next = current ? patchCard(current, task) : null;
          if (next) qc.setQueryData(['board', projectId], next);
          else if (current) refetchBoardSoon(qc, projectId);
        })
        .catch(() => {
          inv(['task', orgId, taskId]);
          if (projectId) refetchBoardSoon(qc, projectId);
        });
      return;
    }
    case 'comment.created': {
      if (isSelf) return;
      if (taskId) {
        inv(['comments', orgId, taskId]);
        inv(['activity', orgId, taskId]);
      }
      return;
    }
    case 'member.joined':
    case 'member.left':
    case 'member.role_changed':
    case 'membership.revoked': {
      inv(['org-members', orgId]);
      inv(['orgs']);
      return;
    }
    case 'notification.created': {
      // Targeted by user_id — only react when it's addressed to me.
      const target = typeof d.user_id === 'string' ? d.user_id : '';
      if (target && me != null && target !== me) return;
      inv(['unread-count', orgId]);
      inv(['notifications', orgId]);
      inv(['home-activity', orgId]);
      return;
    }
    case 'billing.status_changed': {
      inv(['billing-summary', orgId]);
      inv(['usage', orgId]);
      return;
    }
    default:
      return;
  }
}
