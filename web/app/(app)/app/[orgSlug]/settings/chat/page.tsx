'use client';

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field } from '@/components/auth/field';
import { useOrg } from '@/lib/org/context';
import { useToast } from '@/components/ui/toast';
import { ApiError } from '@/lib/api/client';
import {
  CHAT_CATEGORIES,
  createChatChannel,
  deleteChatChannel,
  listChatChannels,
  listChatDeliveries,
  testChatChannel,
  updateChatChannel,
  type ChatChannel,
  type ChatDelivery,
  type ChatProvider,
} from '@/lib/api/chat';

const PROVIDERS: Record<ChatProvider, { label: string; placeholder: string }> = {
  slack: { label: 'Slack', placeholder: 'https://hooks.slack.com/services/…' },
  teams: { label: 'Microsoft Teams', placeholder: 'https://….webhook.office.com/… or https://….logic.azure.com/…' },
};

function CategoryPicker({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <div className="flex flex-wrap gap-2">
      {CHAT_CATEGORIES.map((c) => {
        const on = value.includes(c.id);
        return (
          <button
            key={c.id}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(on ? value.filter((x) => x !== c.id) : [...value, c.id])}
            className={
              'rounded-full border px-3 py-1 text-xs transition-colors ' +
              (on ? 'border-primary bg-secondary font-medium' : 'text-muted-foreground hover:bg-secondary/40')
            }
          >
            {c.label}
          </button>
        );
      })}
    </div>
  );
}

export default function ChatSettingsPage() {
  const { orgId } = useOrg();
  const { toast } = useToast();
  const queryClient = useQueryClient();
  const channelsKey = ['chat-channels', orgId] as const;
  const deliveriesKey = ['chat-deliveries', orgId] as const;

  const channels = useQuery({ queryKey: channelsKey, queryFn: () => listChatChannels(orgId) });
  // Deliveries settle asynchronously in the worker, so keep refreshing while any is pending.
  const deliveries = useQuery({
    queryKey: deliveriesKey,
    queryFn: () => listChatDeliveries(orgId),
    refetchInterval: (q) => (q.state.data?.some((d) => d.status === 'pending') ? 3000 : false),
  });
  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: channelsKey });
    queryClient.invalidateQueries({ queryKey: deliveriesKey });
  };

  const [creating, setCreating] = useState(false);
  const [provider, setProvider] = useState<ChatProvider>('slack');
  const [name, setName] = useState('');
  const [url, setUrl] = useState('');
  const [categories, setCategories] = useState<string[]>(['mention', 'task_assigned']);
  const [formError, setFormError] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () => createChatChannel(orgId, { provider, name: name.trim(), url: url.trim(), categories }),
    onSuccess: () => {
      setCreating(false);
      setName('');
      setUrl('');
      setFormError(null);
      refresh();
      toast({ title: 'Channel connected', variant: 'success' });
    },
    onError: (err) =>
      setFormError(err instanceof ApiError && err.status === 422 ? err.message : 'Couldn’t connect the channel.'),
  });

  const update = useMutation({
    mutationFn: (v: { c: ChatChannel; categories?: string[]; enabled?: boolean }) =>
      updateChatChannel(orgId, v.c.id, {
        name: v.c.name,
        categories: v.categories ?? v.c.categories,
        enabled: v.enabled ?? v.c.enabled,
      }),
    onSuccess: refresh,
    onError: () => toast({ title: 'Couldn’t update the channel', variant: 'error' }),
  });

  const test = useMutation({
    mutationFn: (id: string) => testChatChannel(orgId, id),
    onSuccess: () => {
      refresh();
      toast({ title: 'Test message queued', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t queue the test message', variant: 'error' }),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteChatChannel(orgId, id),
    onSuccess: () => {
      refresh();
      toast({ title: 'Channel removed', variant: 'success' });
    },
    onError: () => toast({ title: 'Couldn’t remove the channel', variant: 'error' }),
  });

  const items = channels.data ?? [];

  return (
    <div className="max-w-2xl space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-muted-foreground">Chat notifications</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Post notifications to a Slack or Microsoft Teams channel through an incoming webhook.
          </p>
        </div>
        {!creating ? (
          <Button size="sm" onClick={() => setCreating(true)}>
            Connect channel
          </Button>
        ) : null}
      </div>

      {creating ? (
        <form
          className="space-y-4 rounded-md border bg-card p-4"
          onSubmit={(e) => {
            e.preventDefault();
            setFormError(null);
            if (name.trim() && url.trim()) create.mutate();
          }}
        >
          <div className="space-y-1.5">
            <span className="text-sm font-medium">Service</span>
            <div className="grid gap-2 sm:grid-cols-2">
              {(Object.keys(PROVIDERS) as ChatProvider[]).map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setProvider(p)}
                  className={
                    'rounded-md border p-3 text-left text-sm transition-colors ' +
                    (provider === p ? 'border-primary bg-secondary/50' : 'hover:bg-secondary/40')
                  }
                >
                  <span className="font-medium">{PROVIDERS[p].label}</span>
                </button>
              ))}
            </div>
          </div>
          <Field id="chat-name" label="Name">
            <Input id="chat-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="#engineering" required />
          </Field>
          <Field id="chat-url" label="Webhook URL">
            <Input
              id="chat-url"
              type="password"
              autoComplete="off"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={PROVIDERS[provider].placeholder}
              required
            />
          </Field>
          <p className="-mt-2 text-xs text-muted-foreground">
            The URL is a secret: it is stored encrypted and can’t be shown again.
          </p>
          <div className="space-y-1.5">
            <span className="text-sm font-medium">Send these notifications</span>
            <CategoryPicker value={categories} onChange={setCategories} />
          </div>
          {formError ? (
            <p role="alert" className="text-sm text-destructive">
              {formError}
            </p>
          ) : null}
          <div className="flex gap-2">
            <Button type="submit" disabled={create.isPending || !name.trim() || !url.trim()}>
              {create.isPending ? 'Connecting…' : 'Connect'}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => {
                setCreating(false);
                setFormError(null);
              }}
            >
              Cancel
            </Button>
          </div>
        </form>
      ) : null}

      {channels.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading channels…</p>
      ) : items.length === 0 ? (
        <p className="rounded-md border bg-secondary/30 p-4 text-sm text-muted-foreground">No chat channels yet.</p>
      ) : (
        <div className="divide-y rounded-md border">
          {items.map((c) => (
            <ChannelRow
              key={c.id}
              channel={c}
              busy={update.isPending || test.isPending || remove.isPending}
              onToggle={() => update.mutate({ c, enabled: !c.enabled })}
              onCategories={(next) => update.mutate({ c, categories: next })}
              onTest={() => test.mutate(c.id)}
              onRemove={() => remove.mutate(c.id)}
            />
          ))}
        </div>
      )}

      <div className="space-y-2">
        <h3 className="text-sm font-semibold text-muted-foreground">Recent deliveries</h3>
        <DeliveryList items={deliveries.data ?? []} loading={deliveries.isLoading} />
      </div>
    </div>
  );
}

