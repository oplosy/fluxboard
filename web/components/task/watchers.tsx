'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Eye, EyeOff } from 'lucide-react';

import { useAuth } from '@/lib/auth/context';
import { useOrg } from '@/lib/org/context';
import { listTaskWatchers, unwatchTask, watchTask } from '@/lib/api/watchers';
import { useToast } from '@/components/ui/toast';
import { Avatar } from '@/components/ui/avatar';
import { Button } from '@/components/ui/button';

// Watchers (FR-TASK-014): anyone who can see the task may follow it and get
// its comment notifications. Callers toggle only themselves.
export function TaskWatchers({ taskId }: { taskId: string }) {
  const { orgId } = useOrg();
  const { userId } = useAuth();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const key = ['task-watchers', taskId] as const;

  const { data: watchers = [] } = useQuery({ queryKey: key, queryFn: () => listTaskWatchers(orgId, taskId) });
  const watching = watchers.some((w) => w.user_id === userId);

  const toggle = useMutation({
    mutationFn: () => (watching ? unwatchTask(orgId, taskId) : watchTask(orgId, taskId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: key }),
    onError: () => toast({ title: 'Could not update watching', variant: 'error' }),
  });

  return (
    <div className="space-y-2">
      <Button size="sm" variant="outline" onClick={() => toggle.mutate()} disabled={toggle.isPending}>
        {watching ? <EyeOff className="mr-1.5 h-3.5 w-3.5" /> : <Eye className="mr-1.5 h-3.5 w-3.5" />}
        {watching ? 'Unwatch' : 'Watch'}
      </Button>
      {watchers.length > 0 ? (
        <ul className="flex flex-wrap gap-1" aria-label="Watchers">
          {watchers.map((w) => (
            <li key={w.user_id} title={w.name || w.email}>
              <Avatar name={w.name || w.email} size="sm" />
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-xs text-muted-foreground">No one is watching this task.</p>
      )}
    </div>
  );
}
