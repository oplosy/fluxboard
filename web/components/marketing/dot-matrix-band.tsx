'use client';

import { DotMatrix } from '@/components/canvas/dot-matrix';
import { Reveal } from '@/components/motion/reveal';

/** Full-width ink band carrying the LED dot-matrix canvas. */
export function DotMatrixBand({ words }: { words: string[] }) {
  return (
    <Reveal variant="clip-up" className="container">
      <div className="relative h-[260px] overflow-hidden rounded-lg bg-ink md:h-[340px]">
        <DotMatrix words={words} />
        <span className="pointer-events-none absolute bottom-3 left-4 font-mono text-[10px] uppercase tracking-[0.2em] text-paper/40">
          Move your cursor · click for a ripple
        </span>
      </div>
    </Reveal>
  );
}
