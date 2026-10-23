import Link from 'next/link';
import { Fragment, type ReactNode } from 'react';

import { cn } from '@/lib/utils';

// A small, SAFE markdown renderer. It builds React elements only (never raw
// HTML), so markup typed by a user is shown as text. Links are limited to
// http(s) and mailto. Supported: headings, paragraphs, bold/italic/strike,
// inline code, fenced code, links + bare URLs, @mentions, quotes, rules,
// bullet/numbered/task lists (nested by indentation). With `taskRefs`, task keys
// of the project (WEB-12) that exist link to the task.

type Item = {
  text: string;
  indent: number;
  ordered: boolean;
  task?: { checked: boolean; line: number | null };
  children: ListBlock[];
};
type ListBlock = { t: 'list'; ordered: boolean; items: Item[] };
type Block =
  | { t: 'p'; lines: string[] }
  | { t: 'h'; level: number; text: string }
  | { t: 'code'; lang: string; text: string }
  | { t: 'quote'; blocks: Block[] }
  | { t: 'hr' }
  | ListBlock;

/** Task keys to link in a text: only `key`-N for a number in `numbers` becomes a link. */
export type TaskRefs = { key: string; numbers: ReadonlySet<number>; href: (number: number) => string };

const SAFE_PROTOCOLS = new Set(['http:', 'https:', 'mailto:']);

/** Returns the URL when it is safe to link to, otherwise null. */
export function safeHref(raw: string): string | null {
  const url = raw.trim();
  if (!url) return null;
  try {
    const u = new URL(url);
    return SAFE_PROTOCOLS.has(u.protocol) ? u.href : null;
  } catch {
    return null;
  }
}

const LIST_ITEM = /^(\s*)([-*+]|\d+[.)])\s+(?:\[([ xX])\]\s+)?(.*)$/;
const FENCE = /^\s*(`{3,}|~{3,})\s*([\w+-]*)\s*$/;
const HEADING = /^(#{1,6})\s+(.*?)\s*#*\s*$/;
const HR = /^\s*([-*_])(\s*\1){2,}\s*$/;

function indentOf(ws: string): number {
  return ws.replace(/\t/g, '    ').length;
}

function buildList(run: Item[], pos: { i: number }, indent: number): ListBlock {
  const list: ListBlock = { t: 'list', ordered: run[pos.i]!.ordered, items: [] };
  while (pos.i < run.length && run[pos.i]!.indent >= indent) {
    const cur = run[pos.i]!;
    if (cur.indent > indent && list.items.length > 0) {
      list.items[list.items.length - 1]!.children.push(buildList(run, pos, cur.indent));
      continue;
    }
    list.items.push(cur);
    pos.i++;
  }
  return list;
}

/** `baseLine` is the source line of lines[0]; null disables task toggling. */
function parseBlocks(lines: string[], baseLine: number | null): Block[] {
  const blocks: Block[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i]!;
    if (!line.trim()) {
      i++;
      continue;
    }
    const fence = FENCE.exec(line);
    if (fence) {
      const mark = fence[1]!;
      const body: string[] = [];
      i++;
      while (i < lines.length && !lines[i]!.trim().startsWith(mark)) body.push(lines[i++]!);
      i++; // closing fence (or end of input)
      blocks.push({ t: 'code', lang: fence[2] ?? '', text: body.join('\n') });
      continue;
    }
    const h = HEADING.exec(line);
    if (h) {
      blocks.push({ t: 'h', level: h[1]!.length, text: h[2]! });
      i++;
      continue;
    }
    if (HR.test(line)) {
      blocks.push({ t: 'hr' });
      i++;
      continue;
    }
    if (/^\s*>/.test(line)) {
      const inner: string[] = [];
      while (i < lines.length && /^\s*>/.test(lines[i]!)) inner.push(lines[i++]!.replace(/^\s*> ?/, ''));
      blocks.push({ t: 'quote', blocks: parseBlocks(inner, null) });
      continue;
    }
    if (LIST_ITEM.test(line)) {
      const run: Item[] = [];
      while (i < lines.length) {
        const m = LIST_ITEM.exec(lines[i]!);
        if (!m || HR.test(lines[i]!)) break;
        run.push({
          text: m[4]!,
          indent: indentOf(m[1]!),
          ordered: /\d/.test(m[2]!),
          task: m[3] === undefined ? undefined : { checked: m[3] !== ' ', line: baseLine === null ? null : baseLine + i },
          children: [],
        });
        i++;
      }
      const pos = { i: 0 };
      while (pos.i < run.length) blocks.push(buildList(run, pos, run[pos.i]!.indent));
      continue;
    }
    const para: string[] = [];
    while (
      i < lines.length &&
      lines[i]!.trim() &&
      !FENCE.test(lines[i]!) &&
      !HEADING.test(lines[i]!) &&
      !/^\s*>/.test(lines[i]!) &&
      !LIST_ITEM.test(lines[i]!) &&
      !HR.test(lines[i]!)
    ) {
      para.push(lines[i++]!);
    }
    blocks.push({ t: 'p', lines: para });
  }
  return blocks;
}

export function parseMarkdown(src: string): Block[] {
  return parseBlocks(
    src.split('\n').map((l) => l.replace(/\r$/, '')),
    0,
  );
}

// Groups: 2 code, 3/4 bold, 5 strike, 6/7 em, 8 link text, 9 link url,
// 10 bare url, 11 mention, 12 escaped char, 13/14 task key + number.
const INLINE =
  /(`+)([^`]+?)\1(?!`)|\*\*(?=\S)([\s\S]+?)\*\*|__(?=\S)([\s\S]+?)__|~~(?=\S)([\s\S]+?)~~|\*(?=[^\s*])([\s\S]*?[^\s*])\*|(?<![\w])_(?=[^\s_])([\s\S]*?[^\s_])_(?![\w])|\[([^\]\n]+)\]\(([^)\s]+)\)|(https?:\/\/[^\s<>]+[^\s<>.,;:!?)\]'"])|(?<![\w@])@([A-Za-z0-9](?:[\w.-]*\w)?)|\\([\\`*_{}[\]()#+\-.!~>@])|(?<![\w@-])([A-Z]{2,6})-(\d{1,9})(?!\w)/g;

