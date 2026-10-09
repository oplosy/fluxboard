'use client';

import { useRef, useState, type ReactNode } from 'react';

import { usePresence } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

interface TooltipProps {
  content: string;
  children: ReactNode;
  side?: 'top' | 'bottom' | 'right';
  className?: string;
  wrapperClassName?: string;
}

/** Hover/focus tooltip with a short intent delay and an exit fade. */
export function Tooltip({ content, children, side = 'top', className, wrapperClassName }: TooltipProps) {
  const [open, setOpen] = useState(false);
  const timer = useRef<number>();
  const { mounted, state } = usePresence(open, 150);

  const show = () => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setOpen(true), 180);
  };
  const hide = () => {
    window.clearTimeout(timer.current);
    setOpen(false);
  };

  return (
    <div className={cn('relative inline-flex', wrapperClassName)} onMouseEnter={show} onMouseLeave={hide} onFocus={show} onBlur={hide}>
      {children}
      {mounted ? (
        <div
          role="tooltip"
          data-state={state}
          className={cn(
            'pointer-events-none absolute z-50 whitespace-nowrap rounded-sm bg-foreground px-2 py-1 font-mono text-[11px] text-background shadow-lg',
            'transition-[opacity,transform] duration-150 ease-out-expo',
            'data-[state=closed]:opacity-0 data-[state=open]:opacity-100',
            side === 'top' && 'bottom-full left-1/2 mb-2 -translate-x-1/2 data-[state=closed]:translate-y-1',
            side === 'bottom' && 'left-1/2 top-full mt-2 -translate-x-1/2 data-[state=closed]:-translate-y-1',
            side === 'right' && 'left-full top-1/2 ml-3 -translate-y-1/2 data-[state=closed]:-translate-x-1',
            className,
          )}
        >
          {content}
        </div>
      ) : null}
    </div>
  );
}
