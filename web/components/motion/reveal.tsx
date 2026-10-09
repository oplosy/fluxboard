'use client';

import { useRef, type CSSProperties, type ElementType, type ReactNode } from 'react';

import { useInView } from '@/lib/motion/hooks';
import { cn } from '@/lib/utils';

export type RevealVariant = 'up' | 'fade' | 'blur' | 'scale' | 'clip' | 'clip-up';

/**
 * Reveals its content on first scroll into view. `index` staggers siblings
 * (70 ms per step); `delay` adds a fixed offset in ms.
 */
export function Reveal({
  as: Tag = 'div',
  variant = 'up',
  index = 0,
  delay = 0,
  className,
  style,
  children,
  ...rest
}: {
  as?: ElementType;
  variant?: RevealVariant;
  index?: number;
  delay?: number;
  className?: string;
  style?: CSSProperties;
  children?: ReactNode;
} & Record<string, unknown>) {
  const ref = useRef<HTMLElement>(null);
  const inView = useInView(ref, { observeParent: variant === 'clip' || variant === 'clip-up', threshold: 0 });
  return (
    <Tag
      ref={ref}
      data-reveal={variant}
      className={cn(inView && 'is-in', className)}
      style={
        {
          '--i': index,
          ...(delay ? { '--reveal-delay': `${delay + index * 70}ms` } : null),
          ...style,
        } as CSSProperties
      }
      {...rest}
    >
      {children}
    </Tag>
  );
}

/**
 * Splits a string into words (or characters) that rise out of a mask when
 * scrolled into view. Screen readers get the plain string.
 */
export function SplitText({
  text,
  as: Tag = 'span',
  by = 'word',
  delay = 0,
  className,
  wordClassName,
}: {
  text: string;
  as?: ElementType;
  by?: 'word' | 'char';
  delay?: number;
  className?: string;
  wordClassName?: (word: string, index: number) => string | undefined;
}) {
  const ref = useRef<HTMLElement>(null);
  const inView = useInView(ref);
  const words = text.split(' ');
  let i = 0;
  return (
    <Tag
      ref={ref}
      aria-label={text}
      className={cn(inView && 'is-in', className)}
      style={{ '--split-delay': `${delay}ms` } as CSSProperties}
    >
      {words.map((word, w) => (
        <span key={w} aria-hidden className="split-word">
          {by === 'word' ? (
            <span className={cn('split-inner', wordClassName?.(word, w))} style={{ '--i': i++ } as CSSProperties}>
              {word}
            </span>
          ) : (
            [...word].map((ch, c) => (
              <span key={c} className="split-inner" style={{ '--i': i++ } as CSSProperties}>
                {ch}
              </span>
            ))
          )}
          {w < words.length - 1 ? ' ' : null}
        </span>
      ))}
    </Tag>
  );
}
