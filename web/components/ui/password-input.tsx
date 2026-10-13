'use client';

import { forwardRef, useState, type InputHTMLAttributes, type KeyboardEvent } from 'react';
import { ArrowBigUp, Eye, EyeOff } from 'lucide-react';

import { cn } from '@/lib/utils';
import { Input } from './input';

export type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>;

// Password field with a show/hide eye toggle and a Caps Lock warning (a common
// cause of "wrong password"). Renders the shared Input so focus rings, borders,
// and sizing stay identical to text fields.
export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(
  ({ className, onKeyDown, onKeyUp, onBlur, ...props }, ref) => {
    const [shown, setShown] = useState(false);
    const [caps, setCaps] = useState(false);
    const trackCaps = (e: KeyboardEvent<HTMLInputElement>) => setCaps(e.getModifierState('CapsLock'));

    return (
      <div className={cn('relative', className)}>
        <Input
          ref={ref}
          type={shown ? 'text' : 'password'}
          className={caps ? 'pr-[5.5rem]' : 'pr-10'}
          onKeyDown={(e) => {
            trackCaps(e);
            onKeyDown?.(e);
          }}
          onKeyUp={(e) => {
            trackCaps(e);
            onKeyUp?.(e);
          }}
          onBlur={(e) => {
            setCaps(false);
            onBlur?.(e);
          }}
          {...props}
        />
        {caps ? (
          <span
            role="status"
            className="absolute right-10 top-1/2 flex -translate-y-1/2 animate-scale-in items-center gap-0.5 rounded-sm bg-warning/15 px-1.5 py-0.5 font-mono text-[10px] font-medium uppercase tracking-[0.06em] text-foreground"
            title="Caps Lock is on"
          >
            <ArrowBigUp className="h-3 w-3" aria-hidden />
            Caps
            <span className="sr-only"> Lock is on</span>
          </span>
        ) : null}
        <button
          type="button"
          onClick={() => setShown((v) => !v)}
          aria-label={shown ? 'Hide password' : 'Show password'}
          aria-pressed={shown}
          title={shown ? 'Hide password' : 'Show password'}
          className="absolute right-1 top-1/2 flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        >
          {shown ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </div>
    );
  },
);
PasswordInput.displayName = 'PasswordInput';
