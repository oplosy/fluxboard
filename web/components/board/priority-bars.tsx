import { priorityLabel, priorityLevel } from '@/lib/board/priority';
import type { Priority } from '@/lib/api/types';
import { cn } from '@/lib/utils';

/** Signal-strength glyph: 1–4 rising bars, urgent lights up in the signal colour. */
export function PriorityBars({ priority, className }: { priority: Priority; className?: string }) {
  const level = priorityLevel(priority);
  if (level === 0) return null;
  return (
    <span className={cn('inline-flex h-3 items-end gap-[2px]', className)} title={priorityLabel(priority)} aria-label={priorityLabel(priority)} role="img">
      {[1, 2, 3, 4].map((i) => (
        <span
          key={i}
          className={cn(
            'w-[3px] rounded-[1px] transition-colors duration-300',
            i <= level ? (level === 4 ? 'bg-signal' : 'bg-foreground') : 'bg-foreground/15',
          )}
          style={{ height: `${25 + i * 18.75}%` }}
        />
      ))}
    </span>
  );
}
