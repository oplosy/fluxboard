'use client';

import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { Bold, Code, Heading2, Italic, Link as LinkIcon, List, ListChecks, ListOrdered, Pencil, Quote } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { continueList, insertLink, toggleLines, wrapSelection, type Edit, type LineKind } from './format';
import { Markdown, toggleTaskLine } from './markdown';

const COLLAPSED_PX = 288;
const isMac = () => typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

type Tool = { label: string; hint: string; icon: ReactNode; run: (t: string, s: number, e: number) => Edit };

const TOOLS: Tool[] = [
  { label: 'Bold', hint: 'B', icon: <Bold className="h-4 w-4" />, run: (t, s, e) => wrapSelection(t, s, e, '**', '**', 'bold text') },
  { label: 'Italic', hint: 'I', icon: <Italic className="h-4 w-4" />, run: (t, s, e) => wrapSelection(t, s, e, '_', '_', 'italic text') },
  { label: 'Heading', hint: '', icon: <Heading2 className="h-4 w-4" />, run: (t, s, e) => toggleLines(t, s, e, 'heading') },
  { label: 'Bulleted list', hint: '', icon: <List className="h-4 w-4" />, run: (t, s, e) => toggleLines(t, s, e, 'ul') },
  { label: 'Numbered list', hint: '', icon: <ListOrdered className="h-4 w-4" />, run: (t, s, e) => toggleLines(t, s, e, 'ol') },
  { label: 'Checklist', hint: '', icon: <ListChecks className="h-4 w-4" />, run: (t, s, e) => toggleLines(t, s, e, 'task') },
  { label: 'Code', hint: '', icon: <Code className="h-4 w-4" />, run: (t, s, e) => codeEdit(t, s, e) },
  { label: 'Link', hint: 'K', icon: <LinkIcon className="h-4 w-4" />, run: (t, s, e) => insertLink(t, s, e) },
  { label: 'Quote', hint: '', icon: <Quote className="h-4 w-4" />, run: (t, s, e) => toggleLines(t, s, e, 'quote' satisfies LineKind) },
];

// Multi-line selections become a fenced block; everything else is inline code.
function codeEdit(t: string, s: number, e: number): Edit {
  if (t.slice(s, e).includes('\n')) return wrapSelection(t, s, e, '```\n', '\n```', 'code');
  return wrapSelection(t, s, e, '`', '`', 'code');
}

/**
 * Markdown description with a rendered read mode and a write/preview editor.
 * `onSave` must reject when the save fails (the caller reports the error); the
 * draft is then kept so nothing typed is lost.
 */
