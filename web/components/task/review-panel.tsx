'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, CornerUpLeft } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/toast';
import { getBoard, reviewTask } from '@/lib/api/board';
import { ApiError } from '@/lib/api/client';
import { useOrg } from '@/lib/org/context';
import { Textarea } from '@/components/ui/textarea';

// Review decision for a task waiting in the board's review stage (ADR-033):
// a person approves it (→ done) or sends it back with what to change
// (→ in progress, same assignee). Agents hand work over but never decide.
export function ReviewPanel({
  taskId,
  projectId,
  columnId,
  canEdit,
}: {
  taskId: string;
  projectId: string;
  columnId: string;
  canEdit: boolean;
}) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const qc = useQueryClient();
  const { data: board } = useQuery({ queryKey: ['board', projectId], queryFn: () => getBoard(orgId, projectId) });
  const [sendingBack, setSendingBack] = useState(false);
  const [comment, setComment] = useState('');
  const [commentError, setCommentError] = useState<string | null>(null);

  const decide = useMutation({
    mutationFn: (decision: 'approve' | 'changes') =>
      reviewTask(orgId, taskId, { decision, comment: decision === 'changes' ? comment.trim() : undefined }),
    onSuccess: (res, decision) => {
      setSendingBack(false);
      setComment('');
      qc.invalidateQueries({ queryKey: ['task', orgId, taskId] });
      qc.invalidateQueries({ queryKey: ['board', projectId] });
      qc.invalidateQueries({ queryKey: ['activity', orgId, taskId] });
      qc.invalidateQueries({ queryKey: ['comments', orgId, taskId] });
      toast({
        title: decision === 'approve' ? 'Approved' : 'Sent back',
        description: `Moved to ${res.column}.`,
        variant: 'success',
      });
    },
    onError: (err) => {
      const fieldMsg = err instanceof ApiError ? err.fieldErrors().comment : undefined;
      if (fieldMsg) {
        setCommentError(fieldMsg);
        return;
      }
      toast({
        title: 'Couldn’t record the review',
        description: err instanceof ApiError ? err.message : undefined,
        variant: 'error',
      });
    },
  });

  const inReview = board?.columns.find((c) => c.id === columnId)?.role === 'review';
  if (!inReview || !canEdit) return null;

  return (
    <section className="space-y-3 rounded-md border border-primary/30 bg-primary/5 p-3">
      <div>
        <p className="text-sm font-medium">Waiting for review</p>
        <p className="text-xs text-muted-foreground">Check the work, then approve it or say what to change.</p>
      </div>
      {sendingBack ? (
        <form
          className="space-y-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (!comment.trim()) {
              setCommentError('Say what needs to change so the assignee can pick it up.');
              return;
            }
            decide.mutate('changes');
          }}
        >
          <label htmlFor="review-comment" className="sr-only">
            What needs to change
          </label>
          <Textarea
            id="review-comment"
            value={comment}
            onChange={(e) => {
              setComment(e.target.value);
              setCommentError(null);
            }}
            rows={3}
            autoFocus
            aria-invalid={commentError ? true : undefined}
            aria-describedby={commentError ? 'review-comment-error' : undefined}
            placeholder="What needs to change?"
          />
          {commentError ? (
            <p id="review-comment-error" role="alert" className="text-xs text-destructive">
              {commentError}
            </p>
          ) : null}
          <div className="flex gap-2">
            <Button type="submit" size="sm" disabled={decide.isPending}>
              Send back
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setSendingBack(false)}>
              Cancel
            </Button>
          </div>
        </form>
      ) : (
        <div className="flex flex-wrap gap-2">
          <Button size="sm" onClick={() => decide.mutate('approve')} disabled={decide.isPending}>
            <Check className="h-4 w-4" />
            Approve
          </Button>
          <Button size="sm" variant="outline" onClick={() => setSendingBack(true)} disabled={decide.isPending}>
            <CornerUpLeft className="h-4 w-4" />
            Send back
          </Button>
        </div>
      )}
    </section>
  );
}
