'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Pause, Play, Repeat } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import {
  getRecurrence,
  setRecurrence,
  stopRecurrence,
  type Recurrence,
  type RecurrenceFreq,
  type RecurrenceInput,
  type RecurrenceMode,
} from '@/lib/api/recurrence';
import { useToast } from '@/components/ui/toast';
import { Select } from '@/components/ui/select';
import { Button } from '@/components/ui/button';

const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

// Recurring tasks: the rule lives on this task; the worker copies it into the
// board's first column on each occurrence date, in the rule's timezone (the
// browser's when the rule is created). Edits affect future occurrences only.
export function TaskRecurrence({ taskId, canEdit }: { taskId: string; canEdit: boolean }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const key = ['task-recurrence', taskId] as const;

  const { data: rule, isLoading } = useQuery({ queryKey: key, queryFn: () => getRecurrence(orgId, taskId) });

  const save = useMutation({
    mutationFn: (input: RecurrenceInput | null) =>
      input ? setRecurrence(orgId, taskId, input) : stopRecurrence(orgId, taskId).then(() => null),
    onSuccess: (r) => queryClient.setQueryData(key, r),
    onError: () => toast({ title: 'Could not update repeat', variant: 'error' }),
  });

  const current = (r: Recurrence): RecurrenceInput => ({
    freq: r.freq,
    weekdays: r.weekdays,
    month_day: r.month_day ?? new Date().getDate(),
    timezone: r.timezone,
    mode: r.mode,
    paused: r.paused,
  });
  const update = (patch: Partial<RecurrenceInput>) => {
    if (rule) save.mutate({ ...current(rule), ...patch });
  };

  const setFreq = (v: string) => {
    if (!v) {
      save.mutate(null);
      return;
    }
    const now = new Date();
    const base: RecurrenceInput = rule
      ? current(rule)
      : {
          freq: 'daily',
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          mode: 'schedule',
          paused: false,
          weekdays: [],
          month_day: now.getDate(),
        };
    save.mutate({
      ...base,
      freq: v as RecurrenceFreq,
      weekdays: base.weekdays.length ? base.weekdays : [now.getDay()],
    });
  };

  if (isLoading) return <span className="text-sm text-muted-foreground">…</span>;

  if (!canEdit) {
    return (
      <span className="text-sm">
        {rule ? `${describe(rule)}${rule.paused ? ' (paused)' : ''}` : 'Does not repeat'}
      </span>
    );
  }

  return (
    <div className="space-y-2">
      <Select
        aria-label="Repeat"
        value={rule?.freq ?? ''}
        disabled={save.isPending}
        onValueChange={setFreq}
        options={[
          { value: '', label: 'Does not repeat' },
          { value: 'daily', label: 'Daily' },
          { value: 'weekly', label: 'Weekly' },
          { value: 'monthly', label: 'Monthly' },
        ]}
      />
      {rule ? (
        <>
          {rule.freq === 'weekly' ? (
            <div className="flex flex-wrap gap-1" role="group" aria-label="Repeat on">
              {WEEKDAYS.map((name, d) => {
                const on = rule.weekdays.includes(d);
                return (
                  <button
                    key={name}
                    type="button"
                    aria-pressed={on}
                    // At least one weekday stays selected.
                    disabled={save.isPending || (on && rule.weekdays.length === 1)}
                    onClick={() =>
                      update({
                        weekdays: on ? rule.weekdays.filter((x) => x !== d) : [...rule.weekdays, d].sort((a, b) => a - b),
                      })
                    }
                    className={`h-7 w-9 rounded-md border text-xs transition-colors ${
                      on ? 'border-foreground bg-foreground text-background' : 'border-input hover:border-foreground/40'
                    }`}
                  >
                    {name}
                  </button>
                );
              })}
            </div>
          ) : null}
          {rule.freq === 'monthly' ? (
            <label className="flex items-center gap-2 text-xs text-muted-foreground">
              Day
              <input
                key={rule.month_day}
                type="number"
                min={1}
                max={31}
                defaultValue={rule.month_day ?? 1}
                onBlur={(e) => {
                  const d = Number(e.target.value);
                  if (d >= 1 && d <= 31 && d !== rule.month_day) update({ month_day: d });
                }}
                className="w-16 rounded-md border border-input bg-card px-2 py-1 text-sm text-foreground outline-none focus-visible:border-foreground"
              />
              <span>(last day in shorter months)</span>
            </label>
          ) : null}
          <Select
            aria-label="Repeat from"
            size="sm"
            value={rule.mode}
            disabled={save.isPending}
            onValueChange={(v) => update({ mode: v as RecurrenceMode })}
            options={[
              { value: 'schedule', label: 'On schedule' },
              { value: 'after_completion', label: 'After completion' },
            ]}
          />
          <div className="flex items-center justify-between gap-2">
            <span className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
              <Repeat className="h-3 w-3 shrink-0" aria-hidden />
              <span className="truncate">{rule.paused ? 'Paused' : rule.timezone}</span>
            </span>
            <Button size="sm" variant="ghost" disabled={save.isPending} onClick={() => update({ paused: !rule.paused })}>
              {rule.paused ? <Play className="mr-1 h-3.5 w-3.5" /> : <Pause className="mr-1 h-3.5 w-3.5" />}
              {rule.paused ? 'Resume' : 'Pause'}
            </Button>
          </div>
        </>
      ) : null}
    </div>
  );
}

function describe(r: Recurrence): string {
  const from = r.mode === 'after_completion' ? ' after completion' : '';
  if (r.freq === 'weekly') return `Weekly on ${r.weekdays.map((d) => WEEKDAYS[d]).join(', ')}${from}`;
  if (r.freq === 'monthly') return `Monthly on day ${r.month_day}${from}`;
  return `Daily${from}`;
}
