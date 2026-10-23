'use client';

import { useRef, type ReactNode } from 'react';

import { useInView } from '@/lib/motion/hooks';

// Mounts its children (and so their queries) only once the section scrolls
// near the viewport: the task detail opens with fewer requests, and the lower
// sections still load before the reader reaches them.
export function LazySection({ children, minHeight = 96 }: { children: ReactNode; minHeight?: number }) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { rootMargin: '300px 0px', threshold: 0 });
  return (
    <div ref={ref} style={inView ? undefined : { minHeight }}>
      {inView ? children : null}
    </div>
  );
}
