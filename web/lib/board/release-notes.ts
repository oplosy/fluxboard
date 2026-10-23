import type { Board, TaskCard } from '@/lib/api/types';
import type { Release } from '@/lib/api/releases';
import { doneColumnId } from '@/lib/board/status';

// Release notes (FR-PROJ-010): the release's tasks that sit in the done
// column, as a Markdown list ready to paste into a changelog.

/** The board's done column: the column with the done role, else the name/last-column guess. */
export function boardDoneColumnId(board: Board): string | undefined {
  return board.columns.find((c) => c.role === 'done')?.id ?? doneColumnId(board);
}

/** Done cards of one release, in board order. */
export function doneTasksOf(board: Board, releaseId: string): TaskCard[] {
  const done = board.columns.find((c) => c.id === boardDoneColumnId(board));
  return (done?.tasks ?? []).filter((t) => t.release_id === releaseId);
}

export function releaseNotesMarkdown(release: Release, done: TaskCard[], projectKey: string): string {
  const day = release.released_at?.slice(0, 10) ?? release.release_date;
  const lines = [`## ${release.name}${day ? ` (${day})` : ''}`, ''];
  if (release.description) lines.push(release.description, '');
  if (done.length === 0) lines.push('_No completed tasks yet._');
  for (const t of done) lines.push(`- ${projectKey}-${t.number}: ${t.title}`);
  return lines.join('\n');
}
