'use client';

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type HTMLAttributes,
  type RefObject,
} from 'react';
import { createPortal } from 'react-dom';

import { cn } from '@/lib/utils';

export type PopoverAlign = 'start' | 'end';

interface Position {
  top?: number;
  bottom?: number;
  left: number;
  minWidth: number;
  maxHeight: number;
  placement: 'bottom' | 'top';
}

const GAP = 6;
const MARGIN = 8;

export interface PopoverProps extends HTMLAttributes<HTMLDivElement> {
  open: boolean;
  /** Called on outside pointer-down. Escape is handled by the owner. */
  onClose: () => void;
  /** Element the popover is anchored to (also excluded from outside-click). */
  anchorRef: RefObject<HTMLElement | null>;
  align?: PopoverAlign;
  /** Match the anchor's width as a minimum. */
  matchWidth?: boolean;
  /** Upper bound for the popover height in px. */
  maxHeight?: number;
}

/**
 * Portalled, anchored overlay shared by Select and Menu. Positions under the
 * anchor, flips above when there is no room, caps its height to the viewport,
 * follows scroll/resize and closes on outside pointer-down.
 */
export function Popover({ open, ...props }: PopoverProps) {
  // Mounted only while open so position state starts fresh on every open.
  return open ? <PopoverInner {...props} /> : null;
}

function PopoverInner({
  onClose,
  anchorRef,
  align = 'start',
  matchWidth = false,
  maxHeight = 288,
  className,
  style,
  children,
  ...props
}: Omit<PopoverProps, 'open'>) {
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<Position | null>(null);

  const place = useCallback(() => {
    const anchor = anchorRef.current;
    const el = ref.current;
    if (!anchor || !el) return;
    const a = anchor.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    const contentH = el.scrollHeight;
    const width = Math.max(el.offsetWidth, matchWidth ? a.width : 0);
    const below = vh - a.bottom - GAP - MARGIN;
    const above = a.top - GAP - MARGIN;
    const flip = contentH > below && above > below;
    const room = Math.max(120, flip ? above : below);
    let left = align === 'end' ? a.right - width : a.left;
    left = Math.min(Math.max(MARGIN, left), Math.max(MARGIN, vw - width - MARGIN));
    setPos({
      left,
      minWidth: matchWidth ? a.width : 0,
      maxHeight: Math.min(maxHeight, room),
      placement: flip ? 'top' : 'bottom',
      ...(flip ? { bottom: vh - a.top + GAP } : { top: a.bottom + GAP }),
    });
  }, [anchorRef, align, matchWidth, maxHeight]);

  useLayoutEffect(() => {
    place();
  }, [place, children]);

  useEffect(() => {
    const onScroll = (e: Event) => {
      // Scrolling inside the popover itself must not move it.
      if (ref.current && e.target instanceof Node && ref.current.contains(e.target)) return;
      place();
    };
    const onDown = (e: PointerEvent) => {
      const t = e.target as Node | null;
      if (!t) return;
      if (ref.current?.contains(t) || anchorRef.current?.contains(t)) return;
      onClose();
    };
    window.addEventListener('resize', place);
    window.addEventListener('scroll', onScroll, true);
    document.addEventListener('pointerdown', onDown, true);
    return () => {
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', onScroll, true);
      document.removeEventListener('pointerdown', onDown, true);
    };
  }, [place, onClose, anchorRef]);

  if (typeof document === 'undefined') return null;

  const css: CSSProperties = pos
    ? { top: pos.top, bottom: pos.bottom, left: pos.left, minWidth: pos.minWidth, maxHeight: pos.maxHeight }
    : { top: 0, left: 0, opacity: 0, pointerEvents: 'none' };

  return createPortal(
    <div
      ref={ref}
      data-placement={pos?.placement ?? 'bottom'}
      style={{ ...css, ...style }}
      className={cn(
        'glass fixed z-[110] overflow-y-auto rounded-lg p-1 text-sm',
        'origin-top animate-scale-in data-[placement=top]:origin-bottom',
        className,
      )}
      {...props}
    >
      {children}
    </div>,
    document.body,
  );
}
