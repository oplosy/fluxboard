'use client';

import {
  cloneElement,
  createContext,
  isValidElement,
  useCallback,
  useContext,
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEvent,
  type ReactElement,
  type ReactNode,
} from 'react';

import { cn } from '@/lib/utils';
import { Popover, type PopoverAlign } from './popover';

const MenuContext = createContext<{ close: () => void } | null>(null);

export interface MenuProps {
  /** A single button-like element; Menu wires up its ref-less click/keyboard handlers. */
  trigger: ReactElement<{
    onClick?: (e: MouseEvent<HTMLElement>) => void;
    onKeyDown?: (e: KeyboardEvent<HTMLElement>) => void;
    'aria-haspopup'?: string;
    'aria-expanded'?: boolean;
    'aria-controls'?: string;
  }>;
  children: ReactNode;
  align?: PopoverAlign;
  className?: string;
  /** Accessible name for the menu. */
  'aria-label'?: string;
}

/**
 * Dropdown menu for action buttons. Items are `MenuItem`s (and `MenuSeparator`s).
 * Arrow keys / Home / End move focus, Enter/Space activate, Escape closes and
 * returns focus to the trigger. Rendered in a portal.
 */
export function Menu({ trigger, children, align = 'start', className, 'aria-label': ariaLabel }: MenuProps) {
  const uid = useId();
  const [open, setOpen] = useState(false);
  // Wrapper only provides a stable anchor; it adds no layout of its own.
  const anchorRef = useRef<HTMLSpanElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const close = useCallback(() => setOpen(false), []);
  const closeAndFocus = useCallback(() => {
    setOpen(false);
    anchorRef.current?.querySelector<HTMLElement>('button, [tabindex]')?.focus();
  }, []);

  const items = () => Array.from(listRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]:not([aria-disabled="true"])') ?? []);

  useEffect(() => {
    if (open) items()[0]?.focus();
  }, [open]);

  const onItemsKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const list = items();
    const i = list.indexOf(document.activeElement as HTMLElement);
    const go = (n: number) => {
      e.preventDefault();
      list[(n + list.length) % list.length]?.focus();
    };
    switch (e.key) {
      case 'ArrowDown':
        go(i + 1);
        break;
      case 'ArrowUp':
        go(i < 0 ? -1 : i - 1);
        break;
      case 'Home':
        go(0);
        break;
      case 'End':
        go(-1);
        break;
      case 'Escape':
        e.preventDefault();
        e.stopPropagation();
        closeAndFocus();
        break;
      case 'Tab':
        setOpen(false);
        break;
    }
  };

  const anchored = isValidElement(trigger)
    ? cloneElement(trigger, {
        'aria-haspopup': 'menu',
        'aria-expanded': open,
        'aria-controls': open ? `${uid}-menu` : undefined,
        onClick: (e: MouseEvent<HTMLElement>) => {
          trigger.props.onClick?.(e);
          if (!e.defaultPrevented) setOpen((o) => !o);
        },
        onKeyDown: (e: KeyboardEvent<HTMLElement>) => {
          trigger.props.onKeyDown?.(e);
          if (e.key === 'ArrowDown' && !e.defaultPrevented) {
            e.preventDefault();
            setOpen(true);
          }
        },
      })
    : trigger;

  return (
    <MenuContext.Provider value={{ close: closeAndFocus }}>
      <span ref={anchorRef} className="inline-flex">
        {anchored}
      </span>
      <Popover open={open} onClose={close} anchorRef={anchorRef} align={align} className={cn('min-w-[11rem]', className)}>
        <div id={`${uid}-menu`} ref={listRef} role="menu" aria-label={ariaLabel} onKeyDown={onItemsKeyDown}>
          {children}
        </div>
      </Popover>
    </MenuContext.Provider>
  );
}

export interface MenuItemProps {
  onSelect?: () => void;
  icon?: ReactNode;
  destructive?: boolean;
  disabled?: boolean;
  children: ReactNode;
  className?: string;
}

export function MenuItem({ onSelect, icon, destructive, disabled, children, className }: MenuItemProps) {
  const ctx = useContext(MenuContext);
  return (
    <button
      type="button"
      role="menuitem"
      tabIndex={-1}
      aria-disabled={disabled || undefined}
      onClick={() => {
        if (disabled) return;
        onSelect?.();
        ctx?.close();
      }}
      className={cn(
        'flex w-full select-none items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-sm outline-none',
        'transition-colors duration-150',
        'hover:bg-secondary focus-visible:bg-secondary focus-visible:outline-none',
        destructive && 'text-destructive hover:bg-destructive/10 focus-visible:bg-destructive/10',
        disabled && 'cursor-not-allowed opacity-45 hover:bg-transparent',
        className,
      )}
    >
      {icon ? (
        <span className={cn('shrink-0 [&>svg]:h-4 [&>svg]:w-4', !destructive && 'text-muted-foreground')}>{icon}</span>
      ) : null}
      <span className="min-w-0 flex-1 truncate">{children}</span>
    </button>
  );
}

export function MenuSeparator() {
  return <div role="separator" className="-mx-1 my-1 h-px bg-border" />;
}

export function MenuLabel({ children }: { children: ReactNode }) {
  return <div className="px-2.5 pb-1 pt-1.5 text-xs font-medium text-muted-foreground">{children}</div>;
}
