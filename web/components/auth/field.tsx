import { Children, cloneElement, isValidElement, type MouseEvent, type ReactElement, type ReactNode } from 'react';
import { AlertCircle, CheckCircle2, Info } from 'lucide-react';

import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

// A labelled form field shared by the auth forms. A problem with the value is
// shown as a speech bubble anchored under the input (with an optional action,
// e.g. "Sign in instead"), above any hint; the input itself is marked
// aria-invalid and described by both so screen readers announce them.
export function Field({
  id,
  label,
  error,
  errorAction,
  hint,
  labelAside,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  errorAction?: ReactNode;
  hint?: ReactNode;
  labelAside?: ReactNode;
  children: ReactNode;
}) {
  const errorId = `${id}-error`;
  const hintId = `${id}-hint`;
  const describedBy = [error ? errorId : null, hint ? hintId : null].filter(Boolean).join(' ') || undefined;

  // Wire a11y onto the control (first element child) without each page
  // repeating aria-invalid / aria-describedby.
  const items = Children.toArray(children);
  const control = items.findIndex((c) => isValidElement(c));
  const content = items.map((child, i) =>
    i === control
      ? cloneElement(child as ReactElement<Record<string, unknown>>, {
          'aria-invalid': error ? true : undefined,
          'aria-describedby': describedBy,
        })
      : child,
  );

  return (
    <div className="space-y-2">
      <div className="flex items-baseline justify-between gap-3">
        <Label htmlFor={id}>{label}</Label>
        {labelAside}
      </div>
      {content}
      {error ? <FieldBubble id={errorId} message={error} action={errorAction} /> : null}
      {hint ? (
        <div id={hintId} className="text-[12.5px] text-muted-foreground">
          {hint}
        </div>
      ) : null}
    </div>
  );
}

// Opaque card base + an inset-shadow tint, so the arrow and the bubble body
// overlap without the seam a translucent background would show.
const BUBBLE_FILL = 'bg-card shadow-[inset_0_0_0_999px_hsl(var(--destructive)/0.07)]';

// The error bubble: a tinted card with an arrow pointing up at the input.
// Re-keyed on the message so a different problem re-plays the entrance.
export function FieldBubble({ id, message, action }: { id?: string; message: string; action?: ReactNode }) {
  return (
    <div key={message} className="relative animate-slide-down pt-1.5">
      <div
        id={id}
        role="alert"
        className={cn(
          'flex items-start gap-2 rounded-md border border-destructive/30 px-3 py-2 text-[13px] leading-snug text-destructive shadow-sm',
          BUBBLE_FILL,
        )}
      >
        <AlertCircle className="mt-[1px] h-3.5 w-3.5 shrink-0" aria-hidden />
        <div className="min-w-0 flex-1">
          <p>{message}</p>
          {action ? <div className="mt-1 text-foreground">{action}</div> : null}
        </div>
      </div>
      {/* Arrow: same opaque fill as the bubble, drawn over its top border. */}
      <span
        aria-hidden
        className={cn('absolute left-4 top-[2px] h-2.5 w-2.5 rotate-45 border-l border-t border-destructive/30', BUBBLE_FILL)}
      />
    </div>
  );
}

// A top-of-form error banner for non-field errors (bad credentials, rate limits).
// Re-keyed on the message so a repeated failure shakes again.
export function FormError({ message, action }: { message?: string | null; action?: ReactNode }) {
  if (!message) return null;
  return (
    <div
      key={message}
      role="alert"
      className="flex animate-[shake_0.5s_var(--ease-out-expo)] items-start gap-2 rounded-md border border-destructive/30 bg-destructive/[0.07] p-3 text-sm text-destructive"
    >
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
      <div className="min-w-0 flex-1">
        <p>{message}</p>
        {action ? <div className="mt-1.5 font-medium">{action}</div> : null}
      </div>
    </div>
  );
}

// A neutral or success banner (e.g. "Password updated — sign in").
export function FormNotice({
  message,
  variant = 'info',
}: {
  message?: string | null;
  variant?: 'info' | 'success';
}) {
  if (!message) return null;
  const Icon = variant === 'success' ? CheckCircle2 : Info;
  return (
    <div
      role="status"
      className={cn(
        'flex animate-slide-down items-start gap-2 rounded-md border p-3 text-sm',
        variant === 'success'
          ? 'border-success/30 bg-success/[0.08] text-foreground'
          : 'border-foreground/15 bg-secondary text-foreground',
      )}
    >
      <Icon className={cn('mt-0.5 h-4 w-4 shrink-0', variant === 'success' ? 'text-success' : 'text-muted-foreground')} />
      <span>{message}</span>
    </div>
  );
}

// onMouseDown for a form's submit button: keeps focus in the field so its blur
// validation can't pop a bubble, push the button down and swallow the click.
// Keyboard and touch activation are unaffected.
export function keepFocus(e: MouseEvent<HTMLElement>) {
  e.preventDefault();
}
