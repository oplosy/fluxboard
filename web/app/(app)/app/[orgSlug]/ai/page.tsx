'use client';

import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import {
  AlertTriangle,
  BarChart3,
  Brain,
  Check,
  Copy,
  ListChecks,
  MessageSquare,
  ScrollText,
  ShieldAlert,
  Sparkles,
  type LucideIcon,
} from 'lucide-react';

import { useOrg } from '@/lib/org/context';
import { listProjects } from '@/lib/api/projects';
import { getBoard } from '@/lib/api/board';
import {
  applyPlan,
  chatTurn,
  dismissRisk,
  listRisks,
  parseTasks,
  planDraft,
  projectDigest,
  scanRisks,
} from '@/lib/api/ai';
import { ApiError } from '@/lib/api/client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';

/** Shared error block: plan limit points at billing, 403 at admins, else raw. */
function AIError({ error, slug }: { error: unknown; slug: string }) {
  if (!error) return null;
  let body: ReactNode;
  if (error instanceof ApiError && error.code === 'plan_limit_exceeded') {
    body = (
      <>
        Monthly AI budget used.{' '}
        <Link className="font-medium underline" href={`/app/${slug}/billing`}>
          Manage billing
        </Link>
      </>
    );
  } else if (error instanceof ApiError && error.code === 'forbidden') {
    body = 'AI is disabled for this workspace.';
  } else if (error instanceof ApiError) {
    body = error.message;
  } else {
    return null;
  }
  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <p className="min-w-0">{body}</p>
    </div>
  );
}

function CopyButton({ text, label = 'Copy' }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="ghost"
      size="sm"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1500);
        } catch {
          /* clipboard unavailable: ignore */
        }
      }}
    >
      {copied ? <Check className="mr-1.5 h-3.5 w-3.5" /> : <Copy className="mr-1.5 h-3.5 w-3.5" />}
      {copied ? 'Copied' : label}
    </Button>
  );
}

function ResultCard({ title, actions, children }: { title: string; actions?: ReactNode; children: ReactNode }) {
  return (
    <div className="animate-fade-in overflow-hidden rounded-lg border border-border bg-background">
      <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-1.5">
        <p className="eyebrow">{title}</p>
        {actions}
      </div>
      <div className="p-4">{children}</div>
    </div>
  );
}

function ResultSkeleton() {
  return (
    <div className="space-y-3 rounded-lg border border-border bg-background p-4" aria-busy="true" aria-label="Generating">
      <Skeleton className="h-4 w-1/3" />
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-5/6" />
      <Skeleton className="h-4 w-2/3" />
    </div>
  );
}

function ItemList({ items }: { items: string[] }) {
  return (
    <ol className="divide-y divide-border overflow-hidden rounded-md border border-border">
      {items.map((item, i) => (
        <li key={i} className="flex items-start gap-3 px-3 py-2.5 text-sm">
          <span className="mt-0.5 w-5 shrink-0 text-right font-mono text-[11px] text-muted-foreground">{i + 1}</span>
          <span className="min-w-0 flex-1 break-words">{item}</span>
        </li>
      ))}
    </ol>
  );
}

function ProjectPicker({
  projectId,
  onChange,
  label = 'Project',
}: {
  projectId: string;
  onChange: (id: string) => void;
  label?: string;
}) {
  const { orgId } = useOrg();
  const projectsQ = useQuery({ queryKey: ['projects', orgId], queryFn: () => listProjects(orgId) });
  const projects = projectsQ.data ?? [];
  return (
    <Select
      aria-label={label}
      placeholder={projectsQ.isLoading ? 'Loading projects…' : 'Select a project…'}
      value={projectId}
      onValueChange={onChange}
      options={projects.map((p) => ({ value: p.id, label: `${p.key} — ${p.name}` }))}
    />
  );
}

function Hint({ children }: { children: ReactNode }) {
  return <p className="text-xs text-muted-foreground">{children}</p>;
}

/* -------------------------------------------------------------------------- */
/* Tools                                                                       */
/* -------------------------------------------------------------------------- */

interface ChatMsg {
  role: 'you' | 'ai';
  text: string;
}

const CHAT_SUGGESTIONS = ['What is overdue this week?', 'Which tasks have no owner?', 'Summarise what changed recently'];

