'use client';

import { useState, type ReactNode } from 'react';
import { Check, Copy } from 'lucide-react';

import { Button } from '@/components/ui/button';

/** Copy text to the clipboard; reports success. */
export function useCopy(): [boolean, (text: string) => void] {
  const [copied, setCopied] = useState(false);
  function copy(text: string) {
    // navigator.clipboard is undefined in insecure contexts; guard so the `.then`
    // doesn't throw synchronously on `undefined`.
    if (!navigator.clipboard) return;
    navigator.clipboard.writeText(text).then(
      () => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      },
      () => undefined,
    );
  }
  return [copied, copy];
}

/** A one-time secret (API or agent key) with a copy button and a dismiss link. */
export function SecretReveal({
  secret,
  onDismiss,
  children,
}: {
  secret: string;
  onDismiss: () => void;
  children?: ReactNode;
}) {
  const [copied, copy] = useCopy();

  return (
    <div className="space-y-2 rounded-md border border-amber-500/40 bg-amber-500/10 p-4">
      <p className="text-sm font-medium text-amber-700 dark:text-amber-400">
        Copy your key now — you won’t be able to see it again.
      </p>
      <div className="flex items-center gap-2">
        <code className="min-w-0 flex-1 truncate rounded bg-background px-3 py-2 font-mono text-sm">{secret}</code>
        <Button size="sm" variant="outline" onClick={() => copy(secret)}>
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
          {copied ? 'Copied' : 'Copy'}
        </Button>
      </div>
      {children}
      <button onClick={onDismiss} className="text-xs text-muted-foreground hover:text-foreground">
        I’ve saved it — dismiss
      </button>
    </div>
  );
}
