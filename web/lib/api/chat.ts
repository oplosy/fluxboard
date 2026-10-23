import { apiFetch } from './client';

// Slack / Teams notification channels (docs/08, FR-NTF-005, ADR-038). All
// ADMIN-only. The webhook URL is sent on create only; the API never returns it,
// just a display hint.

export type ChatProvider = 'slack' | 'teams';

/** Notification categories a channel can subscribe to (matches the prefs matrix). */
export const CHAT_CATEGORIES: { id: string; label: string }[] = [
  { id: 'task_assigned', label: 'Task assigned' },
  { id: 'mention', label: 'Mentions' },
  { id: 'comment', label: 'Comments' },
  { id: 'invite_accepted', label: 'Invitation accepted' },
  { id: 'billing', label: 'Billing' },
  { id: 'approval', label: 'Approvals' },
];

export interface ChatChannel {
  id: string;
  provider: ChatProvider;
  name: string;
  /** Host plus the last characters of the URL; never the full URL. */
  url_hint: string;
  categories: string[];
  enabled: boolean;
  created_at: string;
}

export interface ChatDelivery {
  id: string;
  channel_id: string;
  channel_name?: string;
  /** A notification category, or "test". */
  category: string;
  title: string;
  status: 'pending' | 'sent' | 'failed';
  attempts: number;
  last_error: string;
  created_at: string;
  sent_at: string | null;
}

export interface CreateChatChannelInput {
  provider: ChatProvider;
  name: string;
  url: string;
  categories: string[];
}

export async function listChatChannels(orgId: string): Promise<ChatChannel[]> {
  const res = await apiFetch<{ channels: ChatChannel[] }>(`/orgs/${orgId}/chat-channels`);
  return res.channels ?? [];
}

export async function createChatChannel(orgId: string, input: CreateChatChannelInput): Promise<ChatChannel> {
  const res = await apiFetch<{ channel: ChatChannel }>(`/orgs/${orgId}/chat-channels`, { method: 'POST', body: input });
  return res.channel;
}

export async function updateChatChannel(
  orgId: string,
  id: string,
  input: { name: string; categories: string[]; enabled: boolean },
): Promise<ChatChannel> {
  const res = await apiFetch<{ channel: ChatChannel }>(`/orgs/${orgId}/chat-channels/${id}`, {
    method: 'PATCH',
    body: input,
  });
  return res.channel;
}

export function deleteChatChannel(orgId: string, id: string): Promise<void> {
  return apiFetch(`/orgs/${orgId}/chat-channels/${id}`, { method: 'DELETE' });
}

/** Queue a test message (202). */
export async function testChatChannel(orgId: string, id: string): Promise<ChatDelivery> {
  const res = await apiFetch<{ delivery: ChatDelivery }>(`/orgs/${orgId}/chat-channels/${id}/test`, { method: 'POST' });
  return res.delivery;
}

/** Newest deliveries across the org's channels. */
export async function listChatDeliveries(orgId: string): Promise<ChatDelivery[]> {
  const res = await apiFetch<{ deliveries: ChatDelivery[] }>(`/orgs/${orgId}/chat-deliveries`);
  return res.deliveries ?? [];
}
