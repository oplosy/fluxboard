'use client';

import { use, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Pencil, Trash2 } from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { useProjectByKey } from '@/lib/board/use-project';
import { ApiError } from '@/lib/api/client';
import { deleteWikiPage, getWikiPage, updateWikiPage, type WikiPage } from '@/lib/api/wiki';
import { ProjectFrame } from '@/app/(app)/app/[orgSlug]/projects/[projectKey]/_components/project-frame';
import { DescriptionEditor } from '@/components/editor/description-editor';
import { Button } from '@/components/ui/button';
import { useToast } from '@/components/ui/toast';

const inputClass =
  'w-full rounded-lg border border-input bg-card px-3 py-2 text-lg font-semibold outline-none transition-[border-color,box-shadow] duration-300 hover:border-foreground/40 focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))]';

// One wiki page (FR-PROJ-013): rendered Markdown, edited in place. Every save
// carries the version that was read; if someone saved first the server answers
// 409 and the draft stays in the editor.
export default function WikiPageView(props: { params: Promise<{ projectKey: string; pageId: string }> }) {
  const params = use(props.params);
  const { orgId, slug, role } = useOrg();
  const router = useRouter();
  const qc = useQueryClient();
  const { toast } = useToast();
  const { project, isLoading, isError } = useProjectByKey(params.projectKey);
  const canEdit = role !== 'GUEST'; // the server still gates every write by project role
  const docsHref = `/app/${slug}/projects/${params.projectKey}/docs`;

  const pageKey = ['wiki-page', project?.id, params.pageId] as const;
  const pageQ = useQuery({
    queryKey: pageKey,
    queryFn: () => getWikiPage(orgId, project!.id, params.pageId),
    enabled: Boolean(project),
  });
  const [editingTitle, setEditingTitle] = useState<string | null>(null);

  const refresh = () => {
    qc.invalidateQueries({ queryKey: ['wiki', project?.id] });
    qc.invalidateQueries({ queryKey: ['wiki-mentions', project?.id] });
  };
  const save = useMutation({
    mutationFn: (v: { title: string; body: string }) =>
      updateWikiPage(orgId, project!.id, params.pageId, { ...v, version: pageQ.data!.version }),
    onSuccess: (page: WikiPage) => {
      qc.setQueryData(pageKey, page);
      refresh();
    },
    onError: (err) =>
      toast({
        title:
          err instanceof ApiError && err.status === 409
            ? 'This page changed since you opened it. Copy your text, reload and apply it again.'
            : err instanceof ApiError && err.status === 403
              ? 'You can’t edit this page'
              : 'Couldn’t save the page',
        variant: 'error',
      }),
  });
  const remove = useMutation({
    mutationFn: () => deleteWikiPage(orgId, project!.id, params.pageId),
    onSuccess: () => {
      refresh();
      router.push(docsHref);
    },
    onError: (err) =>
      toast({
        title:
          err instanceof ApiError && err.status === 403
            ? 'Only project leads or the author can delete a page'
            : 'Couldn’t delete the page',
        variant: 'error',
      }),
  });

  if (isLoading || pageQ.isLoading) return <p className="px-6 py-8 text-sm text-muted-foreground">Loading page…</p>;
  if (isError || !project) return <p className="px-6 py-8 text-sm text-destructive">Couldn&apos;t load this project.</p>;
  const page = pageQ.data;

  return (
    <ProjectFrame project={project} narrow>
      <Link href={docsHref} className="mb-4 inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-3.5 w-3.5" /> All pages
      </Link>
      {!page ? (
        <p className="text-sm text-destructive">This page doesn&apos;t exist, or you can&apos;t see it.</p>
      ) : (
        <div className="space-y-4">
          <div className="flex flex-wrap items-start gap-3">
            {editingTitle !== null ? (
              <form
                className="flex min-w-0 flex-1 basis-64 flex-wrap gap-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (!editingTitle.trim()) return;
                  save.mutate({ title: editingTitle.trim(), body: page.body ?? '' }, { onSuccess: () => setEditingTitle(null) });
                }}
              >
                <input
                  autoFocus
                  aria-label="Page title"
                  value={editingTitle}
                  onChange={(e) => setEditingTitle(e.target.value)}
                  maxLength={200}
                  className={`min-w-0 flex-1 ${inputClass}`}
                />
                <Button size="sm" type="submit" disabled={!editingTitle.trim() || save.isPending}>
                  Save
                </Button>
                <Button size="sm" type="button" variant="ghost" onClick={() => setEditingTitle(null)}>
                  Cancel
                </Button>
              </form>
            ) : (
              <h2 className="min-w-0 flex-1 basis-64 break-words text-xl font-bold tracking-tight">{page.title}</h2>
            )}
            {canEdit && editingTitle === null ? (
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => setEditingTitle(page.title)}
                  aria-label="Rename page"
                  className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground"
                >
                  <Pencil className="h-3.5 w-3.5" />
                </button>
                <button
                  type="button"
                  onClick={() => {
                    if (window.confirm(`Delete "${page.title}"? This can’t be undone.`)) remove.mutate();
                  }}
                  aria-label="Delete page"
                  className="rounded-md p-1.5 text-muted-foreground hover:bg-secondary hover:text-destructive"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
            ) : null}
          </div>
          <p className="text-xs text-muted-foreground">Updated {new Date(page.updated_at).toLocaleString()}</p>
          <DescriptionEditor
            key={page.id}
            value={page.body ?? ''}
            canEdit={canEdit}
            placeholder="Write the page…"
            taskRefs={{
              key: project.key,
              numbers: new Set((page.tasks ?? []).map((t) => t.number)),
              href: (n) => `/app/${slug}/projects/${params.projectKey}/tasks/${n}`,
            }}
            onSave={(body) => save.mutateAsync({ title: page.title, body })}
          />
        </div>
      )}
    </ProjectFrame>
  );
}
