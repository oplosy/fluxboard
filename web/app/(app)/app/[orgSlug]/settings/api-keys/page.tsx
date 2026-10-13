'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/auth/field';
import { SecretReveal } from '@/components/org/secret-reveal';
import { useOrg } from '@/lib/org/context';
import { useToast } from '@/components/ui/toast';
import { createApiKey, listApiKeys, revokeApiKey } from '@/lib/api/apikeys';
import type { ApiKey, ApiKeyScope, CreateApiKeyResult } from '@/lib/api/types';

export default function ApiKeysSettingsPage() {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const keysKey = ['api-keys', orgId] as const;

  const keys = useQuery({ queryKey: keysKey, queryFn: () => listApiKeys(orgId) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keysKey });

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [scope, setScope] = useState<'read' | 'write'>('read');
  const [reveal, setReveal] = useState<CreateApiKeyResult | null>(null);

  const create = useMutation({
    mutationFn: () => {
      const scopes: ApiKeyScope[] = scope === 'write' ? ['write'] : ['read'];
      return createApiKey(orgId, { name: name.trim(), scopes });
    },
    onSuccess: (res) => {
      setReveal(res);
      setCreating(false);
      setName('');
      setScope('read');
      invalidate();
    },
    onError: () => toast({ title: 'Couldn’t create key', variant: 'error' }),
  });

  const revoke = useMutation({
    mutationFn: (id: string) => revokeApiKey(orgId, id),
    onSuccess: () => {
      invalidate();
      toast({ title: 'Key revoked', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t revoke key', variant: 'error' }),
  });

  const items = keys.data ?? [];

  return (
    <div className="max-w-2xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold uppercase tracking-wide text-muted-foreground">API keys</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Programmatic access with a <span className="font-mono">Bearer fbk_live_…</span> token.
          </p>
        </div>
        {!creating ? <Button size="sm" onClick={() => setCreating(true)}>Create key</Button> : null}
      </div>

      {reveal ? <SecretReveal secret={reveal.secret} onDismiss={() => setReveal(null)} /> : null}

      {creating ? (
        <form
          className="space-y-4 rounded-md border bg-card p-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (name.trim()) create.mutate();
          }}
        >
          <Field id="key-name" label="Name">
            <Input
              id="key-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="CI pipeline"
              required
            />
          </Field>
          <div className="space-y-1.5">
            <span className="text-sm font-medium">Scope</span>
            <div className="grid gap-2 sm:grid-cols-2">
              {(
                [
                  { v: 'read', label: 'Read only', hint: 'GET requests only.' },
                  { v: 'write', label: 'Read + write', hint: 'Full read/write access.' },
                ] as const
              ).map(({ v, label, hint }) => (
                <button
                  key={v}
                  type="button"
                  onClick={() => setScope(v)}
                  className={
                    'rounded-md border p-3 text-left text-sm transition-colors ' +
                    (scope === v ? 'border-primary bg-secondary/50' : 'hover:bg-secondary/40')
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
              {create.isPending ? 'Creating…' : 'Create key'}
            </Button>
            <Button type="button" variant="ghost" onClick={() => setCreating(false)}>
              Cancel
            </Button>
          </div>
        </form>
      ) : null}

      {keys.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading keys…</p>
      ) : items.length === 0 ? (
        <p className="rounded-md border bg-secondary/30 p-4 text-sm text-muted-foreground">No API keys yet.</p>
      ) : (
        <div className="divide-y rounded-md border">
          {items.map((k) => (
            <KeyRow key={k.id} apiKey={k} onRevoke={() => revoke.mutate(k.id)} busy={revoke.isPending} />
          ))}
        </div>
      )}
    </div>
  );
}

function KeyRow({ apiKey, onRevoke, busy }: { apiKey: ApiKey; onRevoke: () => void; busy: boolean }) {
  const [confirming, setConfirming] = useState(false);
  const revoked = Boolean(apiKey.revoked_at);

  return (
    <div className={'flex flex-wrap items-center gap-3 p-3' + (revoked ? ' opacity-50' : '')}>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {apiKey.name}
          {revoked ? <span className="ml-2 text-xs text-destructive">Revoked</span> : null}
        </p>
        <p className="truncate font-mono text-xs text-muted-foreground">
          {apiKey.prefix} · {apiKey.scopes.join('+')} ·{' '}
          {apiKey.last_used_at
            ? `last used ${new Date(apiKey.last_used_at).toLocaleDateString()}`
            : 'never used'}
        </p>
      </div>
      {revoked ? null : confirming ? (
        <span className="flex items-center gap-1 text-xs">
          <button
            onClick={() => {
              onRevoke();
              setConfirming(false);
            }}
            className="rounded bg-destructive px-2 py-1 font-medium text-destructive-foreground"
          >
            Revoke
          </button>
          <button onClick={() => setConfirming(false)} className="px-1 text-muted-foreground hover:text-foreground">
            Cancel
          </button>
        </span>
      ) : (
        <button
          onClick={() => setConfirming(true)}
          disabled={busy}
          className="text-sm text-destructive hover:underline disabled:opacity-50"
        >
          Revoke
        </button>
      )}
    </div>
  );
}
