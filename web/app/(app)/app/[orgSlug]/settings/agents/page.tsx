'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bot, Check, Copy } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field, FormError } from '@/components/auth/field';
import { SecretReveal, useCopy } from '@/components/org/secret-reveal';
import { useToast } from '@/components/ui/toast';
import { useUpgradeModal } from '@/components/upgrade-modal';
import { useOrg } from '@/lib/org/context';
import { API_BASE, ApiError } from '@/lib/api/client';
import {
  createAgent,
  createAgentKey,
  deleteAgent,
  listAgents,
  revokeAgentKey,
  updateAgent,
} from '@/lib/api/agents';
import { limitLabel } from '@/lib/billing/plans';
import type { Agent, AgentRole, AgentSecretResult } from '@/lib/api/types';

// Mirrors domain/agent.ValidName: 1–40 letters, digits, spaces, . - _
const NAME_RE = /^[\p{L}\p{N} ._-]{1,40}$/u;

const ROLES: { v: AgentRole; label: string; hint: string }[] = [
  { v: 'MEMBER', label: 'Member', hint: 'Sees org projects. Add it to a project to let it create and edit tasks.' },
  { v: 'GUEST', label: 'Guest', hint: 'Sees only the projects you add it to.' },
];

/** Absolute MCP endpoint, also when the API base is a relative path behind a proxy. */
function mcpURL(): string {
  try {
    return new URL(`${API_BASE}/mcp`, window.location.origin).toString();
  } catch {
    return `${API_BASE}/mcp`;
  }
}

function errorText(err: unknown): string {
  return err instanceof ApiError ? err.message : 'Something went wrong. Please try again.';
}

export default function AgentsSettingsPage() {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const upgrade = useUpgradeModal();
  const queryClient = useQueryClient();
  const agentsKey = ['agents', orgId] as const;

  const agents = useQuery({ queryKey: agentsKey, queryFn: () => listAgents(orgId) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: agentsKey });

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [role, setRole] = useState<AgentRole>('MEMBER');
  const [nameError, setNameError] = useState<string | undefined>();
  const [reveal, setReveal] = useState<AgentSecretResult | null>(null);

  const create = useMutation({
    mutationFn: () => createAgent(orgId, { name: name.trim(), role }),
    onSuccess: (res) => {
      setReveal(res);
      setCreating(false);
      setName('');
      setRole('MEMBER');
      invalidate();
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 402) {
        upgrade.open('max_agents');
        return;
      }
      if (err instanceof ApiError && err.fieldErrors().name) {
        setNameError(err.fieldErrors().name);
        return;
      }
      toast({ title: 'Couldn’t create agent', description: errorText(err), variant: 'error' });
    },
  });

  function submit() {
    const n = name.trim();
    if (!NAME_RE.test(n)) {
      setNameError('Use 1–40 letters, digits, spaces, dots, dashes or underscores.');
      return;
    }
    setNameError(undefined);
    create.mutate();
  }

  const list = agents.data?.agents ?? [];
  const limit = agents.data?.limit;
  const atCap = limit ? limit.max >= 0 && limit.current >= limit.max : false;

  return (
    <div className="max-w-2xl space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-muted-foreground">Agents</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Automated teammates that work this organization over MCP with their own key. They show up by name on tasks
            and in activity, and don’t take a seat.
          </p>
          {limit ? (
            <p className="mt-2 text-xs text-muted-foreground">
              {limit.max < 0
                ? `${limit.current} ${limit.current === 1 ? 'agent' : 'agents'} · unlimited on your plan`
                : `${limit.current} of ${limitLabel(limit.max)} agents used on your plan`}
              {atCap ? ' · paused agents count too' : ''}
            </p>
          ) : null}
        </div>
        {!creating ? (
          <Button size="sm" onClick={() => (atCap ? upgrade.open('max_agents') : setCreating(true))}>
            New agent
          </Button>
        ) : null}
      </div>

      {reveal ? (
        <SecretReveal secret={reveal.secret} onDismiss={() => setReveal(null)}>
          <ConnectSnippet name={reveal.agent.name} secret={reveal.secret} />
        </SecretReveal>
      ) : null}

      {creating ? (
        <form
          className="space-y-4 rounded-md border bg-card p-4"
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            submit();
          }}
        >
          <Field id="agent-name" label="Name" error={nameError} hint="Shown on tasks and in activity, e.g. backend-2.">
            <Input
              id="agent-name"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                if (nameError) setNameError(undefined);
              }}
              placeholder="backend-2"
              maxLength={40}
              autoFocus
            />
          </Field>
          <div className="space-y-1.5">
            <span className="text-sm font-medium">Role</span>
            <div className="grid gap-2 sm:grid-cols-2">
              {ROLES.map(({ v, label, hint }) => (
                <button
                  key={v}
                  type="button"
                  aria-pressed={role === v}
                  onClick={() => setRole(v)}
                  className={
                    'rounded-md border p-3 text-left text-sm transition-colors ' +
                    (role === v ? 'border-primary bg-secondary/50' : 'hover:bg-secondary/40')
                  }
                >
                  <span className="font-medium">{label}</span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">{hint}</span>
                </button>
              ))}
            </div>
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={create.isPending || !name.trim()}>
              {create.isPending ? 'Creating…' : 'Create agent'}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setCreating(false);
                setNameError(undefined);
              }}
            >
              Cancel
            </Button>
          </div>
        </form>
      ) : null}

      {agents.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading agents…</p>
      ) : agents.isError ? (
        <FormError message="Couldn’t load agents." />
      ) : list.length === 0 ? (
        <div className="rounded-md border bg-secondary/30 p-4 text-sm text-muted-foreground">
          No agents yet. Create one, give its key to your MCP client, and it can read the board and plan work.
        </div>
      ) : (
        <div className="divide-y rounded-md border">
          {list.map((a) => (
            <AgentRow key={a.id} agent={a} orgId={orgId} onChanged={invalidate} onSecret={setReveal} />
          ))}
        </div>
      )}
    </div>
  );
}

