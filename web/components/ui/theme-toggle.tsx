'use client';

import { Sun, Moon, Monitor } from 'lucide-react';
import { useRef, useState, type MouseEvent as ReactMouseEvent } from 'react';
import { flushSync } from 'react-dom';

import { useTheme } from './theme-provider';
import { usePresence } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

type Theme = 'light' | 'dark' | 'system';

const options = [
  { value: 'light' as const, icon: Sun, label: 'Light' },
  { value: 'dark' as const, icon: Moon, label: 'Dark' },
  { value: 'system' as const, icon: Monitor, label: 'System' },
];

type ViewTransitionDoc = Document & {
  startViewTransition?: (cb: () => void) => { ready: Promise<void> };
};

/**
 * Apply a theme with a circular wipe expanding from the click point
 * (View Transitions API). Falls back to an instant switch.
 */
function useAnimatedSetTheme() {
  const { setTheme } = useTheme();
  return (next: Theme, e?: ReactMouseEvent) => {
    const doc = document as ViewTransitionDoc;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!doc.startViewTransition || reduced || !e) {
      setTheme(next);
      return;
    }
    const x = e.clientX;
    const y = e.clientY;
    const r = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y));
    const vt = doc.startViewTransition(() => {
      flushSync(() => setTheme(next));
    });
    void vt.ready.then(() => {
      document.documentElement.animate(
        { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${r}px at ${x}px ${y}px)`] },
        { duration: 650, easing: 'cubic-bezier(0.76, 0, 0.24, 1)', pseudoElement: '::view-transition-new(root)' },
      );
    });
  };
}

/** Theme control. `compact` cycles light → dark → system on click; the full
 *  variant opens a small menu. Both switch with a circular reveal. */
export function ThemeToggle({ compact = false }: { compact?: boolean }) {
  const { theme, resolvedTheme } = useTheme();
  const apply = useAnimatedSetTheme();
  const [open, setOpen] = useState(false);
  const { mounted, state } = usePresence(open, 180);
  const btn = useRef<HTMLButtonElement>(null);

  const isDark = resolvedTheme === 'dark';
  const icon = (
    <span className="relative h-4 w-4">
      <Sun
        className={cn(
          'absolute inset-0 h-4 w-4 transition-all duration-500 ease-spring',
          isDark ? 'rotate-90 scale-0 opacity-0' : 'rotate-0 scale-100 opacity-100',
        )}
      />
      <Moon
        className={cn(
          'absolute inset-0 h-4 w-4 transition-all duration-500 ease-spring',
          isDark ? 'rotate-0 scale-100 opacity-100' : '-rotate-90 scale-0 opacity-0',
        )}
      />
    </span>
  );

  const buttonClass =
    'flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground';

  if (compact) {
    const cycle = (e: ReactMouseEvent) => {
      const next: Theme = theme === 'light' ? 'dark' : theme === 'dark' ? 'system' : 'light';
      apply(next, e);
    };
    return (
      <button
        type="button"
        onClick={cycle}
        className={buttonClass}
        aria-label={`Switch theme (current: ${theme})`}
        title={`Theme: ${theme}`}
      >
        {icon}
      </button>
    );
  }

  return (
    <div className="relative">
      <button
        ref={btn}
        type="button"
        onClick={() => setOpen((v) => !v)}
        className={buttonClass}
        aria-label="Toggle theme menu"
        aria-expanded={open}
      >
        {icon}
      </button>

      {mounted ? (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} aria-hidden />
          <div
            data-state={state}
            className={cn(
              'glass absolute bottom-full left-0 z-20 mb-2 min-w-36 origin-bottom-left rounded-md p-1',
              'transition-[opacity,transform] duration-200 ease-out-expo',
              'data-[state=closed]:translate-y-1 data-[state=closed]:scale-95 data-[state=closed]:opacity-0',
            )}
          >
            {options.map((opt) => {
              const Icon = opt.icon;
              const active = theme === opt.value;
              return (
                <button
                  key={opt.value}
                  type="button"
                  onClick={(e) => {
                    apply(opt.value, e);
                    setOpen(false);
                  }}
                  className={cn(
                    'flex w-full items-center gap-2 rounded-sm px-2.5 py-1.5 text-sm transition-colors',
                    active ? 'bg-secondary font-medium text-foreground' : 'text-muted-foreground hover:bg-secondary/60 hover:text-foreground',
                  )}
                >
                  <Icon className="h-3.5 w-3.5" />
                  {opt.label}
                  {active ? <span className="ml-auto h-1.5 w-1.5 rounded-full bg-signal" /> : null}
                </button>
              );
            })}
          </div>
        </>
      ) : null}
    </div>
  );
}
