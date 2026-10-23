'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus, Target } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { createGoal, listGoals, type GoalInput } from '@/lib/api/goals';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useToast } from '@/components/ui/toast';
import { GoalForm } from '@/components/goals/goal-form';
import { GoalProgress, GoalStatusBadge, formatDay } from '@/components/goals/goal-progress';

// Goals (FR-PROJ-014): org-level objectives whose progress comes from the epics
// and tasks linked to them. Guests read; members create their own.
export default function GoalsPage() {
  const { orgId, slug, role } = useOrg();
  const router = useRouter();
  const qc = useQueryClient();
  const { toast } = useToast();
  const { byId: people } = useOrgMembers();
  const [showForm, setShowForm] = useState(false);

  const goalsQ = useQuery({ queryKey: ['goals', orgId], queryFn: () => listGoals(orgId) });
  const createMutation = useMutation({
    mutationFn: (input: GoalInput) => createGoal(orgId, input),
    onSuccess: (g) => {
      qc.invalidateQueries({ queryKey: ['goals', orgId] });
      router.push(`/app/${slug}/goals/${g.id}`);
    },
    onError: () => toast({ title: 'Could not create the goal', variant: 'error' }),
  });

  const goals = goalsQ.data ?? [];

  return (
    <div className="px-6 py-6">
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0 flex-1">
          <h1 className="text-xl font-bold tracking-tight">Goals</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Link epics and tasks to a goal and follow its progress across projects.
          </p>
        </div>
        {role !== 'GUEST' ? (
          <Button size="sm" className="shrink-0" onClick={() => setShowForm((v) => !v)}>
            <Plus className="h-4 w-4" /> New goal
          </Button>
        ) : null}
      </div>

      {showForm ? (
        <Card className="mb-6">
          <CardContent className="py-4">
            <GoalForm
              pending={createMutation.isPending}
              submitLabel="Create goal"
              onSubmit={(input) => createMutation.mutate(input)}
              onCancel={() => setShowForm(false)}
            />
          </CardContent>
        </Card>
      ) : null}

      {goalsQ.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading goals…</p>
      ) : goalsQ.isError ? (
        <p className="text-sm text-destructive">Couldn&apos;t load goals.</p>
      ) : goals.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed py-12 text-center">
          <Target className="h-6 w-6 text-muted-foreground" aria-hidden />
          <p className="text-sm font-medium">No goals yet</p>
          <p className="text-xs text-muted-foreground">Create one, then link the epics and tasks that deliver it.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {goals.map((g) => (
            <Card key={g.id}>
              <CardContent className="space-y-3 py-4">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <Link href={`/app/${slug}/goals/${g.id}`} className="truncate text-sm font-semibold hover:underline">
                      {g.title}
                    </Link>
                    <GoalStatusBadge status={g.status} />
                  </div>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    {memberName(people, g.owner_id)}
                    {g.target_date ? ` · Target ${formatDay(g.target_date)}` : ''}
                  </p>
                </div>
                <GoalProgress goal={g} />
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
