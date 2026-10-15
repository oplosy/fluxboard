'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, CornerUpLeft } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useToast } from '@/components/ui/toast';
import {
  cancelApproval,
  decideApproval,
  listApprovals,
  requestApproval,
  type Approval,
  type ApprovalDecision,
} from '@/lib/api/approvals';
import { ApiError } from '@/lib/api/client';
import { useAuth } from '@/lib/auth/context';
import { useOrg } from '@/lib/org/context';
import { memberName, useOrgMembers } from '@/lib/org/use-members';
import { useNow } from '@/lib/use-now';

// Approvals (ADR-037): a contributor asks named people to sign off the task;
// each answers once. Separate from the board's review stage, which moves the
// card. People cannot approve their own request.
export function TaskApprovals({ taskId, canEdit }: { taskId: string; canEdit: boolean }) {
  const { orgId } = useOrg();
  const { userId } = useAuth();
  const { members, byId } = useOrgMembers();
  const { toast } = useToast();
  const qc = useQueryClient();
  const key = ['task-approvals', orgId, taskId] as const;

  const { data: approvals, isLoading } = useQuery({ queryKey: key, queryFn: () => listApprovals(orgId, taskId) });
  const refresh = () => {
    qc.invalidateQueries({ queryKey: key });
    qc.invalidateQueries({ queryKey: ['waiting-approvals', orgId] });
  };
  const fail = (title: string) => (err: unknown) =>
    toast({ title, description: err instanceof ApiError ? err.message : undefined, variant: 'error' });

  const [picked, setPicked] = useState<string[]>([]);
  const [deadline, setDeadline] = useState('');
  const [note, setNote] = useState('');
  const [changes, setChanges] = useState(false);
  const [comment, setComment] = useState('');
  const [commentError, setCommentError] = useState<string | null>(null);

  const request = useMutation({
    mutationFn: () =>
      requestApproval(orgId, taskId, {
        approver_ids: picked,
        // End of the chosen day, local time, so "due Friday" lasts all Friday.
        deadline: deadline ? new Date(`${deadline}T23:59:59`).toISOString() : undefined,
        note: note.trim() || undefined,
      }),
    onSuccess: () => {
      setPicked([]);
      setDeadline('');
      setNote('');
      refresh();
      toast({ title: 'Approval requested', variant: 'success' });
    },
    onError: fail('Couldn’t request approval'),
  });
  const decide = useMutation({
    mutationFn: (v: { id: string; decision: ApprovalDecision; comment?: string }) =>
      decideApproval(orgId, taskId, v.id, v.decision, v.comment),
    onSuccess: () => {
      setChanges(false);
      setComment('');
      refresh();
    },
    onError: (err) => {
      const msg = err instanceof ApiError ? err.fieldErrors().comment : undefined;
      if (msg) setCommentError(msg);
      else fail('Couldn’t record your decision')(err);
    },
  });
  const cancel = useMutation({
    mutationFn: (id: string) => cancelApproval(orgId, taskId, id),
    onSuccess: refresh,
    onError: fail('Couldn’t cancel the request'),
  });

  if (isLoading) return <span className="text-sm text-muted-foreground">…</span>;
  const current: Approval | undefined = approvals?.[0];
  const open = current?.state === 'pending' ? current : undefined;
  const mine = open?.approvers.find((a) => a.user_id === userId && a.decision === '');
  const candidates = members.filter((m) => m.user_id !== userId);

  return (
    <div className="space-y-3">
      {current ? (
        <div className="space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <StateBadge approval={current} />
            {current.deadline ? <Deadline approval={current} /> : null}
          </div>
          <p className="text-xs text-muted-foreground">
            Requested by {memberName(byId, current.requested_by)}
            {current.note ? ` — ${current.note}` : ''}
          </p>
          <ul className="space-y-1">
            {current.approvers.map((a) => (
              <li key={a.user_id} className="text-sm">
                <span>{memberName(byId, a.user_id)}</span>{' '}
                <span className="text-xs text-muted-foreground">
                  {a.decision === 'approved'
                    ? '· approved'
                    : a.decision === 'changes_requested'
                      ? '· requested changes'
                      : '· waiting'}
                </span>
                {a.comment ? <p className="text-xs text-muted-foreground">“{a.comment}”</p> : null}
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p className="text-sm text-muted-foreground">No approval requested.</p>
      )}

      {mine && open ? (
        changes ? (
          <form
            className="space-y-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (!comment.trim()) {
                setCommentError('Say what needs to change.');
                return;
              }
              decide.mutate({ id: open.id, decision: 'changes_requested', comment: comment.trim() });
            }}
          >
            <label htmlFor="approval-comment" className="sr-only">
              What needs to change
            </label>
            <Textarea
              id="approval-comment"
              value={comment}
              rows={3}
              autoFocus
              placeholder="What needs to change?"
              aria-invalid={commentError ? true : undefined}
              onChange={(e) => {
                setComment(e.target.value);
                setCommentError(null);
              }}
            />
            {commentError ? (
              <p role="alert" className="text-xs text-destructive">
                {commentError}
              </p>
            ) : null}
            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={decide.isPending}>
                Request changes
              </Button>
              <Button type="button" size="sm" variant="ghost" onClick={() => setChanges(false)}>
                Cancel
              </Button>
            </div>
          </form>
        ) : (
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              disabled={decide.isPending}
              onClick={() => decide.mutate({ id: open.id, decision: 'approved' })}
            >
              <Check className="h-4 w-4" />
              Approve
            </Button>
            <Button size="sm" variant="outline" disabled={decide.isPending} onClick={() => setChanges(true)}>
              <CornerUpLeft className="h-4 w-4" />
              Request changes
            </Button>
          </div>
        )
      ) : null}

      {open && canEdit && open.requested_by === userId ? (
        <Button size="sm" variant="ghost" disabled={cancel.isPending} onClick={() => cancel.mutate(open.id)}>
          Cancel request
        </Button>
      ) : null}

      {!open && canEdit ? (
        <form
          className="space-y-2 border-t border-border pt-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (picked.length) request.mutate();
          }}
        >
          <fieldset className="space-y-1">
            <legend className="text-xs font-medium text-muted-foreground">
              {current ? 'Request approval again' : 'Request approval from'}
            </legend>
            <div className="max-h-36 space-y-1 overflow-y-auto">
              {candidates.map((m) => (
                <label key={m.user_id} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={picked.includes(m.user_id)}
                    onChange={(e) =>
                      setPicked((p) => (e.target.checked ? [...p, m.user_id] : p.filter((x) => x !== m.user_id)))
                    }
                  />
                  {m.name || m.email}
                </label>
              ))}
            </div>
          </fieldset>
          <label className="block text-xs text-muted-foreground">
            Deadline (optional)
            <input
              type="date"
              value={deadline}
              min={new Date().toISOString().slice(0, 10)}
              onChange={(e) => setDeadline(e.target.value)}
              className="mt-1 w-full rounded-md border border-input bg-card px-3 py-2 text-sm text-foreground"
            />
          </label>
          <label className="block text-xs text-muted-foreground">
            Note (optional)
            <Textarea value={note} rows={2} onChange={(e) => setNote(e.target.value)} />
          </label>
          <Button type="submit" size="sm" disabled={!picked.length || request.isPending}>
            Request approval
          </Button>
        </form>
      ) : null}
    </div>
  );
}

function StateBadge({ approval }: { approval: Approval }) {
  if (approval.stale) {
    return (
      <Badge variant="warning" title="The task was reopened or edited after it was approved">
        Approval outdated
      </Badge>
    );
  }
  switch (approval.state) {
    case 'approved':
      return <Badge variant="success">Approved</Badge>;
    case 'changes_requested':
      return <Badge variant="destructive">Changes requested</Badge>;
    case 'cancelled':
      return <Badge variant="secondary">Cancelled</Badge>;
    default:
      return <Badge variant="outline">Waiting for approval</Badge>;
  }
}

function Deadline({ approval }: { approval: Approval }) {
  const now = useNow(false);
  const when = new Date(approval.deadline as string);
  const overdue = approval.state === 'pending' && when.getTime() < now;
  return (
    <span className={overdue ? 'text-xs font-medium text-destructive' : 'text-xs text-muted-foreground'}>
      {overdue ? 'Overdue · ' : 'Due '}
      {when.toLocaleDateString()}
    </span>
  );
}
