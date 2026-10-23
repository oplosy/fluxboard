'use client';

import { use, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery } from '@tanstack/react-query';
import { BookOpen, Plus, Search } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { createWikiPage, listWikiPages } from '@/lib/api/wiki';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { useToast } from '@/components/ui/toast';

const inputClass =
  'w-full rounded-lg border border-input bg-card px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

// Docs (FR-PROJ-013): the project's wiki. Pages are Markdown, like task
// descriptions, and link to the project's tasks by key (WEB-12).
export default function ProjectDocsPage(props: { params: Promise<{ projectKey: string }> }) {
  const params = use(props.params);
  const { orgId, slug, role } = useOrg();
  const router = useRouter();
  const { toast } = useToast();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);
  const canEdit = role !== 'GUEST'; // the server still gates every write by project role

  const [q, setQ] = useState('');
  const [title, setTitle] = useState<string | null>(null); // null = form closed
  const pagesQ = useQuery({
    queryKey: ['wiki', project?.id, q.trim()],
    queryFn: () => listWikiPages(orgId, project!.id, q.trim()),
    enabled: Boolean(project),
  });
  const create = useMutation({
    mutationFn: (t: string) => createWikiPage(orgId, project!.id, { title: t }),
    onSuccess: (page) => router.push(`/app/${slug}/projects/${params.projectKey}/docs/${page.id}`),
    onError: () => toast({ title: 'Could not create the page', variant: 'error' }),
  });

  if (isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading docs…</p>;
  if (isError || !project) return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;

  const pages = pagesQ.data ?? [];
  const submit = () => title?.trim() && create.mutate(title.trim());

  return (
    <ProjectFrame project={project} narrow>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <p className="min-w-0 flex-1 text-sm text-muted-foreground">
          Notes and specs for this project. Write a task key like {project.key}-12 and it links to the task.
        </p>
        {canEdit ? (
          <Button size="sm" className="shrink-0" onClick={() => setTitle((v) => (v === null ? '' : null))}>
            <Plus className="h-4 w-4" /> New page
          </Button>
        ) : null}
      </div>

      {title !== null ? (
        <Card className="mb-6">
          <CardContent className="flex flex-wrap gap-2 py-4">
            <input
              autoFocus
              aria-label="Page title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') submit();
              }}
              maxLength={200}
              placeholder="Page title"
              className={`min-w-0 flex-1 basis-56 ${inputClass}`}
            />
            <Button size="sm" disabled={!title.trim() || create.isPending} onClick={submit}>
              {create.isPending ? 'Creating…' : 'Create page'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setTitle(null)}>
              Cancel
            </Button>
          </CardContent>
        </Card>
      ) : null}

      <div className="relative mb-4">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
        <input
          type="search"
          aria-label="Search pages by title"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search pages by title"
          className={`pl-9 ${inputClass}`}
        />
      </div>

      {pages.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed py-12 text-center">
          <BookOpen className="h-6 w-6 text-muted-foreground" aria-hidden />
          <p className="text-sm font-medium">{q.trim() ? 'No page matches' : 'No pages yet'}</p>
          {q.trim() ? null : <p className="text-xs text-muted-foreground">Create one to document this project.</p>}
        </div>
      ) : (
        <ul className="divide-y rounded-lg border">
          {pages.map((p) => (
            <li key={p.id}>
              <Link
                href={`/app/${slug}/projects/${params.projectKey}/docs/${p.id}`}
                className="flex items-center justify-between gap-3 px-4 py-3 text-sm hover:bg-secondary/60"
              >
                <span className="min-w-0 truncate font-medium">{p.title}</span>
                <span className="shrink-0 text-xs text-muted-foreground">
                  Updated {new Date(p.updated_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </ProjectFrame>
  );
}
