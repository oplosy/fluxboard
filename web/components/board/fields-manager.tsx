'use client';

import { useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CalendarDays, Hash, ListChecks, MoreHorizontal, Plus, Trash2, Type } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import {
  FIELD_TYPES,
  createCustomField,
  deleteCustomField,
  listCustomFields,
  type CustomFieldType,
} from '@/lib/api/fields';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Menu, MenuItem } from '@/components/ui/menu';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { ConfirmStrip, EmptyState, RowList, SettingsSection } from './settings-section';

const TYPE_ICON: Record<CustomFieldType, ReactNode> = {
  text: <Type />,
  number: <Hash />,
  date: <CalendarDays />,
  select: <ListChecks />,
};

// Project custom-field manager (FR-FIELDS). LEAD-only surface; definitions are
// typed (text/number/date/select) and ordered by position.
export function FieldsManager({ projectId }: { projectId: string }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const key = ['custom-fields', projectId] as const;
  const invalidate = () => queryClient.invalidateQueries({ queryKey: key });

  const fieldsQ = useQuery({ queryKey: key, queryFn: () => listCustomFields(orgId, projectId) });
  const fields = fieldsQ.data ?? [];

  const [showForm, setShowForm] = useState(false);
  const [name, setName] = useState('');
  const [type, setType] = useState<CustomFieldType>('text');
  const [options, setOptions] = useState('');
  const [confirmId, setConfirmId] = useState<string | null>(null);

  const createMutation = useMutation({
    mutationFn: () =>
      createCustomField(orgId, projectId, {
        name: name.trim(),
        type,
        options: options
          .split(',')
          .map((o) => o.trim())
          .filter(Boolean),
        position: fields.length,
      }),
    onSuccess: () => {
      invalidate();
      setShowForm(false);
      setName('');
      setOptions('');
    },
    onError: () => toast({ title: 'Could not create field', variant: 'error' }),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteCustomField(orgId, projectId, id),
    onSuccess: () => {
      setConfirmId(null);
      invalidate();
    },
    onError: () => toast({ title: 'Could not delete field', variant: 'error' }),
  });

  const canSave = name.trim() !== '' && (type !== 'select' || options.split(',').some((o) => o.trim() !== ''));

  return (
    <SettingsSection
      title="Custom fields"
      description="Typed attributes that tasks in this project can carry, like story points or a release date."
      action={
        <Button size="sm" variant="outline" onClick={() => setShowForm((v) => !v)} aria-expanded={showForm}>
          <Plus className="h-3.5 w-3.5" /> New field
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
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Name">
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Story points"
                  autoFocus
                />
              </Field>
              <Field label="Type">
                <Select
                  value={type}
                  onValueChange={(v) => setType(v as CustomFieldType)}
                  options={FIELD_TYPES.map((t) => ({ value: t.value, label: t.label, icon: TYPE_ICON[t.value] }))}
                />
              </Field>
            </div>
            {type === 'select' ? (
              <Field label="Options" hint="Comma separated, e.g. S, M, L">
                <Input value={options} onChange={(e) => setOptions(e.target.value)} placeholder="S, M, L" />
              </Field>
            ) : null}
            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={!canSave || createMutation.isPending}>
                {createMutation.isPending ? 'Creating…' : 'Create field'}
              </Button>
              <Button type="button" size="sm" variant="ghost" onClick={() => setShowForm(false)}>
                Cancel
              </Button>
            </div>
          </form>
        ) : null}

        {fieldsQ.isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : fields.length === 0 ? (
          <EmptyState title="No custom fields yet">
            Add a field to track extra details on every task, such as estimates or customer names.
          </EmptyState>
        ) : (
          <RowList>
            {fields.map((f) => (
              <li key={f.id}>
                <div className="flex items-center gap-3 px-3 py-2.5">
                  <span className="text-muted-foreground [&>svg]:h-4 [&>svg]:w-4">{TYPE_ICON[f.type]}</span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{f.name}</p>
                    {f.type === 'select' ? (
                      <p className="truncate text-xs text-muted-foreground">{f.options.join(', ')}</p>
                    ) : null}
                  </div>
                  <Badge variant="secondary">{FIELD_TYPES.find((t) => t.value === f.type)?.label ?? f.type}</Badge>
                  <Menu
                    align="end"
                    aria-label={`Actions for ${f.name}`}
                    trigger={
                      <Button size="icon" variant="ghost" className="h-8 w-8" aria-label={`Actions for ${f.name}`}>
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    }
                  >
                    <MenuItem destructive icon={<Trash2 />} onSelect={() => setConfirmId(f.id)}>
                      Delete field
                    </MenuItem>
                  </Menu>
                </div>
                {confirmId === f.id ? (
                  <ConfirmStrip
                    confirmLabel="Delete field"
                    onConfirm={() => deleteMutation.mutate(f.id)}
                    onCancel={() => setConfirmId(null)}
                    disabled={deleteMutation.isPending}
                  >
                    <span>Delete {f.name} and its values on every task?</span>
                  </ConfirmStrip>
                ) : null}
              </li>
            ))}
          </RowList>
        )}
      </div>
    </SettingsSection>
  );
}