function renderInline(text: string, refs?: TaskRefs, depth = 0): ReactNode[] {
  const out: ReactNode[] = [];
  const re = new RegExp(INLINE.source, 'g');
  let last = 0;
  let k = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    last = m.index + m[0].length;
    const key = `i${k++}`;
    const nest = (s: string) => (depth < 4 ? renderInline(s, refs, depth + 1) : [s]);
    if (m[2] !== undefined) {
      out.push(
        <code key={key} className="rounded-sm bg-secondary px-1 py-0.5 font-mono text-[0.85em]">
          {m[2]}
        </code>,
      );
    } else if (m[3] !== undefined || m[4] !== undefined) {
      out.push(
        <strong key={key} className="font-semibold">
          {nest((m[3] ?? m[4])!)}
        </strong>,
      );
    } else if (m[5] !== undefined) {
      out.push(
        <del key={key} className="text-muted-foreground">
          {nest(m[5])}
        </del>,
      );
    } else if (m[6] !== undefined || m[7] !== undefined) {
      out.push(<em key={key}>{nest((m[6] ?? m[7])!)}</em>);
    } else if (m[8] !== undefined) {
      const href = safeHref(m[9]!);
      if (href) {
        out.push(
          <a
            key={key}
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-signal underline underline-offset-2 hover:no-underline"
          >
            {nest(m[8])}
          </a>,
        );
      } else {
        out.push(m[0]); // unsafe or invalid target: show the markup as text
      }
    } else if (m[10] !== undefined) {
      const href = safeHref(m[10]);
      if (href) {
        out.push(
          <a
            key={key}
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="break-all text-signal underline underline-offset-2 hover:no-underline"
          >
            {m[10]}
          </a>,
        );
      } else {
        out.push(m[10]);
      }
    } else if (m[11] !== undefined) {
      out.push(
        <span key={key} className="rounded-sm bg-signal/10 px-1 font-medium text-signal">
          @{m[11]}
        </span>,
      );
    } else if (m[12] !== undefined) {
      out.push(m[12]);
    } else if (m[13] !== undefined) {
      const num = Number(m[14]);
      if (refs && m[13] === refs.key && refs.numbers.has(num)) {
        out.push(
          <Link key={key} href={refs.href(num)} className="font-medium text-signal underline underline-offset-2 hover:no-underline">
            {m[0]}
          </Link>,
        );
      } else {
        out.push(m[0]);
      }
    }
  }
  if (last < text.length) out.push(text.slice(last));
  return out.map((n, idx) => (typeof n === 'string' ? <Fragment key={`t${idx}`}>{n}</Fragment> : n));
}