function ChatTool() {
  const { orgId, slug } = useOrg();
  const [draft, setDraft] = useState('');
  const [history, setHistory] = useState<ChatMsg[]>([]);
  const endRef = useRef<HTMLDivElement>(null);
  const chat = useMutation({
    mutationFn: (message: string) =>
      chatTurn(
        orgId,
        message,
        history.filter((m) => m.role === 'ai').map((m) => m.text),
      ),
    onSuccess: (res, message) => {
      setHistory((h) => [...h, { role: 'you', text: message }, { role: 'ai', text: res.text }]);
      setDraft('');
    },
  });

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }, [history.length, chat.isPending]);

  const send = (text = draft) => {
    const t = text.trim();
    if (t && !chat.isPending) chat.mutate(t);
  };

  return (
    <div className="space-y-4">
      <div className="max-h-[420px] min-h-[200px] space-y-3 overflow-y-auto rounded-lg border border-border bg-background p-4">
        {history.length === 0 && !chat.isPending ? (
          <div className="flex flex-col items-center gap-3 py-8 text-center">
            <span className="flex h-10 w-10 items-center justify-center rounded-full bg-secondary text-muted-foreground">
              <MessageSquare className="h-5 w-5" />
            </span>
            <p className="text-sm font-medium">Ask about your workspace</p>
            <div className="flex flex-wrap justify-center gap-2">
              {CHAT_SUGGESTIONS.map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => setDraft(s)}
                  className="rounded-full border border-border px-3 py-1 text-xs text-muted-foreground transition-colors hover:border-foreground/40 hover:text-foreground"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        ) : null}
        {history.map((m, i) => (
          <div key={i} className={cn('flex', m.role === 'you' ? 'justify-end' : 'justify-start')}>
            <div
              className={cn(
                'max-w-[85%] whitespace-pre-wrap break-words rounded-lg px-3.5 py-2 text-sm',
                m.role === 'you' ? 'bg-foreground text-background' : 'border border-border bg-card',
              )}
            >
              {m.text}
            </div>
          </div>
        ))}
        {chat.isPending ? (
          <div className="flex justify-start">
            <div className="w-56 space-y-2 rounded-lg border border-border bg-card px-3.5 py-3">
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-2/3" />
            </div>
          </div>
        ) : null}
        <div ref={endRef} />
      </div>

      <Textarea
        autoResize
        maxRows={6}
        maxLength={4000}
        showCount
        aria-label="Message"
        placeholder="e.g. what is overdue in Payments?"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            send();
          }
        }}
      />
      <div className="flex items-center justify-between gap-3">
        <Hint>Enter to send, Shift+Enter for a new line. Every turn is ledgered and audited.</Hint>
        <Button isLoading={chat.isPending} disabled={!draft.trim()} onClick={() => send()}>
          Send
        </Button>
      </div>
      <AIError error={chat.error} slug={slug} />
    </div>
  );
}

function ParseTool() {
  const { orgId, slug } = useOrg();
  const [input, setInput] = useState('');
  const parse = useMutation({ mutationFn: () => parseTasks(orgId, input.trim()) });

  return (
    <div className="space-y-4">
      <Textarea
        autoResize
        maxRows={10}
        maxLength={4000}
        showCount
        aria-label="Text to parse"
        placeholder="next week: finalise wireframes, kickoff design review, draft sprint demo"
        value={input}
        onChange={(e) => setInput(e.target.value)}
      />
      <div className="flex items-center justify-between gap-3">
        <Hint>One sentence in, structured task drafts out. Nothing is created.</Hint>
        <Button isLoading={parse.isPending} disabled={!input.trim()} onClick={() => parse.mutate()}>
          <Sparkles className="mr-1.5 h-4 w-4" /> Parse tasks
        </Button>
      </div>
      {parse.isPending ? <ResultSkeleton /> : null}
      {parse.data ? (
        <ResultCard
          title={`${parse.data.items.length} task draft${parse.data.items.length === 1 ? '' : 's'}`}
          actions={<CopyButton text={parse.data.items.join('\n')} />}
        >
          {parse.data.items.length > 0 ? (
            <ItemList items={parse.data.items} />
          ) : (
            <p className="text-sm text-muted-foreground">No tasks found in that text.</p>
          )}
        </ResultCard>
      ) : null}
      <AIError error={parse.error} slug={slug} />
    </div>
  );
}

