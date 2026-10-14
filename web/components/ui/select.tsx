'use client';

import { Check, ChevronDown } from 'lucide-react';
import {
  forwardRef,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react';

import { cn } from '@/lib/utils';
import { Popover } from './popover';

export interface SelectOption {
  value: string;
  label: string;
  icon?: ReactNode;
  description?: string;
  disabled?: boolean;
}

export interface SelectProps {
  value: string;
  onValueChange: (value: string) => void;
  options: SelectOption[];
  /** Shown when `value` matches no option (e.g. an empty value). */
  placeholder?: string;
  size?: 'sm' | 'md';
  disabled?: boolean;
  id?: string;
  name?: string;
  required?: boolean;
  className?: string;
  /** Class for the popover. */
  popoverClassName?: string;
  /** Popover alignment relative to the trigger. */
  align?: 'start' | 'end';
  'aria-label'?: string;
  'aria-labelledby'?: string;
  'aria-invalid'?: boolean | 'true' | 'false';
}

/**
 * Accessible single-select: button trigger + portalled listbox. Controlled,
 * with a native-like API (`value`, `onValueChange`). Empty-string values are
 * valid option values. When `name` is set a hidden input carries the value so
 * it participates in form submission.
 */
export const Select = forwardRef<HTMLButtonElement, SelectProps>(function Select(
  {
    value,
    onValueChange,
    options,
    placeholder = 'Select…',
    size = 'md',
    disabled,
    id,
    name,
    required,
    className,
    popoverClassName,
    align = 'start',
    'aria-label': ariaLabel,
    'aria-labelledby': ariaLabelledBy,
    'aria-invalid': ariaInvalid,
  },
  forwardedRef,
) {
  const uid = useId();
  const listId = `${uid}-list`;
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const typeahead = useRef({ text: '', at: 0 });

  const setRefs = useCallback(
    (node: HTMLButtonElement | null) => {
      triggerRef.current = node;
      if (typeof forwardedRef === 'function') forwardedRef(node);
      else if (forwardedRef) forwardedRef.current = node;
    },
    [forwardedRef],
  );

  const selectedIndex = useMemo(() => options.findIndex((o) => o.value === value), [options, value]);
  const selected = selectedIndex >= 0 ? options[selectedIndex] : undefined;

  const enabled = useCallback((i: number) => i >= 0 && i < options.length && !options[i]!.disabled, [options]);
  const step = useCallback(
    (from: number, dir: 1 | -1) => {
      for (let i = from + dir; i >= 0 && i < options.length; i += dir) if (enabled(i)) return i;
      return from;
    },
    [options.length, enabled],
  );
  const edge = useCallback((dir: 1 | -1) => step(dir === 1 ? -1 : options.length, dir), [options.length, step]);

  const close = useCallback((refocus = true) => {
    setOpen(false);
    if (refocus) triggerRef.current?.focus();
  }, []);
  const closeNoFocus = useCallback(() => close(false), [close]);

  const openList = useCallback(
    (at?: 'last') => {
      if (disabled) return;
      setActive(at === 'last' ? edge(-1) : enabled(selectedIndex) ? selectedIndex : edge(1));
      setOpen(true);
    },
    [disabled, edge, enabled, selectedIndex],
  );

  const choose = useCallback(
    (i: number) => {
      const o = options[i];
      if (!o || o.disabled) return;
      if (o.value !== value) onValueChange(o.value);
      close();
    },
    [options, value, onValueChange, close],
  );

  // Move DOM focus into the list on open; keep the active option in view.
  useEffect(() => {
    if (open) listRef.current?.focus({ preventScroll: true });
  }, [open]);
  useEffect(() => {
    if (!open || active < 0) return;
    listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)?.scrollIntoView?.({ block: 'nearest' });
  }, [open, active]);

  const runTypeahead = (key: string, from: number) => {
    const now = Date.now();
    const t = typeahead.current;
    const fresh = now - t.at > 600;
    t.text = fresh ? key : t.text + key;
    t.at = now;
    const q = t.text.toLowerCase();
    // A fresh single letter starts after the current item; a growing prefix may keep it.
    const offset = fresh ? 1 : 0;
    for (let n = 0; n < options.length; n++) {
      const i = (from + offset + n + options.length) % options.length;
      if (enabled(i) && options[i]!.label.toLowerCase().startsWith(q)) return i;
    }
    return -1;
  };

  const isChar = (e: KeyboardEvent) => e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && e.key !== ' ';

  const onTriggerKeyDown = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      openList(e.key === 'ArrowUp' ? 'last' : undefined);
    } else if (isChar(e)) {
      const i = runTypeahead(e.key, selectedIndex);
      if (i >= 0) onValueChange(options[i]!.value);
    }
  };

  const onListKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setActive((a) => step(a, 1));
        break;
      case 'ArrowUp':
        e.preventDefault();
        setActive((a) => step(a, -1));
        break;
      case 'Home':
        e.preventDefault();
        setActive(edge(1));
        break;
      case 'End':
        e.preventDefault();
        setActive(edge(-1));
        break;
      case 'Enter':
      case ' ':
        e.preventDefault();
        choose(active);
        break;
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        close();
        break;
      case 'Tab':
        close(false);
        break;
      default:
        if (isChar(e)) {
          const i = runTypeahead(e.key, active);
          if (i >= 0) setActive(i);
        }
    }
  };

  return (
    <>
      <button
        ref={setRefs}
        type="button"
        id={id}
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledBy}
        aria-invalid={ariaInvalid}
        aria-required={required || undefined}
        disabled={disabled}
        data-state={open ? 'open' : 'closed'}
        onClick={() => (open ? close() : openList())}
        onKeyDown={onTriggerKeyDown}
        className={cn(
          'inline-flex w-full min-w-0 items-center gap-2 rounded-md border border-input bg-card text-left text-foreground',
          'transition-[border-color,box-shadow,background-color] duration-300 ease-out-expo',
          'hover:border-foreground/40',
          'focus-visible:border-foreground focus-visible:shadow-[inset_0_-2px_0_hsl(var(--signal))] focus-visible:outline-none',
          'data-[state=open]:border-foreground',
          'aria-[invalid=true]:border-destructive',
          'disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-input',
          size === 'sm' ? 'h-8 px-2.5 text-[13px]' : 'h-11 px-3.5 text-sm',
          className,
        )}
      >
        {selected?.icon ? (
          <span className="shrink-0 text-muted-foreground [&>svg]:h-4 [&>svg]:w-4">{selected.icon}</span>
        ) : null}
        <span className={cn('min-w-0 flex-1 truncate', !selected && 'text-muted-foreground/80')}>
          {selected ? selected.label : placeholder}
        </span>
        <ChevronDown
          aria-hidden
          className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-out-expo',
            open && 'rotate-180',
          )}
        />
      </button>
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <Popover open={open} onClose={closeNoFocus} anchorRef={triggerRef} align={align} matchWidth className={popoverClassName}>
        <div
          ref={listRef}
          id={listId}
          role="listbox"
          tabIndex={-1}
          aria-label={ariaLabel}
          aria-labelledby={ariaLabelledBy}
          aria-activedescendant={active >= 0 ? `${uid}-opt-${active}` : undefined}
          onKeyDown={onListKeyDown}
          className="outline-none"
        >
          {options.length === 0 ? <div className="px-2.5 py-2 text-[13px] text-muted-foreground">No options</div> : null}
          {options.map((o, i) => {
            const isSel = i === selectedIndex;
            return (
              <div
                key={`${o.value}-${i}`}
                id={`${uid}-opt-${i}`}
                role="option"
                data-index={i}
                aria-selected={isSel}
                aria-disabled={o.disabled || undefined}
                data-active={i === active}
                onPointerMove={() => !o.disabled && i !== active && setActive(i)}
                onClick={() => choose(i)}
                className={cn(
                  'flex cursor-pointer select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-sm',
                  'data-[active=true]:bg-secondary',
                  isSel && 'font-medium',
                  o.disabled && 'cursor-not-allowed opacity-45',
                )}
              >
                {o.icon ? <span className="shrink-0 text-muted-foreground [&>svg]:h-4 [&>svg]:w-4">{o.icon}</span> : null}
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{o.label}</span>
                  {o.description ? (
                    <span className="block truncate text-xs font-normal text-muted-foreground">{o.description}</span>
                  ) : null}
                </span>
                <Check aria-hidden className={cn('h-4 w-4 shrink-0 text-signal-ink', !isSel && 'invisible')} />
              </div>
            );
          })}
        </div>
      </Popover>
    </>
  );
});
