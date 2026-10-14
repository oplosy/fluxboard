'use client';

import { cloneElement, isValidElement, useId, type ReactElement, type ReactNode } from 'react';

import { cn } from '@/lib/utils';
import { Label } from './label';

export interface FieldProps {
  label?: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  /** Appends a muted "(optional)" to the label. */
  optional?: boolean;
  /** Override the generated control id. */
  htmlFor?: string;
  className?: string;
  /**
   * A single control (Input, Select, Textarea, ...). Field injects `id`,
   * `aria-describedby` and `aria-invalid` into it unless already set.
   */
  children: ReactElement<{ id?: string; 'aria-describedby'?: string; 'aria-invalid'?: boolean | 'true' | 'false' }> | ReactNode;
}

/** Label + control + hint/error wrapper for forms. */
export function Field({ label, hint, error, optional, htmlFor, className, children }: FieldProps) {
  const uid = useId();
  const childId = isValidElement<{ id?: string }>(children) ? children.props.id : undefined;
  const id = htmlFor ?? childId ?? `${uid}-control`;
  const hintId = hint && !error ? `${uid}-hint` : undefined;
  const errorId = error ? `${uid}-error` : undefined;
  const describedBy = errorId ?? hintId;

  const control = isValidElement<{ id?: string; 'aria-describedby'?: string; 'aria-invalid'?: boolean | 'true' | 'false' }>(
    children,
  )
    ? cloneElement(children, {
        id: children.props.id ?? id,
        'aria-describedby': children.props['aria-describedby'] ?? describedBy,
        'aria-invalid': children.props['aria-invalid'] ?? (error ? true : undefined),
      })
    : children;

  return (
    <div className={cn('space-y-1.5', className)}>
      {label ? (
        <Label htmlFor={id}>
          {label}
          {optional ? <span className="ml-1 font-normal text-muted-foreground">(optional)</span> : null}
        </Label>
      ) : null}
      {control}
      {error ? (
        <p id={errorId} role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : hint ? (
        <p id={hintId} className="text-xs text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
