import { cn } from '@/lib/utils';

interface SpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const sizes = {
  sm: 'h-3 gap-[2px] [&>span]:w-[2px]',
  md: 'h-4 gap-[3px] [&>span]:w-[3px]',
  lg: 'h-7 gap-1 [&>span]:w-1',
};

/** Loading indicator: four bouncing columns echoing the logo mark. */
export function Spinner({ size = 'md', className }: SpinnerProps) {
  return (
    <span className={cn('inline-flex items-end', sizes[size], className)} role="status" aria-label="Loading">
      {[0, 1, 2, 3].map((i) => (
        <span
          key={i}
          className={cn('h-full origin-bottom animate-bar-bounce rounded-[1px] bg-current', i === 3 && 'text-signal')}
          style={{ animationDelay: `${i * 120}ms` }}
        />
      ))}
      <span className="sr-only">Loading…</span>
    </span>
  );
}
