'use client';

import { useEffect, useRef, useState } from 'react';
import { Bot, Check, Copy, GitPullRequestArrow, History, KeyRound, Lock, Timer } from 'lucide-react';

import { ScrambleText } from '@/components/motion/effects';
import { Reveal, SplitText } from '@/components/motion/reveal';
import { useInView } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

const MCP_URL = `${process.env.NEXT_PUBLIC_API_BASE ?? 'http://localhost:8080/api/v1'}/mcp`;

// ── Animated session ────────────────────────────────────────────────────

// One agent's turn through a task, as the MCP server answers it. `lane` is
// where the card sits on the mini board after the call.
const session = [
  { call: 'tasks.next', args: '{"project_id":"…"}', reply: 'claimed WEB-42 · lease 30m', lane: 0 },
  { call: 'tasks.get', args: '{"task_id":"…"}', reply: 'v7 · 2 subtasks · 0 blockers', lane: 0 },
  { call: 'tasks.update', args: '{"version":7,"column":"in_progress"}', reply: 'v8 · In progress', lane: 1 },
  { call: 'tasks.heartbeat', args: '{"task_id":"…"}', reply: 'lease renewed · 30m', lane: 1 },
  { call: 'tasks.comment', args: '{"body":"Fixed the redirect loop."}', reply: 'comment added', lane: 1 },
  { call: 'tasks.submit_for_review', args: '{"version":8}', reply: 'Review · claim released', lane: 2 },
  { call: 'tasks.update', args: '{"column":"done"}', reply: 'Only a person can mark a task done.', lane: 2, refused: true },
] as const;

const lanes = ['To do', 'In progress', 'Review', 'Done'];

function AgentSession() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: false });
  const [step, setStep] = useState(0);

  useEffect(() => {
    if (!inView) return;
    const id = window.setInterval(() => setStep((n) => (n >= session.length - 1 ? 0 : n + 1)), 1600);
    return () => window.clearInterval(id);
  }, [inView]);

  const lane = session[step]!.lane;
  return (
    <div ref={ref} className="flex h-full flex-col rounded-lg border border-border bg-card">
      <div className="flex items-center gap-1.5 border-b border-border px-3 py-2 text-xs text-muted-foreground">
        <span className="h-2 w-2 rounded-full bg-foreground/15" />
        <span className="h-2 w-2 rounded-full bg-foreground/15" />
        <span className="h-2 w-2 rounded-full bg-foreground/15" />
        <span className="ml-2 font-mono">POST /api/v1/mcp</span>
        <span className="ml-auto flex items-center gap-1.5 font-medium">
          <Bot className="h-3.5 w-3.5" strokeWidth={1.8} /> release-bot
        </span>
      </div>

      {/* The card the agent works on, moving across a four-lane board. */}
      <div className="grid grid-cols-4 gap-px border-b border-border bg-border" aria-hidden>
        {lanes.map((name, i) => (
          <div key={name} className="relative h-[86px] bg-background p-2">
            <p className="flex items-center gap-1 text-[10.5px] font-medium text-muted-foreground">
              {name}
              {i === 3 ? <Lock className="h-3 w-3" strokeWidth={2} /> : null}
            </p>
            {i === 3 ? <p className="mt-2 text-[10px] leading-tight text-muted-foreground/80">people only</p> : null}
            {lane === i ? (
              <div
                key={`${i}-${step === 0 ? 'start' : 'on'}`}
                className="animate-slide-up absolute inset-x-2 bottom-2 rounded-sm border border-foreground/15 bg-card px-2 py-1.5 shadow-sm"
              >
                <p className="font-mono text-[10px] text-muted-foreground">WEB-42</p>
                <p className="truncate text-[11px] font-medium">Fix login redirect</p>
              </div>
            ) : null}
          </div>
        ))}
      </div>

      <ol className="flex-1 space-y-1.5 px-3 py-3 font-mono text-[11.5px] leading-5">
        {session.map((s, i) => (
          <li
            key={i}
            className={cn('transition-opacity duration-500', i <= step ? 'opacity-100' : 'opacity-0')}
            aria-hidden={i > step}
          >
            <p className="truncate">
              <span className="text-muted-foreground">→</span> <span className="text-signal-ink">{s.call}</span>{' '}
              <span className="text-muted-foreground">{s.args}</span>
            </p>
            <p className={cn('truncate pl-4', 'refused' in s ? 'text-destructive' : 'text-foreground')}>
              ← {'refused' in s ? '✕ ' : ''}
              {s.reply}
            </p>
          </li>
        ))}
      </ol>
    </div>
  );
}

