'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { SecretReveal, useCopy } from '@/components/org/secret-reveal';
import { useToast } from '@/components/ui/toast';
import { useOrg } from '@/lib/org/context';
import { listProjects } from '@/lib/api/projects';
import {
  disconnectGithub,
  type GithubSettings,
  getGithubSettings,
  githubWebhookUrl,
  rotateGithubSecret,
  updateGithubSettings,
} from '@/lib/api/github';

// GitHub pull-request linking (FR-TASK-016). A task key such as WEB-12 in a PR
// title or branch links the PR to the task; optionally merging moves the task.
export default function GithubSettingsPage() {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const key = ['github-settings', orgId] as const;
  const [copied, copy] = useCopy();

  const settings = useQuery({ queryKey: key, queryFn: () => getGithubSettings(orgId) });

  const [secret, setSecret] = useState<string | null>(null);

  const rotate = useMutation({
    mutationFn: () => rotateGithubSecret(orgId),
    onSuccess: (s) => {
      setSecret(s);
      queryClient.invalidateQueries({ queryKey: key });
    },
    onError: () => toast({ title: 'Couldn’t generate a secret', variant: 'error' }),
  });

  const disconnect = useMutation({
    mutationFn: () => disconnectGithub(orgId),
    onSuccess: () => {
      setSecret(null);
      queryClient.invalidateQueries({ queryKey: key });
      toast({ title: 'GitHub disconnected', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t disconnect', variant: 'error' }),
  });

  const data = settings.data;
  const url = data ? githubWebhookUrl(data.webhook_path) : '';

  return (
    <div className="max-w-2xl space-y-6">
      <div>
        <h2 className="text-sm font-semibold text-muted-foreground">GitHub</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Put a task key such as <span className="font-mono">WEB-12</span> in a pull request title
          or branch name to link it to that task. Linked pull requests show in the task detail.
        </p>
      </div>

      {settings.isLoading || !data ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : (
        <>
          <section className="space-y-3 rounded-md border bg-card p-4">
            <h3 className="text-sm font-semibold">1. Webhook</h3>
            <p className="text-sm text-muted-foreground">
              In your GitHub repository (or organization) open Settings → Webhooks → Add webhook.
              Use the URL below, content type <span className="font-mono">application/json</span>,
              the secret, and select only the “Pull requests” event.
            </p>
            <div className="flex items-center gap-2">
              <code
                className="min-w-0 flex-1 break-all rounded bg-secondary px-2 py-1.5 text-xs"
                data-testid="webhook-url"
              >
                {url}
              </code>
              <Button size="sm" variant="outline" onClick={() => copy(url)}>
                {copied ? 'Copied' : 'Copy'}
              </Button>
            </div>
            {secret ? <SecretReveal secret={secret} onDismiss={() => setSecret(null)} /> : null}
            <div className="flex flex-wrap gap-2">
              <Button size="sm" onClick={() => rotate.mutate()} disabled={rotate.isPending}>
                {data.configured ? 'Rotate secret' : 'Generate secret'}
              </Button>
              {data.configured ? (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => disconnect.mutate()}
                  disabled={disconnect.isPending}
                >
                  Disconnect
                </Button>
              ) : null}
            </div>
            {data.configured ? (
              <p className="text-xs text-muted-foreground">
                Rotating replaces the secret at once; update it in GitHub too. The secret is shown
                only once.
              </p>
            ) : null}
          </section>

          {data.configured ? <GithubSettingsForm initial={data} /> : null}
        </>
      )}
    </div>
  );
}

// Allow-list and move-on-merge options. State starts from the saved settings;
// the form is remounted whenever the integration is (re)connected.
function GithubSettingsForm({ initial }: { initial: GithubSettings }) {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const projects = useQuery({ queryKey: ['projects', orgId], queryFn: () => listProjects(orgId) });
  const [repos, setRepos] = useState(initial.repos.join('\n'));
  const [moveIds, setMoveIds] = useState<string[]>(initial.move_on_merge_project_ids);

  const save = useMutation({
    mutationFn: () =>
      updateGithubSettings(orgId, {
        repos: repos.split(/[\s,]+/).filter(Boolean),
        move_on_merge_project_ids: moveIds,
      }),
    onSuccess: (s) => {
      queryClient.setQueryData(['github-settings', orgId], s);
      toast({ title: 'GitHub settings saved', variant: 'success' });
    },
    onError: () =>
      toast({ title: 'Couldn’t save — repositories must look like owner/name', variant: 'error' }),
  });

  return (
    <form
      className="space-y-5 rounded-md border bg-card p-4"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate();
      }}
    >
      <div className="space-y-1.5">
        <label htmlFor="gh-repos" className="text-sm font-medium">
          Allowed repositories
        </label>
        <Textarea
          id="gh-repos"
          value={repos}
          onChange={(e) => setRepos(e.target.value)}
          placeholder={'acme/web\nacme/api'}
          rows={3}
        />
        <p className="text-xs text-muted-foreground">
          One owner/name per line. Leave empty to accept any.
        </p>
      </div>

      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">Move to done when the pull request merges</legend>
        {(projects.data ?? []).length === 0 ? (
          <p className="text-sm text-muted-foreground">No projects yet.</p>
        ) : (
          (projects.data ?? []).map((p) => (
            <label key={p.id} className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={moveIds.includes(p.id)}
                onChange={(e) =>
                  setMoveIds((ids) =>
                    e.target.checked ? [...ids, p.id] : ids.filter((i) => i !== p.id),
                  )
                }
              />
              <span className="font-mono text-xs text-muted-foreground">{p.key}</span> {p.name}
            </label>
          ))
        )}
        <p className="text-xs text-muted-foreground">
          Tasks with open blockers stay where they are, as with automations.
        </p>
      </fieldset>

      <Button type="submit" disabled={save.isPending}>
        {save.isPending ? 'Saving…' : 'Save'}
      </Button>
    </form>
  );
}
