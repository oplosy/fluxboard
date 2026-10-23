'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { listPagesMentioningTask } from '@/lib/api/wiki';

// "Mentioned in" (FR-PROJ-013): the project's docs pages that name this task
// by key. Renders nothing when there are none (or the viewer can't read docs).
export function WikiMentions({
  projectId,
  projectKey,
  taskNumber,
}: {
  projectId: string;
  projectKey: string;
  taskNumber: number;
}) {
  const { orgId, slug } = useOrg();
  const { data: pages = [] } = useQuery({
    queryKey: ['wiki-mentions', projectId, taskNumber],
    queryFn: () => listPagesMentioningTask(orgId, projectId, taskNumber),
  });
  if (pages.length === 0) return null;
  return (
    <div>
      <h3 className="mb-2 text-sm font-semibold">Mentioned in</h3>
      <ul className="space-y-1">
        {pages.map((p) => (
          <li key={p.id}>
            <Link
              href={`/app/${slug}/projects/${projectKey}/docs/${p.id}`}
              className="text-sm text-signal underline underline-offset-2 hover:no-underline"
            >
              {p.title}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
