'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { assignRelease, listReleases } from '@/lib/api/releases';
import { useToast } from '@/components/ui/toast';
import { Select } from '@/components/ui/select';

// Release of a task (FR-PROJ-010): a picker for editors, the release name for
// everyone else. Assignment needs CONTRIBUTOR on the project (server-gated).
export function TaskRelease({
  taskId,
  projectId,
  releaseId,
  canEdit,
}: {
  taskId: string;
  projectId: string;
  releaseId?: string | null;
  canEdit: boolean;
}) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const qc = useQueryClient();
  const { data: releases = [] } = useQuery({
    queryKey: ['releases', projectId],
    queryFn: () => listReleases(orgId, projectId),
  });
  const current = releases.find((r) => r.id === releaseId);

  const assign = useMutation({
    mutationFn: (next: string | null) => assignRelease(orgId, projectId, { task_ids: [taskId], release_id: next }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['task', orgId, taskId] });
      qc.invalidateQueries({ queryKey: ['board', projectId] });
      qc.invalidateQueries({ queryKey: ['releases', projectId] });
    },
    onError: () => toast({ title: 'Could not change the release', variant: 'error' }),
  });

  if (!canEdit) {
    return <span className="text-sm">{current?.name ?? <span className="text-muted-foreground">None</span>}</span>;
  }
  return (
    <Select
      aria-label="Release"
      value={releaseId ?? ''}
      onValueChange={(v) => assign.mutate(v || null)}
      options={[
        { value: '', label: 'No release' },
        ...releases.map((r) => ({ value: r.id, label: r.released_at ? `${r.name} (released)` : r.name })),
      ]}
    />
  );
}