const HEADING_CLASS = [
  '',
  'mt-5 mb-2 text-xl font-bold tracking-tight first:mt-0',
  'mt-5 mb-2 text-lg font-bold tracking-tight first:mt-0',
  'mt-4 mb-1.5 text-base font-semibold first:mt-0',
  'mt-4 mb-1.5 text-sm font-semibold first:mt-0',
  'mt-3 mb-1 text-sm font-semibold first:mt-0',
  'mt-3 mb-1 text-sm font-medium text-muted-foreground first:mt-0',
];

function renderList(list: ListBlock, onToggle: ((line: number) => void) | undefined, key: number, refs?: TaskRefs): ReactNode {
  const Tag = list.ordered ? 'ol' : 'ul';
  return (
    <Tag key={key} className={cn('my-2 space-y-1 pl-5 marker:text-muted-foreground', list.ordered ? 'list-decimal' : 'list-disc')}>
      {list.items.map((it, idx) => (
        <li key={idx} className={cn(it.task && '-ml-5 flex list-none items-start gap-2')}>
          {it.task ? (
            <input
              type="checkbox"
              checked={it.task.checked}
              disabled={!onToggle || it.task.line === null}
              onChange={() => it.task!.line !== null && onToggle?.(it.task!.line)}
              aria-label={it.text || 'Task'}
              className="mt-[0.3rem] h-3.5 w-3.5 shrink-0 cursor-pointer accent-signal disabled:cursor-default"
            />
          ) : null}
          <div className={cn('min-w-0', it.task?.checked && 'text-muted-foreground line-through')}>
            {renderInline(it.text, refs)}
            {it.children.map((c, ci) => renderList(c, onToggle, ci, refs))}
          </div>
        </li>
      ))}
    </Tag>
  );
}

function renderBlocks(blocks: Block[], onToggle: ((line: number) => void) | undefined, refs?: TaskRefs): ReactNode[] {
  return blocks.map((b, i) => {
    switch (b.t) {
      case 'h': {
        const Tag = `h${Math.min(b.level, 6)}` as 'h1';
        return (
          <Tag key={i} className={HEADING_CLASS[b.level]}>
            {renderInline(b.text, refs)}
          </Tag>
        );
      }
      case 'p':
        return (
          <p key={i} className="my-2 first:mt-0 last:mb-0">
            {b.lines.map((l, li) => (
              <Fragment key={li}>
                {li > 0 ? <br /> : null}
                {renderInline(l, refs)}
              </Fragment>
            ))}
          </p>
        );
      case 'code':
        return (
          <pre
            key={i}
            className="my-3 overflow-x-auto rounded-md border border-border bg-secondary px-3.5 py-3 font-mono text-[13px] leading-relaxed"
          >
            <code>{b.text}</code>
          </pre>
        );
      case 'quote':
        return (
          <blockquote key={i} className="my-3 border-l-2 border-signal pl-4 text-muted-foreground">
            {renderBlocks(b.blocks, undefined, refs)}
          </blockquote>
        );
      case 'hr':
        return <hr key={i} className="my-4 border-border" />;
      case 'list':
        return renderList(b, onToggle, i, refs);
    }
  });
}

/** Flips `[ ]` / `[x]` on the given 0-based source line. Returns src unchanged if it is not a task. */
export function toggleTaskLine(src: string, line: number): string {
  const lines = src.split('\n');
  const l = lines[line];
  if (l === undefined) return src;
  const next = l.replace(
    /^(\s*(?:[-*+]|\d+[.)])\s+\[)([ xX])(\])/,
    (_m, a: string, c: string, d: string) => `${a}${c === ' ' ? 'x' : ' '}${d}`,
  );
  if (next === l) return src;
  lines[line] = next;
  return lines.join('\n');
}

export function Markdown({
  source,
  onToggleTask,
  taskRefs,
  className,
}: {
  source: string;
  /** Called with the 0-based source line of a clicked task checkbox. */
  onToggleTask?: (line: number) => void;
  /** Link the project's task keys that exist (wiki pages). */
  taskRefs?: TaskRefs;
  className?: string;
}) {
  return <div className={cn('break-words text-sm leading-relaxed', className)}>{renderBlocks(parseMarkdown(source), onToggleTask, taskRefs)}</div>;
}
