'use client';

import { useEffect } from 'react';
import { RotateCcw } from 'lucide-react';

import { ApiError } from '@/lib/api/client';
import { Button } from '@/components/ui/button';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // eslint-disable-next-line no-console
    console.error(error);
  }, [error]);

  const forbidden = error instanceof ApiError && error.status === 403;

  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-6 p-6 text-center">
      {/* Broken column glyph: the last bar falls over. */}
      <div className="flex h-14 items-end gap-1.5" aria-hidden>
        {[0.5, 0.85, 0.35].map((h, i) => (
          <span
            key={i}
            className="w-3 animate-[slide-up_0.5s_var(--ease-out-expo)_backwards] rounded-[2px] bg-foreground/80"
            style={{ height: `${h * 100}%`, animationDelay: `${i * 80}ms` }}
          />
        ))}
        <span className="h-[65%] w-3 origin-bottom-left animate-[topple_1s_var(--ease-spring)_0.4s_both] rounded-[2px] bg-destructive" />
      </div>
      <div className="animate-[slide-up_0.6s_var(--ease-out-expo)_backwards] [animation-delay:150ms]">
        <p className="font-mono text-[11px] uppercase tracking-[0.2em] text-muted-foreground">{forbidden ? '403' : 'Error'}</p>
        <h1 className="mt-2 text-2xl font-semibold tracking-[-0.03em]">
          {forbidden ? "You don't have access" : 'Something went wrong'}
        </h1>
        <p className="mt-2 max-w-md text-sm text-muted-foreground">
          {forbidden
            ? 'Your account lacks permission for this resource. Ask an admin for access.'
            : error.message || 'An unexpected error occurred.'}
        </p>
      </div>
      <Button variant="outline" onClick={reset} className="gap-2">
        <RotateCcw className="h-4 w-4 transition-transform duration-500 ease-spring group-hover:-rotate-180" />
        Try again
      </Button>
    </div>
  );
}