function PlanTool() {
  const { orgId, slug } = useOrg();
  const [brief, setBrief] = useState('');
  const [projectId, setProjectId] = useState('');
  const [columnId, setColumnId] = useState('');
  const plan = useMutation({ mutationFn: () => planDraft(orgId, brief.trim()) });
  const boardQ = useQuery({
    queryKey: ['board', orgId, projectId],
    queryFn: () => getBoard(orgId, projectId),
    enabled: projectId !== '',
  });
  const apply = useMutation({
    mutationFn: () =>
      applyPlan(orgId, {
        project_id: projectId,
        column_id: columnId,
        items: (plan.data?.items ?? []).map((title) => ({ title })),
      }),
  });
  const columns = boardQ.data?.columns ?? [];

  return (
    <div className="space-y-4">
      <Textarea
        autoResize
        maxRows={10}
        maxLength={4000}
        showCount
        aria-label="Project brief"
        placeholder="Launch usage-based billing for EU customers…"
        value={brief}
        onChange={(e) => setBrief(e.target.value)}
      />
      <div className="flex items-center justify-between gap-3">
        <Hint>Brief in, phased draft out. Applying writes real tasks, all or nothing.</Hint>
        <Button isLoading={plan.isPending} disabled={!brief.trim()} onClick={() => plan.mutate()}>
          <Sparkles className="mr-1.5 h-4 w-4" /> Draft plan
        </Button>
      </div>
      {plan.isPending ? <ResultSkeleton /> : null}
      {plan.data ? (
        <ResultCard
          title="Plan draft"
          actions={
            <CopyButton text={[plan.data.text, ...plan.data.items.map((t, i) => `${i + 1}. ${t}`)].filter(Boolean).join('\n')} />
          }
        >
          <div className="space-y-4">
            {plan.data.text ? <p className="whitespace-pre-wrap text-sm leading-relaxed">{plan.data.text}</p> : null}
            {plan.data.items.length > 0 ? (
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <p className="eyebrow">Proposed tasks</p>
                  <Badge variant="secondary">{plan.data.items.length}</Badge>
                  {plan.data.replayed ? <Badge variant="outline">replayed</Badge> : null}
                </div>
                <ItemList items={plan.data.items} />
              </div>
            ) : null}
          </div>
        </ResultCard>
      ) : null}

      {plan.data && plan.data.items.length > 0 ? (
        <div className="space-y-3 rounded-lg border border-border p-4">
          <div>
            <p className="text-sm font-medium">Apply to a board</p>
            <Hint>Choose where the {plan.data.items.length} tasks should land.</Hint>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <ProjectPicker
              projectId={projectId}
              onChange={(id) => {
                setProjectId(id);
                setColumnId('');
              }}
            />
            <Select
              aria-label="Column"
              placeholder={projectId && boardQ.isLoading ? 'Loading columns…' : 'Select a column…'}
              value={columnId}
              onValueChange={setColumnId}
              disabled={!projectId}
              options={columns.map((c) => ({ value: c.id, label: c.name }))}
            />
          </div>
          <Button
            variant="secondary"
            isLoading={apply.isPending}
            disabled={!projectId || !columnId || Boolean(apply.data)}
            onClick={() => apply.mutate()}
          >
            Apply {plan.data.items.length} tasks
          </Button>
          {apply.data ? (
            <div className="space-y-2 rounded-md bg-success/10 p-3 text-sm">
              <p className="flex items-center gap-1.5 font-medium text-success">
                <Check className="h-4 w-4" />
                {apply.data.replayed ? 'Replayed. Nothing was duplicated.' : `Created ${apply.data.tasks.length} tasks.`}
              </p>
              <ul className="space-y-0.5 text-muted-foreground">
                {apply.data.tasks.map((t) => (
                  <li key={t.id}>
                    <span className="font-mono text-xs">#{t.number}</span> {t.title}
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <AIError error={apply.error} slug={slug} />
        </div>
      ) : null}
      <AIError error={plan.error} slug={slug} />
    </div>
  );
}

function DigestTool() {
  const { orgId, slug } = useOrg();
  const [projectId, setProjectId] = useState('');
  const digest = useMutation({ mutationFn: () => projectDigest(orgId, projectId) });
  const stats = digest.data?.stats;

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="min-w-0 sm:w-80">
          <ProjectPicker projectId={projectId} onChange={setProjectId} />
        </div>
        <Button isLoading={digest.isPending} disabled={!projectId} onClick={() => digest.mutate()}>
          <Sparkles className="mr-1.5 h-4 w-4" /> Generate digest
        </Button>
      </div>
      <Hint>Numbers come from live data. The AI only narrates.</Hint>
      {digest.isPending ? <ResultSkeleton /> : null}
      {digest.data && stats ? (
        <ResultCard title="Status digest" actions={<CopyButton text={digest.data.text} />}>
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-px overflow-hidden rounded-md border border-border bg-border sm:grid-cols-4">
              {[
                { label: 'Total', value: stats.total, bad: false },
                { label: 'Overdue', value: stats.overdue, bad: stats.overdue > 0 },
                { label: 'Unassigned', value: stats.unassigned, bad: false },
                { label: 'Urgent / high', value: stats.urgent_high, bad: false },
              ].map((s) => (
                <div key={s.label} className="bg-card px-3 py-2.5">
                  <p className="text-xs text-muted-foreground">{s.label}</p>
                  <p className={cn('font-display text-2xl font-semibold tabular-nums', s.bad && 'text-destructive')}>
                    {s.value}
                  </p>
                </div>
              ))}
            </div>
            <p className="whitespace-pre-wrap text-sm leading-relaxed">{digest.data.text}</p>
          </div>
        </ResultCard>
      ) : null}
      <AIError error={digest.error} slug={slug} />
    </div>
  );
}

const scoreVariant = { high: 'destructive', medium: 'warning', low: 'secondary' } as const;

function RisksTool() {
  const { orgId, slug } = useOrg();
  const queryClient = useQueryClient();
  const [projectId, setProjectId] = useState('');
  const risksQ = useQuery({
    queryKey: ['ai-risks', orgId, projectId],
    queryFn: () => listRisks(orgId, projectId),
    enabled: projectId !== '',
  });
  const scan = useMutation({
    mutationFn: () => scanRisks(orgId, projectId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['ai-risks', orgId, projectId] }),
  });
  const dismiss = useMutation({
    mutationFn: (taskId: string) => dismissRisk(orgId, taskId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['ai-risks', orgId, projectId] }),
  });
  const risks = risksQ.data ?? [];

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="min-w-0 sm:w-80">
          <ProjectPicker projectId={projectId} onChange={setProjectId} />
        </div>
        <Button isLoading={scan.isPending} disabled={!projectId} onClick={() => scan.mutate()}>
          <Sparkles className="mr-1.5 h-4 w-4" /> Scan now
        </Button>
      </div>
      <Hint>Overdue, ownership and capacity signals. Business plan only.</Hint>

      {!projectId ? (
        <div className="rounded-lg border border-dashed border-border px-5 py-10 text-center text-sm text-muted-foreground">
          Pick a project to see its open risks.
        </div>
      ) : risksQ.isLoading || scan.isPending ? (
        <div className="space-y-2">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-16 w-full" />
        </div>
      ) : risks.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border px-5 py-10 text-center">
          <span className="flex h-10 w-10 items-center justify-center rounded-full bg-secondary text-muted-foreground">
            <ShieldAlert className="h-5 w-5" />
          </span>
          <p className="text-sm font-medium">No open risks</p>
          <p className="text-sm text-muted-foreground">Scan to recompute signals for this project.</p>
        </div>
      ) : (
        <ul className="space-y-2">
          {risks.map((r) => (
            <li key={r.id} className="flex items-start justify-between gap-3 rounded-lg border border-border bg-background p-3">
              <div className="min-w-0 space-y-2">
                <div className="flex items-center gap-2">
                  <Badge variant={scoreVariant[r.score] ?? 'secondary'} className="uppercase">
                    {r.score}
                  </Badge>
                  <span className="font-mono text-xs text-muted-foreground">{r.task_id.slice(0, 8)}</span>
                </div>
                <ul className="space-y-1 text-sm text-muted-foreground">
                  {r.signals.map((s, i) => (
                    <li key={i} className="break-words">
                      <span className="font-mono text-xs text-foreground">{s.code}</span> {s.detail}
                    </li>
                  ))}
                </ul>
              </div>
              <Button
                variant="outline"
                size="sm"
                className="shrink-0"
                isLoading={dismiss.isPending && dismiss.variables === r.task_id}
                onClick={() => dismiss.mutate(r.task_id)}
              >
                Dismiss
              </Button>
            </li>
          ))}
        </ul>
      )}
      <AIError error={scan.error ?? dismiss.error ?? risksQ.error} slug={slug} />
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Page                                                                        */
/* -------------------------------------------------------------------------- */

interface ToolDef {
  id: string;
  name: string;
  description: string;
  icon: LucideIcon;
  render: () => ReactNode;
}

const TOOLS: ToolDef[] = [
  { id: 'chat', name: 'Assistant', description: 'Ask questions about your workspace', icon: Brain, render: () => <ChatTool /> },
  { id: 'parse', name: 'Task parser', description: 'Turn a sentence into task drafts', icon: ListChecks, render: () => <ParseTool /> },
  { id: 'plan', name: 'Plan draft', description: 'Brief in, phased plan out', icon: ScrollText, render: () => <PlanTool /> },
  { id: 'digest', name: 'Status digest', description: 'Sponsor-ready project summary', icon: BarChart3, render: () => <DigestTool /> },
  { id: 'risks', name: 'Risk review', description: 'Overdue, ownership and capacity signals', icon: ShieldAlert, render: () => <RisksTool /> },
];

export default function AIPage() {
  const [active, setActive] = useState(TOOLS[0]!.id);
  const current = TOOLS.find((t) => t.id === active) ?? TOOLS[0]!;

  return (
    <div className="mx-auto max-w-5xl px-5 py-8 lg:px-10 lg:py-10">
      <header className="mb-8">
        <p className="eyebrow">Governed AI</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-[-0.04em] sm:text-4xl">
          AI assistant<span className="text-signal">.</span>
        </h1>
        <p className="mt-2 max-w-2xl text-sm text-muted-foreground">
          Draft plans, parse tasks, summarise projects and spot risks. Every run is ledgered and audited, and nothing
          changes your boards until you apply it.
        </p>
      </header>

      <div role="tablist" aria-label="AI tools" className="mb-6 grid grid-cols-2 gap-2 md:grid-cols-3 lg:grid-cols-5">
        {TOOLS.map((t) => {
          const Icon = t.icon;
          const selected = t.id === active;
          return (
            <button
              key={t.id}
              type="button"
              role="tab"
              id={`ai-tab-${t.id}`}
              aria-selected={selected}
              aria-controls={`ai-panel-${t.id}`}
              onClick={() => setActive(t.id)}
              className={cn(
                'group flex flex-col items-start gap-2 rounded-lg border p-3 text-left transition-[border-color,background-color,box-shadow] duration-300',
                'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                selected
                  ? 'border-foreground bg-card shadow-[inset_0_-2px_0_hsl(var(--signal))]'
                  : 'border-border bg-card/50 hover:border-foreground/40 hover:bg-card',
              )}
            >
              <Icon className={cn('h-4 w-4', selected ? 'text-signal' : 'text-muted-foreground group-hover:text-foreground')} />
              <span className="text-sm font-medium leading-tight">{t.name}</span>
              <span className="text-xs leading-snug text-muted-foreground">{t.description}</span>
            </button>
          );
        })}
      </div>

      {TOOLS.map((t) => (
        <div
          key={t.id}
          role="tabpanel"
          id={`ai-panel-${t.id}`}
          aria-labelledby={`ai-tab-${t.id}`}
          hidden={t.id !== active}
        >
          <Card className="p-5 sm:p-6">
            <div className="mb-5 flex items-center gap-2">
              <t.icon className="h-4 w-4 text-muted-foreground" />
              <h2 className="text-base font-semibold tracking-[-0.02em]">{t.name}</h2>
              <span className="text-sm text-muted-foreground">· {t.description}</span>
            </div>
            {t.render()}
          </Card>
        </div>
      ))}
      <span className="sr-only">{current.name} selected</span>
    </div>
  );
}
