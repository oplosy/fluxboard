'use client';

import { useState } from 'react';

import { Button } from '@/components/ui/button';
import type { GoalInput } from '@/lib/api/goals';

export const goalInputClass =
  'w-full rounded-lg border border-input bg-card px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

// Create / edit form for a goal: title, target date, description.
export function GoalForm({
  initial,
  pending,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  initial?: { title: string; description: string; target_date: string | null };
  pending: boolean;
  submitLabel: string;
  onSubmit: (input: GoalInput) => void;
  onCancel: () => void;
}) {
  const [title, setTitle] = useState(initial?.title ?? '');
  const [day, setDay] = useState(initial?.target_date ?? '');
  const [description, setDescription] = useState(initial?.description ?? '');
  const submit = () =>
    title.trim() && onSubmit({ title: title.trim(), description: description.trim(), target_date: day || null });
  return (
    <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
      <label className="text-xs font-medium text-muted-foreground">
        Title
        <input
          autoFocus
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') submit();
          }}
          maxLength={200}
          placeholder="e.g. Launch v2"
          className={`mt-1 ${goalInputClass}`}
        />
      </label>
      <label className="text-xs font-medium text-muted-foreground">
        Target date
        <input type="date" value={day} onChange={(e) => setDay(e.target.value)} className={`mt-1 ${goalInputClass}`} />
      </label>
      <label className="text-xs font-medium text-muted-foreground sm:col-span-2">
        Description
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          maxLength={2000}
          rows={2}
          placeholder="What success looks like (optional)"
          className={`mt-1 ${goalInputClass}`}
        />
      </label>
      <div className="flex gap-2 sm:col-span-2">
        <Button size="sm" disabled={!title.trim() || pending} onClick={submit}>
          {pending ? 'Saving…' : submitLabel}
        </Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
}
