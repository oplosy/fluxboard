import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { continueList, toggleLines, wrapSelection } from './format';
import { Markdown, safeHref, toggleTaskLine } from './markdown';

describe('Markdown renderer safety', () => {
  it('does not render javascript: links as links', () => {
    const { container } = render(<Markdown source="[click](javascript:alert(1))" />);
    expect(container.querySelector('a')).toBeNull();
    expect(container.textContent).toContain('[click](javascript:alert(1))');
  });

  it('does not render data: or vbscript: links', () => {
    const { container } = render(<Markdown source={'[a](data:text/html,x) [b](vbscript:x)'} />);
    expect(container.querySelector('a')).toBeNull();
  });

  it('renders script tags as text', () => {
    const { container } = render(<Markdown source={'<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>'} />);
    expect(container.querySelector('script')).toBeNull();
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toContain('<script>alert(1)</script>');
  });

  it('opens safe links in a new tab with noopener', () => {
    render(<Markdown source="[docs](https://example.com/a)" />);
    const a = screen.getByRole('link', { name: 'docs' });
    expect(a).toHaveAttribute('href', 'https://example.com/a');
    expect(a).toHaveAttribute('target', '_blank');
    expect(a).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('only allows http, https and mailto', () => {
    expect(safeHref('https://a.b')).toBeTruthy();
    expect(safeHref('mailto:a@b.co')).toBeTruthy();
    expect(safeHref('javascript:alert(1)')).toBeNull();
    expect(safeHref('/relative')).toBeNull();
  });
});

describe('Markdown renderer features', () => {
  it('renders headings, emphasis, code and lists', () => {
    const { container } = render(
      <Markdown source={'# Title\n\n**bold** _it_ `code`\n\n- a\n- b\n\n1. one\n2. two\n\n```\nlet x = 1\n```'} />,
    );
    expect(container.querySelector('h1')?.textContent).toBe('Title');
    expect(container.querySelector('strong')?.textContent).toBe('bold');
    expect(container.querySelector('em')?.textContent).toBe('it');
    expect(container.querySelectorAll('ul > li')).toHaveLength(2);
    expect(container.querySelectorAll('ol > li')).toHaveLength(2);
    expect(container.querySelector('pre code')?.textContent).toBe('let x = 1');
  });

  it('keeps single newlines of plain-text descriptions', () => {
    const { container } = render(<Markdown source={'line one\nline two'} />);
    expect(container.querySelectorAll('br')).toHaveLength(1);
  });

  it('renders task checkboxes and reports the source line on toggle', () => {
    const calls: number[] = [];
    render(<Markdown source={'intro\n\n- [ ] first\n- [x] second'} onToggleTask={(l) => calls.push(l)} />);
    const boxes = screen.getAllByRole('checkbox');
    expect(boxes).toHaveLength(2);
    expect(boxes[1]).toBeChecked();
    boxes[0]!.click();
    expect(calls).toEqual([2]);
  });

  it('toggles a task line in the source', () => {
    expect(toggleTaskLine('a\n- [ ] x', 1)).toBe('a\n- [x] x');
    expect(toggleTaskLine('a\n- [x] x', 1)).toBe('a\n- [ ] x');
    expect(toggleTaskLine('a\n- x', 1)).toBe('a\n- x');
  });
});

describe('format helpers', () => {
  it('wraps and unwraps a selection', () => {
    const w = wrapSelection('hello world', 0, 5, '**', '**', 'x');
    expect(w.text).toBe('**hello** world');
    expect(wrapSelection(w.text, w.start, w.end, '**', '**', 'x').text).toBe('hello world');
  });

  it('toggles list prefixes on lines', () => {
    const on = toggleLines('a\nb', 0, 3, 'ul');
    expect(on.text).toBe('- a\n- b');
    expect(toggleLines(on.text, on.start, on.end, 'ul').text).toBe('a\nb');
    expect(toggleLines('a\nb', 0, 3, 'ol').text).toBe('1. a\n2. b');
  });

  it('continues and ends lists on Enter', () => {
    expect(continueList('- a', 3)?.text).toBe('- a\n- ');
    expect(continueList('1. a', 4)?.text).toBe('1. a\n2. ');
    expect(continueList('- [x] a', 7)?.text).toBe('- [x] a\n- [ ] ');
    expect(continueList('- a\n- ', 6)?.text).toBe('- a\n');
    expect(continueList('plain', 5)).toBeNull();
  });
});
