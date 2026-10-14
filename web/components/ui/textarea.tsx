'use client';

import {
  forwardRef,
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type TextareaHTMLAttributes,
} from 'react';

import { cn } from '@/lib/utils';

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** Grow with content (up to `maxRows`) instead of showing a scrollbar. */
  autoResize?: boolean;
  maxRows?: number;
  /** Show a `n / maxLength` counter (requires `maxLength`). */
  showCount?: boolean;
}

/** Shared field chrome; matches Input (hairline border, signal underline on focus). */
export const textareaClass = cn(
  'block w-full rounded-md border border-input bg-card px-3.5 py-2.5 text-sm leading-relaxed text-foreground',
  'transition-[border-color,box-shadow,background-color] duration-300 ease-out-expo',
  'placeholder:text-muted-foreground/80',
  'hover:border-foreground/40',
  'focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none',
  'aria-[invalid=true]:border-destructive',
  'disabled:cursor-not-allowed disabled:opacity-50',
);

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, autoResize, maxRows = 12, showCount, maxLength, rows, onChange, value, defaultValue, ...props },
  forwardedRef,
) {
  const inner = useRef<HTMLTextAreaElement | null>(null);
  const [typedLength, setTypedLength] = useState(typeof defaultValue === 'string' ? defaultValue.length : 0);
  const length = typeof value === 'string' ? value.length : typedLength;

  const setRefs = useCallback(
    (node: HTMLTextAreaElement | null) => {
      inner.current = node;
      if (typeof forwardedRef === 'function') forwardedRef(node);
      else if (forwardedRef) forwardedRef.current = node;
    },
    [forwardedRef],
  );

  const fit = useCallback(() => {
    const el = inner.current;
    if (!el || !autoResize) return;
    el.style.height = 'auto';
    const cs = getComputedStyle(el);
    const line = parseFloat(cs.lineHeight) || 20;
    const pad = parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom);
    const max = line * maxRows + pad;
    el.style.height = `${Math.min(el.scrollHeight, max)}px`;
    el.style.overflowY = el.scrollHeight > max ? 'auto' : 'hidden';
  }, [autoResize, maxRows]);

  useLayoutEffect(fit, [fit, value]);

  const handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
    setTypedLength(e.target.value.length);
    fit();
    onChange?.(e);
  };

  const field = (
    <textarea
      ref={setRefs}
      rows={rows ?? (autoResize ? 1 : 3)}
      maxLength={maxLength}
      value={value}
      defaultValue={defaultValue}
      onChange={handleChange}
      className={cn(textareaClass, autoResize ? 'resize-none' : 'resize-y', className)}
      {...props}
    />
  );

  if (!showCount || !maxLength) return field;
  return (
    <div className="w-full">
      {field}
      <div className="mt-1 text-right text-xs tabular-nums text-muted-foreground">
        {length} / {maxLength}
      </div>
    </div>
  );
});
