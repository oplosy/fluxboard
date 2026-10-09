import Link from 'next/link';

import { cn } from '@/lib/utils';

// The mark is four kanban columns of different heights; the last one carries
// the signal colour ("done"). On hover (or when `animated`) the bars bounce
// like a level meter.
export function LogoMark({ className, animated = false }: { className?: string; animated?: boolean }) {
  const bars = [
    { x: 1, h: 10 },
    { x: 7, h: 16 },
    { x: 13, h: 7 },
    { x: 19, h: 13 },
  ];
  return (
    <svg viewBox="0 0 24 24" className={cn('shrink-0', className)} aria-hidden>
      {bars.map((b, i) => (
        <rect
          key={b.x}
          x={b.x}
          y={20 - b.h}
          width={4}
          height={b.h}
          rx={1}
          className={cn(
            'origin-bottom [transform-box:fill-box]',
            i === 3 ? 'fill-signal' : 'fill-current',
            animated ? 'animate-bar-bounce' : 'group-hover/logo:animate-bar-bounce',
          )}
          style={{ animationDelay: `${i * 110}ms` }}
        />
      ))}
      <rect x={0} y={21.5} width={24} height={1.5} rx={0.75} className="fill-current opacity-40" />
    </svg>
  );
}

export function Logo({
  href = '/',
  className,
  showWordmark = true,
}: {
  href?: string;
  className?: string;
  showWordmark?: boolean;
}) {
  return (
    <Link href={href} className={cn('group/logo inline-flex items-center gap-2.5', className)} aria-label="Fluxboard home">
      <LogoMark className="h-6 w-6" />
      {showWordmark ? (
        <span className="font-display text-[17px] font-semibold tracking-[-0.03em]">fluxboard</span>
      ) : null}
    </Link>
  );
}
