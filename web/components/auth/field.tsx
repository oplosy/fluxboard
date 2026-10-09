import type { ReactNode } from 'react';
import { AlertCircle } from 'lucide-react';

import { Label } from '@/components/ui/label';

// A labelled form field with an optional inline error, shared by the auth forms.
export function Field({
  id,
  label,
  error,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {error ? <p className="animate-slide-down text-[13px] text-destructive">{error}</p> : null}
    </div>
  );
}

// A top-of-form error banner for non-field errors (bad credentials, rate limits).
// Re-keyed on the message so a repeated failure shakes again.
export function FormError({ message }: { message?: string | null }) {
  if (!message) return null;
  return (
    <div
      key={message}
      role="alert"
      className="flex animate-[shake_0.5s_var(--ease-out-expo)] items-start gap-2 rounded-md border border-destructive/30 bg-destructive/[0.07] p-3 text-sm text-destructive"
    >
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  );
}
