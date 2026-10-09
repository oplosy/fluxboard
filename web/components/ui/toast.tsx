'use client';

import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { X, CheckCircle2, AlertCircle, Info } from 'lucide-react';

import { cn } from '@/lib/utils';

type ToastVariant = 'default' | 'success' | 'error';

interface Toast {
  id: number;
  title: string;
  description?: string;
  variant: ToastVariant;
  leaving: boolean;
}

interface ToastContextValue {
  toast: (t: { title: string; description?: string; variant?: ToastVariant }) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

let nextId = 1;
const LIFETIME = 5000;
const EXIT_MS = 280;

const variantIcons: Record<ToastVariant, typeof Info> = {
  default: Info,
  success: CheckCircle2,
  error: AlertCircle,
};

const accent: Record<ToastVariant, string> = {
  default: 'bg-foreground',
  success: 'bg-success',
  error: 'bg-destructive',
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const remove = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  // Mark as leaving first so the exit animation plays, then drop it.
  const dismiss = useCallback(
    (id: number) => {
      setToasts((prev) => prev.map((t) => (t.id === id ? { ...t, leaving: true } : t)));
      window.setTimeout(() => remove(id), EXIT_MS);
    },
    [remove],
  );

  const toast = useCallback<ToastContextValue['toast']>(({ title, description, variant = 'default' }) => {
    const id = nextId++;
    setToasts((prev) => [...prev.slice(-4), { id, title, description, variant, leaving: false }]);
  }, []);

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div
        className="pointer-events-none fixed bottom-0 right-0 z-[95] flex w-full max-w-sm flex-col gap-2 p-4"
        role="region"
        aria-label="Notifications"
        aria-live="polite"
      >
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onDismiss={() => dismiss(t.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastItem({ toast: t, onDismiss }: { toast: Toast; onDismiss: () => void }) {
  const Icon = variantIcons[t.variant];
  const [paused, setPaused] = useState(false);
  const remaining = useRef(LIFETIME);

  // Auto-dismiss; hovering pauses the countdown (and its progress bar) and
  // resumes with whatever time was left.
  useEffect(() => {
    if (paused || t.leaving) return;
    const started = performance.now();
    const id = window.setTimeout(onDismiss, remaining.current);
    return () => {
      window.clearTimeout(id);
      remaining.current = Math.max(0, remaining.current - (performance.now() - started));
    };
  }, [paused, t.leaving, onDismiss]);

  return (
    <div
      onPointerEnter={() => setPaused(true)}
      onPointerLeave={() => setPaused(false)}
      className={cn(
        'glass pointer-events-auto relative flex items-start gap-3 overflow-hidden rounded-lg p-4 pl-5',
        t.leaving
          ? 'translate-x-[110%] opacity-0 transition-[transform,opacity] duration-300 ease-in-out-quart'
          : 'animate-[toast-in_0.55s_var(--ease-spring)_backwards]',
      )}
      role={t.variant === 'error' ? 'alert' : 'status'}
    >
      <span className={cn('absolute inset-y-0 left-0 w-1', accent[t.variant])} aria-hidden />
      <Icon
        className={cn(
          'mt-0.5 h-4 w-4 shrink-0',
          t.variant === 'success' && 'text-success',
          t.variant === 'error' && 'text-destructive',
          t.variant === 'default' && 'text-foreground',
        )}
      />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">{t.title}</p>
        {t.description ? <p className="mt-1 text-sm text-muted-foreground">{t.description}</p> : null}
      </div>
      <button
        onClick={onDismiss}
        className="shrink-0 rounded-sm p-0.5 text-muted-foreground transition-[color,transform] duration-200 hover:rotate-90 hover:text-foreground"
        aria-label="Dismiss"
      >
        <X className="h-3.5 w-3.5" />
      </button>
      <span
        aria-hidden
        className={cn('absolute bottom-0 left-0 h-[2px] w-full origin-left', accent[t.variant], 'opacity-40')}
        style={{
          animation: `toast-life ${LIFETIME}ms linear forwards`,
          animationPlayState: paused || t.leaving ? 'paused' : 'running',
        }}
      />
    </div>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within <ToastProvider>');
  return ctx;
}
