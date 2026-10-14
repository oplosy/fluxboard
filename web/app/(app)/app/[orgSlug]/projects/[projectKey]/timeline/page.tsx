'use client';

import { use, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { getBoard } from '@/lib/api/board';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { Timeline } from '@/components/board/timeline';

export default function ProjectTimelinePage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId, slug } = useOrg();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);

  const board = useQuery({
    queryKey: ['board', project?.id],
    queryFn: () => getBoard(orgId, project!.id),
    enabled: Boolean(project),
  });

  const data = board.data;
  const tasks = useMemo(() => {
    if (!data) return [];
    const out: { task: (typeof data.columns)[number]['tasks'][number]; columnName: string }[] = [];
    for (const col of data.columns) {
      for (const t of col.tasks) out.push({ task: t, columnName: col.name });
    }
    return out;
  }, [data]);

  if (isLoading || board.isLoading) {
    return <p className="px-6 py-8 text-sm text-muted-foreground">Loading timeline…</p>;
  }
  if (isError || !project) {
    return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;
  }

  return (
    <ProjectFrame project={project}>
      <Timeline
        tasks={tasks}
        taskHref={(t) => `/app/${slug}/projects/${project.key}/tasks/${t.number}`}
      />
    </ProjectFrame>
  );
}
