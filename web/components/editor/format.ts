// Pure text transforms behind the editor toolbar and keyboard shortcuts. Each
// returns the new text plus the selection to restore.

export interface Edit {
  text: string;
  start: number;
  end: number;
}

export function wrapSelection(text: string, s: number, e: number, before: string, after: string, placeholder: string): Edit {
  const sel = text.slice(s, e);
  if (s >= before.length && text.slice(s - before.length, s) === before && text.slice(e, e + after.length) === after) {
    // Already wrapped outside the selection: unwrap.
    return { text: text.slice(0, s - before.length) + sel + text.slice(e + after.length), start: s - before.length, end: e - before.length };
  }
  if (sel.length >= before.length + after.length && sel.startsWith(before) && sel.endsWith(after)) {
    const inner = sel.slice(before.length, sel.length - after.length);
    return { text: text.slice(0, s) + inner + text.slice(e), start: s, end: s + inner.length };
  }
  const body = sel || placeholder;
  const next = text.slice(0, s) + before + body + after + text.slice(e);
  return { text: next, start: s + before.length, end: s + before.length + body.length };
}

export function insertLink(text: string, s: number, e: number): Edit {
  const sel = text.slice(s, e);
  const isUrl = /^https?:\/\/\S+$/.test(sel);
  const label = isUrl ? 'link' : sel || 'link text';
  const url = isUrl ? sel : 'https://';
  const next = text.slice(0, s) + `[${label}](${url})` + text.slice(e);
  if (isUrl || !sel) {
    // Select the label the user has to fill in.
    return { text: next, start: s + 1, end: s + 1 + label.length };
  }
  // Selected text becomes the label; select the placeholder URL.
  const urlStart = s + label.length + 3;
  return { text: next, start: urlStart, end: urlStart + url.length };
}

export type LineKind = 'ul' | 'ol' | 'task' | 'quote' | 'heading';

const OWN: Record<LineKind, RegExp> = {
  ul: /^(\s*)[-*+] (?!\[[ xX]\] )/,
  ol: /^(\s*)\d+\. /,
  task: /^(\s*)[-*+] \[[ xX]\] /,
  quote: /^()> ?/,
  heading: /^()## /,
};
const ANY_LIST = /^(\s*)(?:[-*+] \[[ xX]\] |[-*+] |\d+\. )/;

/** Toggles a line prefix (list, quote, heading) on every line the selection touches. */
export function toggleLines(text: string, s: number, e: number, kind: LineKind): Edit {
  const ls = text.lastIndexOf('\n', s - 1) + 1;
  let le = text.indexOf('\n', e);
  if (le === -1) le = text.length;
  const lines = text.slice(ls, le).split('\n');
  const multi = lines.length > 1;
  const active = lines.filter((l) => !multi || l.trim());
  const allOn = active.length > 0 && active.every((l) => OWN[kind].test(l));
  let n = 0;
  const out = lines.map((l) => {
    if (multi && !l.trim()) return l;
    n++;
    if (allOn) return l.replace(OWN[kind], '$1');
    if (kind === 'quote') return `> ${l}`;
    if (kind === 'heading') return `## ${l.replace(/^#{1,6} /, '')}`;
    const base = l.replace(ANY_LIST, '$1');
    const indent = /^\s*/.exec(base)![0];
    const prefix = kind === 'ul' ? '- ' : kind === 'ol' ? `${n}. ` : '- [ ] ';
    return indent + prefix + base.slice(indent.length);
  });
  const joined = out.join('\n');
  return { text: text.slice(0, ls) + joined + text.slice(le), start: ls, end: ls + joined.length };
}

/** Enter inside a list/quote line: continue the marker, or end the list on an empty item. */
export function continueList(text: string, caret: number): { text: string; caret: number } | null {
  const ls = text.lastIndexOf('\n', caret - 1) + 1;
  const m = /^(\s*)([-*+] \[[ xX]\] |[-*+] |(\d+)\. |> )(.*)$/.exec(text.slice(ls, caret));
  if (!m) return null;
  if (!m[4]) {
    // Empty item: drop the marker and leave the list.
    return { text: text.slice(0, ls) + text.slice(caret), caret: ls };
  }
  const marker = m[3] ? `${Number(m[3]) + 1}. ` : m[2]!.replace(/\[[ xX]\]/, '[ ]');
  const ins = `\n${m[1]}${marker}`;
  return { text: text.slice(0, caret) + ins + text.slice(caret), caret: caret + ins.length };
}
