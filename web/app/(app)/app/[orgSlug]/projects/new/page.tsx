'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, FormError } from '@/components/auth/field';
import { useOrg } from '@/lib/org/context';
import { createProject, createProjectFromTemplate, listProjectTemplates } from '@/lib/api/projects';
import { ApiError } from '@/lib/api/client';
import type { ProjectTemplate, Visibility } from '@/lib/api/types';
import { cn } from '@/lib/utils';
import { Textarea } from '@/components/ui/textarea';

// A project key is 2–10 upper-case letters/digits starting with a letter
// (mirrors ValidKey server-side). We suggest one from the name until edited.
function suggestKey(name: string): string {
  const cleaned = name.toUpperCase().replace(/[^A-Z0-9]/g, '');
  const withLetterHead = cleaned.replace(/^[0-9]+/, '');
  return withLetterHead.slice(0, 6);
}

// Blank is the plain create (default board); the rest come from the server catalog.
const BLANK: ProjectTemplate = {
  key: 'blank',
  name: 'Blank',
  description: 'The default board. Shape it yourself.',
  columns: [{ name: 'Backlog' }, { name: 'Todo' }, { name: 'In Progress' }, { name: 'Review' }, { name: 'Done' }],
  labels: [],
  task_count: 0,
};

const COLORS = ['#2f7468', '#6e9f8d', '#d49a52', '#c36d58', '#8d9a65', '#5f8ea0', '#a77c91', '#64748b'];

export default function NewProjectPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { orgId, slug } = useOrg();

  const [name, setName] = useState('');
  // The key follows the name until the user edits it directly.
  const [keyDraft, setKeyDraft] = useState<string | null>(null);
  const key = keyDraft ?? suggestKey(name);
  const [description, setDescription] = useState('');
  const [color, setColor] = useState(COLORS[0] as string);
  const [visibility, setVisibility] = useState<Visibility>('org');
  const [templateKey, setTemplateKey] = useState('blank');
  // One key per visit: a retried submit replays instead of creating twice.
  const [idempotencyKey] = useState(() => crypto.randomUUID());

  const templatesQuery = useQuery({
    queryKey: ['project-templates', orgId],
    queryFn: () => listProjectTemplates(orgId),
    staleTime: Infinity,
  });
  const templates = [BLANK, ...(templatesQuery.data ?? [])];

  const mutation = useMutation({
    mutationFn: () => {
      const input = { key, name, description, color, visibility };
      if (templateKey === 'blank') {
        return createProject(orgId, input);
      }
      return createProjectFromTemplate(orgId, templateKey, input, idempotencyKey);
    },
    onSuccess: async (project) => {
      await queryClient.invalidateQueries({ queryKey: ['projects', orgId] });
      router.replace(`/app/${slug}/projects/${project.key}`);
    },
  });

  const err = mutation.error;
  const fieldErrors = err instanceof ApiError ? err.fieldErrors() : {};
  const banner =
    err instanceof ApiError
      ? err.status === 409
        ? 'That project key is already taken.'
        : err.status === 402
          ? 'Your plan has reached its project limit. Upgrade to add more.'
          : err.status !== 422
            ? err.message
            : null
      : err
        ? 'Something went wrong.'
        : null;
  const planLimited = err instanceof ApiError && err.status === 402;

  return (
    <div className="mx-auto max-w-2xl px-6 py-12 lg:py-16">
      <Card>
        <CardHeader>
          <CardTitle>New project</CardTitle>
          <CardDescription>
            Starts with a default board (To Do / In Progress / Done) you can edit later.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              mutation.mutate();
            }}
          >
            <FormError message={banner} />
            {planLimited ? (
              <Link
                href={`/app/${slug}/billing`}
                className="block text-sm text-primary hover:underline"
              >
                Go to billing →
              </Link>
            ) : null}

            <Field id="name" label="Name" error={fieldErrors.name}>
              <Input
                id="name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Website Redesign"
              />
            </Field>

            <Field id="key" label="Key" error={fieldErrors.key}>
              <Input
                id="key"
                required
                value={key}
                onChange={(e) => {
                  setKeyDraft(e.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 10));
                }}
                placeholder="WEB"
                className="font-mono uppercase"
              />
              <p className="text-xs text-muted-foreground">
                Short prefix for task numbers, e.g. {key || 'WEB'}-42. 2–10 letters/digits.
              </p>
            </Field>

            <Field id="description" label="Description (optional)">
              <Textarea
                id="description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                placeholder="What is this project about?"
              />
            </Field>

            <div className="space-y-1.5">
              <span className="text-sm font-medium">Color</span>
              <div className="flex flex-wrap gap-2">
                {COLORS.map((c) => (
                  <button
                    key={c}
                    type="button"
                    onClick={() => setColor(c)}
                    aria-label={`Select color ${c}`}
                    className={cn(
                      'h-7 w-7 rounded-full ring-offset-2 ring-offset-background transition',
                      color === c && 'ring-2 ring-ring',
                    )}
                    style={{ backgroundColor: c }}
                  />
                ))}
              </div>
            </div>

            <div className="space-y-1.5">
              <span className="text-sm font-medium">Visibility</span>
              <div className="grid gap-2 sm:grid-cols-2">
                {(
                  [
                    { v: 'org', label: 'Organization', hint: 'All org members can see it.' },
                    { v: 'private', label: 'Private', hint: 'Only project members.' },
                  ] as const
                ).map(({ v, label, hint }) => (
                  <button
                    key={v}
                    type="button"
                    onClick={() => setVisibility(v)}
                    className={cn(
                      'rounded-2xl bg-secondary/45 p-4 text-left text-sm transition-colors',
                      visibility === v ? 'bg-primary/10 text-foreground ring-1 ring-primary/30' : 'hover:bg-secondary',
                    )}
                  >
                    <span className="font-medium">{label}</span>
                    <span className="mt-0.5 block text-xs text-muted-foreground">{hint}</span>
                  </button>
                ))}
              </div>
            </div>

            <div className="space-y-1.5">
              <span className="text-sm font-medium">Template</span>
              <div className="grid gap-2 sm:grid-cols-2">
                {templates.map((t) => (
                  <button
                    key={t.key}
                    type="button"
                    onClick={() => setTemplateKey(t.key)}
                    className={cn(
                      'rounded-2xl bg-secondary/45 p-4 text-left text-sm transition-colors',
                      templateKey === t.key ? 'bg-primary/10 text-foreground ring-1 ring-primary/30' : 'hover:bg-secondary',
                    )}
                  >
                    <span className="font-medium">{t.name}</span>
                    <span className="mt-0.5 block text-xs text-muted-foreground">{t.description}</span>
                    <span className="mt-1.5 block text-[11px] text-muted-foreground">
                      {t.columns.map((c) => c.name).join(' → ')}
                    </span>
                    {t.task_count > 0 ? (
                      <span className="mt-0.5 block text-[11px] text-muted-foreground">
                        {t.labels.length} labels · {t.task_count} starter tasks
                      </span>
                    ) : null}
                  </button>
                ))}
              </div>
            </div>

            <Button type="submit" className="w-full" disabled={mutation.isPending || !name || !key}>
              {mutation.isPending ? 'Creating…' : 'Create project'}
            </Button>
          </form>

          <p className="mt-6 text-center text-sm text-muted-foreground">
            <Link href={`/app/${slug}/projects`} className="text-primary hover:underline">
              Back to projects
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