// ── Tool table ──────────────────────────────────────────────────────────

type Access = 'read' | 'write' | 'agent';

const toolGroups: { group: string; tools: [string, string, Access][] }[] = [
  {
    group: 'Read the workspace',
    tools: [
      ['projects.list', 'Projects the key can see', 'read'],
      ['projects.board', 'Columns, roles and cards in order', 'read'],
      ['tasks.search', 'Text and filter search, paged', 'read'],
      ['tasks.get', 'One task with subtasks, comments, blockers', 'read'],
    ],
  },
  {
    group: 'Do the work',
    tools: [
      ['tasks.next', 'Pick and claim the next ready task', 'agent'],
      ['tasks.claim · heartbeat · release', 'Hold a task under a lease', 'agent'],
      ['tasks.update', 'Edit with a version check', 'agent'],
      ['tasks.comment · create', 'Leave notes, split work', 'agent'],
      ['tasks.link', 'Add a blocker', 'write'],
      ['tasks.submit_for_review', 'Hand over to a person', 'agent'],
    ],
  },
  {
    group: 'Plan and report',
    tools: [
      ['ai.plan_draft · plan_apply', 'Draft a plan, then create its tasks', 'agent'],
      ['ai.digest · ai.chat', 'Status digest, questions about the workspace', 'read'],
      ['ai.risks_scan · list · dismiss', 'Find and triage delivery risks', 'write'],
    ],
  },
];

const accessLabel: Record<Access, string> = { read: 'Read key', write: 'Write key', agent: 'Agent key' };

