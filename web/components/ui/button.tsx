'use client';

import { forwardRef, type ButtonHTMLAttributes, type PointerEvent as ReactPointerEvent } from 'react';

import { cn } from '@/lib/utils';
import { Spinner } from './spinner';

type Variant = 'default' | 'signal' | 'secondary' | 'outline' | 'ghost' | 'destructive';
type Size = 'default' | 'sm' | 'lg' | 'xl' | 'icon';

// Each solid variant has a "sweep" layer that wipes up from the bottom edge
// on hover; text sits above it (z-10) so contrast never changes mid-sweep.
const variants: Record<Variant, { base: string; sweep?: string }> = {
  default: { base: 'bg-primary text-primary-foreground', sweep: 'bg-signal' },
  signal: { base: 'bg-signal text-signal-foreground', sweep: 'bg-primary' },
  secondary: { base: 'bg-secondary text-secondary-foreground', sweep: 'bg-foreground/10' },
  outline: { base: 'border border-foreground/20 bg-transparent text-foreground hover:border-foreground/60', sweep: 'bg-secondary' },
  ghost: { base: 'text-foreground/80 hover:bg-secondary hover:text-foreground' },
  destructive: { base: 'bg-destructive text-destructive-foreground', sweep: 'bg-foreground/15' },
};

// Text colour after the sweep has covered the button.
const sweptText: Partial<Record<Variant, string>> = {
  default: 'hover:text-signal-foreground',
  signal: 'hover:text-primary-foreground',
};

const sizes: Record<Size, string> = {
  default: 'h-10 px-4 text-sm',
  sm: 'h-8 px-3 text-[13px]',
  lg: 'h-12 px-6 text-[15px]',
  xl: 'h-14 px-8 text-base',
  icon: 'h-10 w-10',
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
  isLoading?: boolean;
  /** Roll the label on hover (string children only). Defaults on for lg/xl. */
  roll?: boolean;
}

function spawnRipple(e: ReactPointerEvent<HTMLButtonElement>) {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  const btn = e.currentTarget;
  const r = btn.getBoundingClientRect();
  const size = Math.max(r.width, r.height) * 2.4;
  const dot = document.createElement('span');
  dot.className = 'ripple';
  dot.style.width = dot.style.height = `${size}px`;
  dot.style.left = `${e.clientX - r.left}px`;
  dot.style.top = `${e.clientY - r.top}px`;
  btn.appendChild(dot);
  dot.addEventListener('animationend', () => dot.remove(), { once: true });
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = 'default',
      size = 'default',
      type = 'button',
      isLoading,
      roll,
      children,
      disabled,
      onPointerDown,
      ...props
    },
    ref,
  ) => {
    const v = variants[variant];
    const canRoll = typeof children === 'string' && !isLoading && (roll ?? (size === 'lg' || size === 'xl'));
    return (
      <button
        ref={ref}
        type={type}
        disabled={disabled || isLoading}
        aria-busy={isLoading || undefined}
        onPointerDown={(e) => {
          if (!disabled && !isLoading) spawnRipple(e);
          onPointerDown?.(e);
        }}
        className={cn(
          'group relative isolate inline-flex select-none items-center justify-center gap-2 overflow-hidden whitespace-nowrap rounded-md font-medium tracking-[-0.01em]',
          'transition-[transform,color,background-color,border-color,box-shadow] duration-250 ease-out-expo',
          'active:scale-[0.97] active:duration-100',
          'disabled:pointer-events-none disabled:opacity-45',
          v.base,
          sweptText[variant],
          sizes[size],
          className,
        )}
        {...props}
      >
        {v.sweep ? (
          <span
            aria-hidden
            className={cn(
              'absolute inset-0 -z-10 origin-bottom scale-y-0 transition-transform duration-400 ease-out-expo group-hover:scale-y-100',
              v.sweep,
            )}
          />
        ) : null}
        {isLoading ? <Spinner size="sm" /> : null}
        {canRoll ? (
          <span className="roll">
            <span>{children}</span>
            <span aria-hidden>{children}</span>
          </span>
        ) : (
          children
        )}
      </button>
    );
  },
);
Button.displayName = 'Button';
