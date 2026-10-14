import type { ReactNode } from 'react';

import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

// A settings section: title + description on the left (top on mobile), the
// controls on the right. `action` sits next to the title (e.g. "New field").
export function SettingsSection({
  title,
  description,
  action,
  danger,
  children,
  className,
}: {
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  danger?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn('p-5 sm:p-6', danger && 'border-destructive/40', className)}>
      <section className="grid gap-x-10 gap-y-5 lg:grid-cols-[14rem_minmax(0,1fr)]">
        <header className="space-y-1.5">
          <div className="flex items-center justify-between gap-3 lg:block">
            <h2 className={cn('text-base font-semibold tracking-[-0.02em]', danger && 'text-destructive')}>{title}</h2>
            {action ? <div className="lg:mt-3">{action}</div> : null}
          </div>
          {description ? <p className="text-[13px] leading-relaxed text-muted-foreground">{description}</p> : null}
        </header>
        <div className="min-w-0">{children}</div>
      </section>
    </Card>
  );
}

// Bordered list container for editor rows.
export function RowList({ children }: { children: ReactNode }) {
  return <ul className="divide-y divide-border overflow-hidden rounded-md border border-border bg-card">{children}</ul>;
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="rounded-md border border-dashed border-border px-4 py-8 text-center">
      <p className="text-sm font-medium">{title}</p>
      {children ? <p className="mx-auto mt-1 max-w-sm text-[13px] text-muted-foreground">{children}</p> : null}
    </div>
  );
}

// Inline confirmation strip shown under a row before a destructive action.
export function ConfirmStrip({
  children,
  confirmLabel,
  onConfirm,
  onCancel,
  disabled,
}: {
  children: ReactNode;
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
  disabled?: boolean;
}) {
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center gap-2 border-t border-destructive/25 bg-destructive/5 px-3 py-2.5 text-[13px]"
    >
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">{children}</div>
      <Button size="sm" variant="destructive" onClick={onConfirm} disabled={disabled}>
        {confirmLabel}
      </Button>
      <Button size="sm" variant="ghost" onClick={onCancel}>
        Cancel
      </Button>
    </div>
  );
}