function ToolTable() {
  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full min-w-[560px] text-left text-sm">
        <thead className="bg-secondary/60 text-xs text-muted-foreground">
          <tr>
            <th className="px-4 py-2.5 font-medium">Tool</th>
            <th className="px-4 py-2.5 font-medium">What it does</th>
            <th className="px-4 py-2.5 text-right font-medium">Needs</th>
          </tr>
        </thead>
        {toolGroups.map((g, gi) => (
          <tbody key={g.group} className="border-t border-border">
            <tr>
              <th colSpan={3} scope="colgroup" className="bg-card px-4 pb-1 pt-3 text-xs font-medium text-muted-foreground">
                {g.group}
              </th>
            </tr>
            {g.tools.map(([name, what, access], i) => (
              <Reveal
                key={name}
                as="tr"
                variant="fade"
                index={gi * 3 + i}
                className="group bg-card transition-colors hover:bg-secondary/40"
              >
                <td className="px-4 py-2 font-mono text-[12.5px] text-signal-ink">{name}</td>
                <td className="px-4 py-2 text-muted-foreground">{what}</td>
                <td className="px-4 py-2 text-right">
                  <span
                    className={cn(
                      'inline-block rounded-sm px-1.5 py-0.5 text-[11px] font-medium',
                      access === 'read' && 'bg-secondary text-foreground',
                      access === 'write' && 'bg-foreground text-background',
                      access === 'agent' && 'bg-signal text-signal-foreground',
                    )}
                  >
                    {accessLabel[access]}
                  </span>
                </td>
              </Reveal>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}

// ── Client templates ────────────────────────────────────────────────────

const templates = [
  {
    id: 'claude',
    label: 'Claude Code',
    file: 'terminal',
    body: `claude mcp add --transport http fluxboard \\\n  ${MCP_URL} \\\n  --header "Authorization: Bearer fbk_live_…"`,
  },
  {
    id: 'cursor',
    label: 'Cursor',
    file: '.cursor/mcp.json',
    body: `{\n  "mcpServers": {\n    "fluxboard": {\n      "url": "${MCP_URL}",\n      "headers": { "Authorization": "Bearer fbk_live_…" }\n    }\n  }\n}`,
  },
  {
    id: 'http',
    label: 'Any client',
    file: 'HTTP',
    body: `POST ${MCP_URL}\nAuthorization: Bearer fbk_live_…\nContent-Type: application/json\n\n{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
  },
];

function ClientTemplates() {
  const [active, setActive] = useState(templates[0]!.id);
  const [copied, setCopied] = useState(false);
  const t = templates.find((x) => x.id === active) ?? templates[0]!;

  async function copy() {
    try {
      await navigator.clipboard.writeText(t.body);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard blocked (insecure origin or permissions): the text stays selectable.
    }
  }

  return (
    <div className="flex h-full flex-col rounded-lg border border-border bg-card">
      <div role="tablist" aria-label="MCP client" className="flex items-center gap-1 border-b border-border px-2 py-1.5">
        {templates.map((x) => (
          <button
            key={x.id}
            role="tab"
            aria-selected={x.id === active}
            onClick={() => setActive(x.id)}
            className={cn(
              'rounded-sm px-2.5 py-1 text-xs font-medium transition-colors',
              x.id === active ? 'bg-foreground text-background' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {x.label}
          </button>
        ))}
        <button
          onClick={copy}
          aria-label="Copy configuration"
          className="ml-auto flex items-center gap-1 rounded-sm px-2 py-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
          {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <p className="px-4 pt-3 font-mono text-[10.5px] text-muted-foreground">{t.file}</p>
      <pre key={t.id} className="animate-slide-up flex-1 overflow-x-auto px-4 pb-4 pt-1 font-mono text-[12px] leading-relaxed">
        {t.body}
      </pre>
      <p className="border-t border-border px-4 py-2.5 text-xs text-muted-foreground">
        Create an agent under <span className="font-medium text-foreground">Settings → Agents</span>, add it to a
        project, and paste its key.
      </p>
    </div>
  );
}

// ── Guardrails ──────────────────────────────────────────────────────────

const guardrails = [
  { icon: Timer, t: 'Claims with a lease', b: 'One agent per task. A lease that is not renewed frees the task for others.' },
  { icon: GitPullRequestArrow, t: 'People approve', b: 'Agents send work to Review. Only a person moves a task to Done.' },
  { icon: KeyRound, t: 'Scoped keys', b: 'Read keys see read tools only. Agents act under their own project roles.' },
  { icon: History, t: 'Version checks', b: 'Every edit names the version it read, so an agent never overwrites a person.' },
];

export function AgentsSection() {
  return (
    <section className="border-t border-border py-24 md:py-36">
      <div className="container">
        <div className="grid gap-6 lg:grid-cols-[0.9fr_1.1fr] lg:items-end">
          <div>
            <Reveal variant="fade" className="kicker">
              04 — Agents
            </Reveal>
            <h2 className="mt-4 text-[clamp(2.2rem,4.6vw,4rem)] font-semibold leading-[0.95] tracking-[-0.045em]">
              <SplitText text="Your AI agents work the same board." wordClassName={(w) => (w === 'agents' ? 'text-signal' : undefined)} />
            </h2>
          </div>
          <Reveal index={2} className="max-w-[48ch] text-[17px] leading-relaxed text-muted-foreground lg:justify-self-end">
            Fluxboard is an MCP server. Point Claude Code, Cursor or any MCP client at one URL with an agent key, and it
            picks up tasks, reports progress and hands work back for review, under the same roles and audit log as your
            team.
          </Reveal>
        </div>

        <div className="mt-14 grid grid-cols-1 gap-6 lg:grid-cols-[1.15fr_0.85fr]">
          <Reveal variant="scale">
            <AgentSession />
          </Reveal>
          <Reveal variant="up" index={1}>
            <ClientTemplates />
          </Reveal>
        </div>

        <div className="mt-6 grid gap-px overflow-hidden rounded-lg border border-border bg-border sm:grid-cols-2 lg:grid-cols-4">
          {guardrails.map((g, i) => (
            <Reveal key={g.t} index={i} className="group relative bg-background p-6 transition-colors duration-500 hover:bg-card">
              <g.icon className="h-5 w-5 transition-[transform,color] duration-500 ease-spring group-hover:scale-110 group-hover:text-signal" strokeWidth={1.6} />
              <h3 className="mt-8 text-[13px] font-semibold">
                <ScrambleText text={g.t} replayOnHover />
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{g.b}</p>
              <span className="absolute inset-x-0 bottom-0 h-[2px] origin-left scale-x-0 bg-signal transition-transform duration-500 ease-out-expo group-hover:scale-x-100" />
            </Reveal>
          ))}
        </div>

        <div className="mt-14 grid gap-6 lg:grid-cols-[0.35fr_1fr]">
          <Reveal variant="fade">
            <p className="kicker">21 tools</p>
            <p className="mt-3 max-w-[30ch] text-sm leading-relaxed text-muted-foreground">
              Speaks the 2026 stateless protocol and the 2025 handshake revisions, so current and older clients connect.
            </p>
          </Reveal>
          <ToolTable />
        </div>
      </div>
    </section>
  );
}