export function DescriptionEditor({
  value,
  canEdit,
  onSave,
  placeholder = 'Add a description…',
}: {
  value: string;
  canEdit: boolean;
  onSave: (text: string) => Promise<unknown>;
  placeholder?: string;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState('');
  const [tab, setTab] = useState<'write' | 'preview'>('write');
  const [saving, setSaving] = useState(false);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [overflows, setOverflows] = useState(false);
  const ta = useRef<HTMLTextAreaElement>(null);
  const content = useRef<HTMLDivElement>(null);
  const pendingSel = useRef<{ start: number; end: number } | null>(null);

  const dirty = editing && draft !== value;

  // Restore the selection after a toolbar/shortcut edit re-rendered the textarea.
  useLayoutEffect(() => {
    const sel = pendingSel.current;
    if (!sel || !ta.current) return;
    pendingSel.current = null;
    ta.current.focus();
    ta.current.setSelectionRange(sel.start, sel.end);
  }, [draft]);

  useLayoutEffect(() => {
    if (editing || !content.current) return;
    setOverflows(content.current.scrollHeight > COLLAPSED_PX + 24);
  }, [value, editing]);

  function startEdit() {
    if (!canEdit || editing) return;
    setDraft(value);
    setTab('write');
    setConfirmDiscard(false);
    setEditing(true);
    requestAnimationFrame(() => {
      const el = ta.current;
      if (!el) return;
      el.focus();
      el.setSelectionRange(el.value.length, el.value.length);
    });
  }

  function cancel(force = false) {
    if (!force && dirty) {
      setConfirmDiscard(true);
      return;
    }
    setConfirmDiscard(false);
    setEditing(false);
  }

  async function save() {
    if (saving) return;
    if (draft === value) {
      setEditing(false);
      return;
    }
    setSaving(true);
    try {
      await onSave(draft);
      setEditing(false);
    } catch {
      // The caller already surfaced the error; stay in edit mode with the draft.
    } finally {
      setSaving(false);
    }
  }

  function apply(edit: Edit) {
    pendingSel.current = { start: edit.start, end: edit.end };
    setDraft(edit.text);
    setConfirmDiscard(false);
  }

  function runTool(tool: Tool) {
    const el = ta.current;
    if (!el) return;
    apply(tool.run(el.value, el.selectionStart, el.selectionEnd));
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    const mod = e.ctrlKey || e.metaKey;
    const key = e.key.toLowerCase();
    if (mod && key === 'enter') {
      e.preventDefault();
      void save();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation(); // don't close a surrounding modal
      cancel();
    } else if (mod && !e.shiftKey && !e.altKey && (key === 'b' || key === 'i' || key === 'k')) {
      e.preventDefault();
      runTool(TOOLS.find((t) => t.hint.toLowerCase() === key)!);
    } else if (e.key === 'Enter' && !mod && !e.shiftKey && !e.altKey && !e.nativeEvent.isComposing) {
      const el = e.currentTarget;
      if (el.selectionStart !== el.selectionEnd) return;
      const next = continueList(el.value, el.selectionStart);
      if (next) {
        e.preventDefault();
        apply({ text: next.text, start: next.caret, end: next.caret });
      }
    }
  }

  const modKey = isMac() ? '⌘' : 'Ctrl';

  if (!editing) {
    const empty = !value.trim();
    if (empty) {
      return canEdit ? (
        <button
          type="button"
          onClick={startEdit}
          className="group flex w-full items-center gap-2 rounded-md border border-dashed border-border px-3.5 py-4 text-left text-sm text-muted-foreground transition-colors hover:border-foreground/40 hover:bg-secondary/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal"
        >
          <Pencil className="h-3.5 w-3.5" aria-hidden />
          {placeholder}
        </button>
      ) : (
        <p className="text-sm text-muted-foreground">No description.</p>
      );
    }
    const collapsed = overflows && !expanded;
    return (
      <div className="group/desc relative">
        <div
          role={canEdit ? 'group' : undefined}
          aria-label="Description"
          tabIndex={canEdit ? 0 : undefined}
          onKeyDown={(e) => {
            if (canEdit && e.target === e.currentTarget && e.key.toLowerCase() === 'e' && !e.metaKey && !e.ctrlKey && !e.altKey) {
              e.preventDefault();
              startEdit();
            }
          }}
          onClick={(e) => {
            if (!canEdit) return;
            if ((e.target as HTMLElement).closest('a,input,button')) return;
            if (window.getSelection()?.toString()) return; // selecting text, not editing
            startEdit();
          }}
          className={cn(
            'rounded-md outline-none focus-visible:ring-2 focus-visible:ring-signal',
            canEdit && 'cursor-text hover:bg-secondary/30 -mx-2 px-2 py-1',
          )}
        >
          <div
            ref={content}
            className="relative overflow-hidden"
            style={
              collapsed
                ? {
                    maxHeight: COLLAPSED_PX,
                    maskImage: 'linear-gradient(to bottom, black 70%, transparent)',
                    WebkitMaskImage: 'linear-gradient(to bottom, black 70%, transparent)',
                  }
                : undefined
            }
          >
            <Markdown
              source={value}
              onToggleTask={canEdit && !saving ? (line) => {
                setSaving(true);
                onSave(toggleTaskLine(value, line))
                  .catch(() => undefined)
                  .finally(() => setSaving(false));
              } : undefined}
            />
          </div>
        </div>
        {overflows ? (
          <button
            type="button"
            onClick={() => setExpanded((x) => !x)}
            aria-expanded={expanded}
            className="mt-1 text-xs font-medium text-muted-foreground underline-offset-2 hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal"
          >
            {expanded ? 'Show less' : 'Show more'}
          </button>
        ) : null}
        {canEdit ? (
          <div className="absolute right-0 -top-9 opacity-0 transition-opacity focus-within:opacity-100 group-hover/desc:opacity-100">
            <Tooltip content="Edit (E)">
              <button
                type="button"
                onClick={startEdit}
                aria-label="Edit description"
                className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal"
              >
                <Pencil className="h-3.5 w-3.5" />
              </button>
            </Tooltip>
          </div>
        ) : null}
      </div>
    );
  }

  return (
    <div className="rounded-md border border-input bg-card focus-within:border-foreground/60">
      <div className="flex flex-wrap items-center gap-1 border-b border-border px-1.5 py-1">
        <div role="tablist" aria-label="Editor mode" className="mr-1 flex rounded-md bg-secondary p-0.5">
          {(['write', 'preview'] as const).map((t) => (
            <button
              key={t}
              type="button"
              role="tab"
              aria-selected={tab === t}
              onClick={() => setTab(t)}
              className={cn(
                'rounded-sm px-2.5 py-1 text-xs font-medium capitalize transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal',
                tab === t ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {t}
            </button>
          ))}
        </div>
        <div role="toolbar" aria-label="Formatting" className={cn('flex flex-wrap items-center gap-0.5', tab === 'preview' && 'pointer-events-none opacity-40')}>
          {TOOLS.map((tool) => (
            <Tooltip key={tool.label} content={tool.hint ? `${tool.label} (${modKey}+${tool.hint})` : tool.label}>
              <button
                type="button"
                aria-label={tool.label}
                disabled={tab === 'preview'}
                onMouseDown={(e) => e.preventDefault()} // keep the textarea selection
                onClick={() => runTool(tool)}
                className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal"
              >
                {tool.icon}
              </button>
            </Tooltip>
          ))}
        </div>
      </div>

      {tab === 'write' ? (
        <Textarea
          ref={ta}
          autoResize
          maxRows={20}
          rows={5}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Write in markdown — **bold**, - lists, - [ ] tasks, `code`"
          aria-label="Description"
          disabled={saving}
          className="rounded-none border-0 bg-transparent focus-visible:shadow-none"
        />
      ) : (
        <div className="min-h-[7rem] px-3.5 py-2.5">
          {draft.trim() ? <Markdown source={draft} /> : <p className="text-sm text-muted-foreground">Nothing to preview.</p>}
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border px-2 py-1.5">
        {confirmDiscard ? (
          <div role="alertdialog" aria-label="Discard changes" className="flex items-center gap-2 text-xs">
            <span className="text-muted-foreground">Discard unsaved changes?</span>
            <Button size="sm" variant="destructive" onClick={() => cancel(true)}>
              Discard
            </Button>
            <Button size="sm" variant="ghost" onClick={() => { setConfirmDiscard(false); ta.current?.focus(); }}>
              Keep editing
            </Button>
          </div>
        ) : (
          <span className="eyebrow hidden sm:inline">
            {modKey}+Enter to save · Esc to cancel
          </span>
        )}
        <div className="ml-auto flex items-center gap-2">
          <Button size="sm" variant="ghost" onClick={() => cancel()} disabled={saving}>
            Cancel
          </Button>
          <Button size="sm" onClick={() => void save()} isLoading={saving} disabled={!dirty}>
            Save
          </Button>
        </div>
      </div>
    </div>
  );
}
