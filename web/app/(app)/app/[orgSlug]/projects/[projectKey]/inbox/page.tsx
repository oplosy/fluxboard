'use client';

import Link from 'next/link';
import { use, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { ApiError } from '@/lib/api/client';
import { listOrgMembers } from '@/lib/api/orgs';
import {
  acceptRequest,
  declineRequest,
  listProjectMemberIds,
  listRequests,
  triageRequest,
  type RequestStatus,
} from '@/lib/api/requests';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Select } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useToast } from '@/components/ui/toast';
import { cn } from '@/lib/utils';

const FILTERS: { value: RequestStatus | ''; label: string }[] = [
  { value: 'new', label: 'New' },
  { value: 'accepted', label: 'Accepted' },
  { value: 'done', label: 'Done' },
  { value: 'declined', label: 'Declined' },
  { value: '', label: 'All' },
];

const STATUS_VARIANT: Record<RequestStatus, 'warning' | 'default' | 'success' | 'secondary'> = {
  new: 'warning',
  accepted: 'default',
  done: 'success',
  declined: 'secondary',
};

// Request inbox: public-form submissions awaiting triage. Accepting creates the
// task; declining mails the typed reason to the submitter.
export default function ProjectInboxPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId, slug } = useOrg();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);

  const [filter, setFilter] = useState<RequestStatus | ''>('new');
  const [declining, setDeclining] = useState<string | null>(null);
  const [reason, setReason] = useState('');

  const requestsKey = ['requests', project?.id, filter] as const;
  const requestsQ = useQuery({
    queryKey: requestsKey,
    queryFn: () => listRequests(orgId, project!.id, filter || undefined),
    enabled: Boolean(project),
  });
  const ownersQ = useQuery({
    queryKey: ['request-owners', project?.id],
    queryFn: async () => {
      const [ids, members] = await Promise.all([
        listProjectMemberIds(orgId, project!.id),
        listOrgMembers(orgId, { limit: 200 }),
      ]);
      const byId = new Map(members.items.map((m) => [m.user_id, m]));
      return ids.map((id) => ({ value: id, label: byId.get(id)?.name || byId.get(id)?.email || id }));
    },
    enabled: Boolean(project),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['requests', project?.id] });
    queryClient.invalidateQueries({ queryKey: ['board', project?.id] });
  };
  const fail = (title: string) => (err: unknown) =>
    toast({
      title: err instanceof ApiError && err.status === 409 ? 'This request was already handled' : title,
      variant: 'error',
    });

  const triage = useMutation({
    mutationFn: (v: { id: string; owner_id: string; priority: string }) =>
      triageRequest(orgId, project!.id, v.id, { owner_id: v.owner_id, priority: v.priority }),
    onSuccess: invalidate,
    onError: fail('Could not update the request'),
  });
  const accept = useMutation({
    mutationFn: (id: string) => acceptRequest(orgId, project!.id, id),
    onSuccess: (res) => {
      invalidate();
      toast({ title: `Created task #${res.number}` });
    },
    onError: fail('Could not accept the request'),
  });
  const decline = useMutation({
    mutationFn: (v: { id: string; reason: string }) => declineRequest(orgId, project!.id, v.id, v.reason),
    onSuccess: () => {
      invalidate();
      setDeclining(null);
      setReason('');
    },
    onError: fail('Could not decline the request'),
  });

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading inbox…</p>;
  if (isError || !project)
    return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;

  const requests = requestsQ.data ?? [];
  const owners = ownersQ.data ?? [];
  const ownerOptions = [{ value: '', label: 'Unassigned' }, ...owners];
  const boardHref = `/app/${slug}/projects/${params.projectKey}`;

  return (
    <ProjectFrame project={project} narrow>
      <p className="mb-4 text-sm text-muted-foreground">
        Requests from your public forms. Accept one to create a task; declining emails the reason to
        the submitter. Internal comments on the task are never shared with them.
      </p>
      <div className="mb-4 flex flex-wrap gap-1" role="tablist" aria-label="Request status">
        {FILTERS.map((f) => (
          <button
            key={f.label}
            type="button"
            role="tab"
            aria-selected={filter === f.value}
            onClick={() => setFilter(f.value)}
            className={cn(
              'rounded-md px-3 py-1.5 text-sm transition-colors',
              filter === f.value
                ? 'bg-foreground text-background'
                : 'text-muted-foreground hover:bg-secondary',
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      {requestsQ.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading requests…</p>
      ) : requests.length === 0 ? (
        <Card>
          <CardContent className="py-10 text-center text-sm text-muted-foreground">
            No {filter ? filter : ''} requests yet.
          </CardContent>
        </Card>
      ) : (
        <ul className="space-y-3">
          {requests.map((r) => (
            <li key={r.id}>
              <Card>
                <CardContent className="space-y-3 py-4">
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div className="min-w-0">
                      <h2 className="truncate text-sm font-semibold">{r.title}</h2>
                      <p className="text-xs text-muted-foreground">
                        {r.submitter_name || 'Anonymous'}
                        {r.submitter_email ? ` <${r.submitter_email}>` : ''} ·{' '}
                        {new Date(r.received_at).toLocaleString()}
                      </p>
                    </div>
                    <Badge variant={STATUS_VARIANT[r.status]}>{r.status}</Badge>
                  </div>
                  {r.description ? (
                    <p className="whitespace-pre-wrap text-sm">{r.description}</p>
                  ) : null}
                  {r.status === 'declined' && r.decline_reason ? (
                    <p className="text-sm text-muted-foreground">Reason: {r.decline_reason}</p>
                  ) : null}
                  {r.task_id ? (
                    <Link href={boardHref} className="text-sm underline">
                      View on the board
                    </Link>
                  ) : null}

                  {r.status === 'new' ? (
                    <div className="space-y-3 border-t border-border pt-3">
                      <div className="flex flex-wrap items-center gap-3">
                        <div className="w-48">
                          <Select
                            id={`owner-${r.id}`}
                            aria-label="Owner"
                            value={r.owner_id}
                            onValueChange={(v) =>
                              triage.mutate({ id: r.id, owner_id: v, priority: r.priority })
                            }
                            options={ownerOptions}
                          />
                        </div>
                        <Button size="sm" onClick={() => accept.mutate(r.id)} disabled={accept.isPending}>
                          Accept
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setDeclining(declining === r.id ? null : r.id);
                            setReason('');
                          }}
                        >
                          Decline
                        </Button>
                      </div>
                      {declining === r.id ? (
                        <div className="space-y-2">
                          <Textarea
                            value={reason}
                            onChange={(e) => setReason(e.target.value)}
                            rows={2}
                            maxLength={500}
                            placeholder="Reason (emailed to the submitter)"
                            aria-label="Decline reason"
                          />
                          <Button
                            size="sm"
                            variant="destructive"
                            disabled={!reason.trim() || decline.isPending}
                            onClick={() => decline.mutate({ id: r.id, reason: reason.trim() })}
                          >
                            Confirm decline
                          </Button>
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                </CardContent>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </ProjectFrame>
  );
}