function ChannelRow({
  channel,
  busy,
  onToggle,
  onCategories,
  onTest,
  onRemove,
}: {
  channel: ChatChannel;
  busy: boolean;
  onToggle: () => void;
  onCategories: (next: string[]) => void;
  onTest: () => void;
  onRemove: () => void;
}) {
  const [confirming, setConfirming] = useState(false);
  return (
    <div className={'space-y-3 p-3' + (channel.enabled ? '' : ' opacity-70')}>
      <div className="flex flex-wrap items-center gap-3">
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-medium">
            {channel.name}
            <span className="ml-2 text-xs font-normal text-muted-foreground">{PROVIDERS[channel.provider].label}</span>
            {channel.enabled ? null : <span className="ml-2 text-xs text-muted-foreground">Paused</span>}
          </p>
          <p className="truncate font-mono text-xs text-muted-foreground">{channel.url_hint}</p>
        </div>
        <div className="flex items-center gap-3 text-sm">
          <button onClick={onTest} disabled={busy} className="hover:underline disabled:opacity-50">
            Send test
          </button>
          <button onClick={onToggle} disabled={busy} className="hover:underline disabled:opacity-50">
            {channel.enabled ? 'Pause' : 'Resume'}
          </button>
          {confirming ? (
            <span className="flex items-center gap-1 text-xs">
              <button
                onClick={() => {
                  onRemove();
                  setConfirming(false);
                }}
                className="rounded bg-destructive px-2 py-1 font-medium text-destructive-foreground"
              >
                Remove
              </button>
              <button onClick={() => setConfirming(false)} className="px-1 text-muted-foreground hover:text-foreground">
                Cancel
              </button>
            </span>
          ) : (
            <button onClick={() => setConfirming(true)} disabled={busy} className="text-destructive hover:underline disabled:opacity-50">
              Remove
            </button>
          )}
        </div>
      </div>
      <CategoryPicker value={channel.categories} onChange={onCategories} />
    </div>
  );
}

const STATUS_STYLE: Record<ChatDelivery['status'], string> = {
  pending: 'text-muted-foreground',
  sent: 'text-emerald-600',
  failed: 'text-destructive',
};

function DeliveryList({ items, loading }: { items: ChatDelivery[]; loading: boolean }) {
  if (loading) return <p className="text-sm text-muted-foreground">Loading deliveries…</p>;
  if (items.length === 0) {
    return <p className="rounded-md border bg-secondary/30 p-4 text-sm text-muted-foreground">Nothing delivered yet.</p>;
  }
  return (
    <ul className="divide-y rounded-md border">
      {items.map((d) => (
        <li key={d.id} className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 p-3 text-sm">
          <span className={'w-16 shrink-0 text-xs font-medium capitalize ' + STATUS_STYLE[d.status]}>{d.status}</span>
          <span className="min-w-0 flex-1 truncate">
            {d.title}
            <span className="ml-2 text-xs text-muted-foreground">
              {d.channel_name} · {d.category}
            </span>
          </span>
          <time className="text-xs text-muted-foreground" dateTime={d.created_at}>
            {new Date(d.created_at).toLocaleString()}
          </time>
          {d.last_error ? (
            <p className="w-full pl-[4.75rem] text-xs text-muted-foreground">
              {d.last_error} · {d.attempts} {d.attempts === 1 ? 'attempt' : 'attempts'}
            </p>
          ) : null}
        </li>
      ))}
    </ul>
  );
}
