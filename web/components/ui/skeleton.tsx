import { cn } from '@/lib/utils';

/** Placeholder block with a diagonal sheen sweeping across it. */
export function Skeleton({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'animate-shimmer rounded-sm bg-[length:220%_100%]',
        'bg-[linear-gradient(105deg,hsl(var(--muted))_35%,hsl(var(--foreground)/0.04)_50%,hsl(var(--muted))_65%)]',
        className,
      )}
      {...props}
    />
  );
}

/** Common preset skeletons for quick use. */
export function SkeletonLine({ className }: { className?: string }) {
  return <Skeleton className={cn('h-4 w-full', className)} />;
}

export function SkeletonCard({ className }: { className?: string }) {
  return (
    <div className={cn('space-y-3 rounded-lg border bg-card p-6', className)}>
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-4/5" />
    </div>
  );
}
