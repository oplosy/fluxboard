'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { assignEpic, listEpics } from '@/lib/api/epics';
import { useToast } from '@/components/ui/toast';
import { Select } from '@/components/ui/select';
import { EpicChip } from '@/components/board/card';

// Epic of a task (FR-PROJ-008): a picker for editors, the epic lozenge for
// everyone else. Assignment needs CONTRIBUTOR on the project (server-gated).
export function TaskEpic({
  taskId,
  projectId,
  epicId,
  canEdit,
}: {
  taskId: string;
  projectId: string;
  epicId?: string | null;
  canEdit: boolean;
}) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const qc = useQueryClient();
  const { data: epics = [] } = useQuery({ queryKey: ['epics', projectId], queryFn: () => listEpics(orgId, projectId) });
  const current = epics.find((e) => e.id === epicId);

  const assign = useMutation({
    mutationFn: (next: string | null) => assignEpic(orgId, projectId, { task_ids: [taskId], epic_id: next }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['task', orgId, taskId] });
      qc.invalidateQueries({ queryKey: ['board', projectId] });
      qc.invalidateQueries({ queryKey: ['epics', projectId] });
    },
    onError: () => toast({ title: 'Could not change the epic', variant: 'error' }),
  });

  if (!canEdit) {
    return current ? <EpicChip epic={current} /> : <span className="text-sm text-muted-foreground">None</span>;
  }
  return (
    <Select
      aria-label="Epic"
      value={epicId ?? ''}
      onValueChange={(v) => assign.mutate(v || null)}
      options={[{ value: '', label: 'No epic' }, ...epics.map((e) => ({ value: e.id, label: e.name }))]}
    />
  );
}