function ConnectSnippet({ name, secret }: { name: string; secret: string }) {
  const [copied, copy] = useCopy();
  const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'agent';
  const config = JSON.stringify(
    { mcpServers: { [`fluxboard-${slug}`]: { type: 'http', url: mcpURL(), headers: { Authorization: `Bearer ${secret}` } } } },
    null,
    2,
  );
  return (
    <div className="space-y-1.5 pt-1">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-muted-foreground">MCP client config</p>
        <button
          type="button"
          onClick={() => copy(config)}
          className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
        >
          {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
          {copied ? 'Copied' : 'Copy config'}
        </button>
      </div>
      <pre className="overflow-x-auto rounded bg-background px-3 py-2 font-mono text-xs leading-relaxed">{config}</pre>
    </div>
  );
}

function AgentRow({
  agent,
  orgId,
  onChanged,
  onSecret,
}: {
  agent: Agent;
  orgId: string;
  onChanged: () => void;
  onSecret: (r: AgentSecretResult) => void;
}) {
  const { toast } = useToast();
  const [renaming, setRenaming] = useState(false);
  const [draft, setDraft] = useState(agent.name);
  const [renameError, setRenameError] = useState<string | undefined>();
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null);
  const paused = agent.status === 'paused';

  const fail = (title: string) => (err: unknown) => toast({ title, description: errorText(err), variant: 'error' });

  const update = useMutation({
    mutationFn: (input: Parameters<typeof updateAgent>[2]) => updateAgent(orgId, agent.id, input),
    onSuccess: () => {
      setRenaming(false);
      onChanged();
    },
    onError: (err) => {
      if (err instanceof ApiError && err.fieldErrors().name) {
        setRenameError(err.fieldErrors().name);
        return;
      }
      fail('Couldn’t update agent')(err);
    },
  });
  const addKey = useMutation({
    mutationFn: () => createAgentKey(orgId, agent.id),
    onSuccess: (res) => {
      onSecret(res);
      onChanged();
    },
    onError: fail('Couldn’t create key'),
  });
  const revoke = useMutation({
    mutationFn: (keyId: string) => revokeAgentKey(orgId, agent.id, keyId),
    onSuccess: () => {
      setConfirmRevoke(null);
      onChanged();
      toast({ title: 'Key revoked', variant: 'success' });
    },
    onError: fail('Couldn’t revoke key'),
  });
  const remove = useMutation({
    mutationFn: () => deleteAgent(orgId, agent.id),
    onSuccess: () => {
      onChanged();
      toast({ title: `${agent.name} deleted`, variant: 'success' });
    },
    onError: fail('Couldn’t delete agent'),
  });

  const lastUsed = agent.keys
    .map((k) => k.last_used_at)
    .filter((t): t is string => Boolean(t))
    .sort()
    .at(-1);

  return (
    <div className="space-y-3 p-3">
      <div className="flex flex-wrap items-center gap-3">
        <span
          className={
            'flex h-8 w-8 shrink-0 items-center justify-center rounded-md ' +
            (paused ? 'bg-secondary text-muted-foreground' : 'bg-primary/10 text-primary')
          }
          aria-hidden
        >
          <Bot className="h-4 w-4" />
        </span>
        <div className="min-w-0 flex-1">
          {renaming ? (
            <form
              noValidate
              onSubmit={(e) => {
                e.preventDefault();
                const n = draft.trim();
                if (!NAME_RE.test(n)) {
                  setRenameError('Use 1–40 letters, digits, spaces, dots, dashes or underscores.');
                  return;
                }
                update.mutate({ name: n });
              }}
            >
              <Field id={`rename-${agent.id}`} label="Name" error={renameError}>
                <Input
                  id={`rename-${agent.id}`}
                  value={draft}
                  maxLength={40}
                  autoFocus
                  onChange={(e) => {
                    setDraft(e.target.value);
                    setRenameError(undefined);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') setRenaming(false);
                  }}
                />
              </Field>
              <div className="mt-2 flex gap-2">
                <Button type="submit" size="sm" disabled={update.isPending}>
                  Save
                </Button>
                <Button type="button" size="sm" variant="ghost" onClick={() => setRenaming(false)}>
                  Cancel
                </Button>
              </div>
            </form>
          ) : (
            <>
              <p className="flex flex-wrap items-center gap-1.5 text-sm font-medium">
                <span className="truncate">{agent.name}</span>
                <Badge variant="secondary">{agent.role === 'GUEST' ? 'Guest' : 'Member'}</Badge>
                {paused ? <Badge variant="warning">Paused</Badge> : null}
              </p>
              <p className="truncate text-xs text-muted-foreground">
                {agent.keys.length === 0
                  ? 'No active key'
                  : `${agent.keys.length} ${agent.keys.length === 1 ? 'key' : 'keys'} · ${
                      lastUsed ? `last active ${new Date(lastUsed).toLocaleDateString()}` : 'never connected'
                    }`}
              </p>
            </>
          )}
        </div>
        {!renaming ? (
          <div className="flex flex-wrap items-center gap-1">
            <Button
              size="sm"
              variant="ghost"
              disabled={update.isPending}
              onClick={() => update.mutate({ status: paused ? 'active' : 'paused' })}
            >
              {paused ? 'Resume' : 'Pause'}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setDraft(agent.name);
                setRenameError(undefined);
                setRenaming(true);
              }}
            >
              Rename
            </Button>
            {confirmDelete ? (
              <span className="flex items-center gap-1 text-xs">
                <button
                  onClick={() => remove.mutate()}
                  disabled={remove.isPending}
                  className="rounded bg-destructive px-2 py-1 font-medium text-destructive-foreground disabled:opacity-50"
                >
                  Delete
                </button>
                <button
                  onClick={() => setConfirmDelete(false)}
                  className="px-1 text-muted-foreground hover:text-foreground"
                >
                  Cancel
                </button>
              </span>
            ) : (
              <Button size="sm" variant="ghost" className="text-destructive" onClick={() => setConfirmDelete(true)}>
                Delete
              </Button>
            )}
          </div>
        ) : null}
      </div>

      {confirmDelete ? (
        <p className="pl-11 text-xs text-muted-foreground">
          Its keys stop working right away. Tasks and history keep its name.
        </p>
      ) : null}

      <div className="space-y-1.5 pl-11">
        {agent.keys.map((k) => (
          <div key={k.id} className="flex flex-wrap items-center gap-2 text-xs">
            <code className="font-mono text-muted-foreground">{k.prefix}…</code>
            <span className="text-muted-foreground">
              {k.last_used_at ? `used ${new Date(k.last_used_at).toLocaleDateString()}` : 'never used'}
            </span>
            {confirmRevoke === k.id ? (
              <span className="flex items-center gap-1">
                <button
                  onClick={() => revoke.mutate(k.id)}
                  disabled={revoke.isPending}
                  className="rounded bg-destructive px-2 py-0.5 font-medium text-destructive-foreground disabled:opacity-50"
                >
                  Revoke
                </button>
                <button onClick={() => setConfirmRevoke(null)} className="px-1 text-muted-foreground hover:text-foreground">
                  Cancel
                </button>
                {agent.keys.length === 1 ? (
                  <span className="text-muted-foreground">— the agent can’t connect until it gets a new key</span>
                ) : null}
              </span>
            ) : (
              <button onClick={() => setConfirmRevoke(k.id)} className="text-destructive hover:underline">
                Revoke
              </button>
            )}
          </div>
        ))}
        {agent.keys.length < 3 ? (
          <button
            onClick={() => addKey.mutate()}
            disabled={addKey.isPending}
            className="text-xs font-medium text-primary hover:underline disabled:opacity-50"
          >
            {agent.keys.length === 0 ? 'Create key' : 'Add key (to rotate)'}
          </button>
        ) : (
          <p className="text-xs text-muted-foreground">3 keys at most — revoke one to add another.</p>
        )}
      </div>
    </div>
  );
}
