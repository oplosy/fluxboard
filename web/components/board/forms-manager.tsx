'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, Check, Copy, Link2, MoreHorizontal, Plus, RefreshCw, Trash2 } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import {
  createForm,
  deleteForm,
  listForms,
  rotateFormToken,
  updateForm,
  type IntakeForm,
} from '@/lib/api/forms';
import { getBoard } from '@/lib/api/board';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Menu, MenuItem, MenuSeparator } from '@/components/ui/menu';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { cn } from '@/lib/utils';
import { ConfirmStrip, EmptyState, RowList, SettingsSection } from './settings-section';

// Project intake-form manager (ADR-023). LEAD-only surface. The raw token is
// shown once at create/rotate (API-key-style reveal) because only its hash is
// stored — known tokens are tracked in-session for the copy button.
export function FormsManager({ projectId }: { projectId: string }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const key = ['intake-forms', projectId] as const;
  const invalidate = () => queryClient.invalidateQueries({ queryKey: key });

  const formsQ = useQuery({ queryKey: key, queryFn: () => listForms(orgId, projectId) });
  const forms = formsQ.data ?? [];
  const boardQ = useQuery({ queryKey: ['board', projectId], queryFn: () => getBoard(orgId, projectId) });
  const columns = boardQ.data?.columns ?? [];

  // formId -> raw token revealed this session (create/rotate).
  const [revealed, setRevealed] = useState<Record<string, string>>({});
  const [copied, setCopied] = useState<string | null>(null);
  const [confirmId, setConfirmId] = useState<string | null>(null);

  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [targetColumn, setTargetColumn] = useState('');

  const createMutation = useMutation({
    mutationFn: () =>
      createForm(orgId, projectId, {
        name: name.trim(),
        description: description.trim(),
        target_column_id: targetColumn || (columns[0]?.id ?? ''),
      }),
    onSuccess: ({ form, token }) => {
      invalidate();
      setRevealed((m) => ({ ...m, [form.id]: token }));
      setShowForm(false);
      setName('');
      setDescription('');
      setTargetColumn('');
      toast({ title: 'Form created — copy the link below', variant: 'success' });
    },
    onError: () => toast({ title: 'Could not create form', variant: 'error' }),
  });

  const toggleMutation = useMutation({
    mutationFn: (f: IntakeForm) =>
      updateForm(orgId, projectId, f.id, {
        name: f.name,
        description: f.description,
        target_column_id: f.target_column_id,
        is_active: !f.is_active,
      }),
    onSuccess: invalidate,
    onError: () => toast({ title: 'Could not update form', variant: 'error' }),
  });

  const rotateMutation = useMutation({
    mutationFn: (formId: string) => rotateFormToken(orgId, projectId, formId),
    onSuccess: (token, formId) => {
      setRevealed((m) => ({ ...m, [formId]: token }));
      toast({ title: 'New link issued — the old URL is dead', variant: 'success' });
    },
    onError: () => toast({ title: 'Could not rotate token', variant: 'error' }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteForm(orgId, projectId, id),
    onSuccess: () => {
      setConfirmId(null);
      invalidate();
    },
    onError: () => toast({ title: 'Could not delete form', variant: 'error' }),
  });

  function publicUrl(token: string): string {
    return `${window.location.origin}/f/${token}`;
  }

  async function copyLink(formId: string) {
    const token = revealed[formId];
    if (!token) return;
    try {
      await navigator.clipboard.writeText(publicUrl(token));
      setCopied(formId);
      setTimeout(() => setCopied((c) => (c === formId ? null : c)), 2000);
    } catch {
      toast({ title: 'Copy failed — select the link manually', variant: 'error' });
    }
  }

  const canSave = name.trim() !== '' && columns.length > 0;

  return (
    <SettingsSection
      title="Intake forms"
      description="Shareable links that file tasks into this project, no account needed. Submissions land in the form's target column."
      action={
        <Button size="sm" variant="outline" onClick={() => setShowForm((v) => !v)} aria-expanded={showForm}>
          <Plus className="h-3.5 w-3.5" /> New form
        </Button>
      }
    >
      <div className="space-y-4">
        {showForm ? (
          <form
            className="space-y-4 rounded-md border border-border bg-secondary/30 p-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (canSave) createMutation.mutate();
            }}
          >
            <Field label="Name">
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Bug reports" autoFocus />
            </Field>
            <Field label="Description" hint="Shown on the form page." optional>
              <Input
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What should submitters include?"
              />
            </Field>
            <Field label="Target column">
              <Select
                value={targetColumn || (columns[0]?.id ?? '')}
                onValueChange={setTargetColumn}
                options={columns.map((c) => ({ value: c.id, label: c.name }))}
              />
            </Field>
            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={!canSave || createMutation.isPending}>
                {createMutation.isPending ? 'Creating…' : 'Create form'}
              </Button>
              <Button type="button" size="sm" variant="ghost" onClick={() => setShowForm(false)}>
                Cancel
              </Button>
            </div>
          </form>
        ) : null}

        {formsQ.isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : forms.length === 0 ? (
          <EmptyState title="No intake forms yet">
            Create one to accept bug reports or requests from anyone with the link.
          </EmptyState>
        ) : (
          <RowList>
            {forms.map((f) => {
              const token = revealed[f.id];
              return (
                <li key={f.id}>
                  <div className="flex items-center gap-3 px-3 py-2.5">
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <p className="truncate text-sm font-medium">{f.name}</p>
                        <Badge variant={f.is_active ? 'success' : 'secondary'}>
                          {f.is_active ? 'Accepting' : 'Paused'}
                        </Badge>
                      </div>
                      <p className="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground">
                        <ArrowRight className="h-3 w-3" aria-hidden />
                        <span className="truncate">{columns.find((c) => c.id === f.target_column_id)?.name ?? 'column'}</span>
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => toggleMutation.mutate(f)}
                      disabled={toggleMutation.isPending}
                      className={cn(
                        'relative h-5 w-9 shrink-0 rounded-full transition-colors disabled:opacity-50',
                        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal focus-visible:ring-offset-2 focus-visible:ring-offset-background',
                        f.is_active ? 'bg-foreground' : 'bg-border',
                      )}
                      role="switch"
                      aria-checked={f.is_active}
                      aria-label={`${f.name}: ${f.is_active ? 'pause' : 'resume'} form`}
                    >
                      <span
                        className={cn(
                          'absolute top-0.5 h-4 w-4 rounded-full bg-background shadow transition-all',
                          f.is_active ? 'left-[1.125rem]' : 'left-0.5',
                        )}
                      />
                    </button>
                    <Menu
                      align="end"
                      aria-label={`Actions for ${f.name}`}
                      trigger={
                        <Button size="icon" variant="ghost" className="h-8 w-8" aria-label={`Actions for ${f.name}`}>
                          <MoreHorizontal className="h-4 w-4" />
                        </Button>
                      }
                    >
                      <MenuItem icon={<Copy />} disabled={!token} onSelect={() => copyLink(f.id)}>
                        Copy link
                      </MenuItem>
                      <MenuItem icon={<RefreshCw />} onSelect={() => rotateMutation.mutate(f.id)}>
                        Issue new link
                      </MenuItem>
                      <MenuSeparator />
                      <MenuItem destructive icon={<Trash2 />} onSelect={() => setConfirmId(f.id)}>
                        Delete form
                      </MenuItem>
                    </Menu>
                  </div>
                  {token ? (
                    <div className="mx-3 mb-3 flex items-center gap-2 rounded-md bg-secondary/60 px-3 py-2">
                      <Link2 className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden />
                      <code className="min-w-0 flex-1 truncate font-mono text-xs">{publicUrl(token)}</code>
                      <button
                        type="button"
                        onClick={() => copyLink(f.id)}
                        className="rounded p-1 hover:bg-secondary"
                        aria-label="Copy link"
                        title="Copy link"
                      >
                        {copied === f.id ? (
                          <Check className="h-3.5 w-3.5 text-success" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </div>
                  ) : (
                    <p className="px-3 pb-2.5 text-xs text-muted-foreground">
                      Link hidden. Issue a new one to see it again (only its hash is stored).
                    </p>
                  )}
                  {confirmId === f.id ? (
                    <ConfirmStrip
                      confirmLabel="Delete form"
                      onConfirm={() => deleteMutation.mutate(f.id)}
                      onCancel={() => setConfirmId(null)}
                      disabled={deleteMutation.isPending}
                    >
                      <span>Delete this form? Its link stops working; submitted tasks stay.</span>
                    </ConfirmStrip>
                  ) : null}
                </li>
              );
            })}
          </RowList>
        )}
      </div>
    </SettingsSection>
  );
}
