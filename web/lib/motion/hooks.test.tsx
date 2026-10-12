import { StrictMode, type ReactNode } from 'react';
import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { usePresence } from './hooks';

const strict = ({ children }: { children: ReactNode }) => <StrictMode>{children}</StrictMode>;

describe('usePresence', () => {
  beforeEach(() => vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'requestAnimationFrame', 'cancelAnimationFrame'] }));
  afterEach(() => vi.useRealTimers());

  it('mounts on open, opens after two frames, unmounts after the exit', () => {
    const { result, rerender } = renderHook(({ open }) => usePresence(open, 200), {
      initialProps: { open: false },
      wrapper: strict,
    });
    // Let the initial (closed) exit timer run out, as on a page that has been idle.
    act(() => vi.advanceTimersByTime(500));
    expect(result.current).toEqual({ mounted: false, state: 'closed' });

    rerender({ open: true });
    expect(result.current).toEqual({ mounted: true, state: 'closed' });
    act(() => vi.advanceTimersToNextFrame());
    act(() => vi.advanceTimersToNextFrame());
    expect(result.current).toEqual({ mounted: true, state: 'open' });

    rerender({ open: false });
    expect(result.current).toEqual({ mounted: true, state: 'closed' });
    act(() => vi.advanceTimersByTime(199));
    expect(result.current.mounted).toBe(true);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toEqual({ mounted: false, state: 'closed' });
  });

  it('stays mounted when reopened during the exit', () => {
    const { result, rerender } = renderHook(({ open }) => usePresence(open, 200), {
      initialProps: { open: true },
      wrapper: strict,
    });
    expect(result.current).toEqual({ mounted: true, state: 'open' });

    rerender({ open: false });
    act(() => vi.advanceTimersByTime(100));
    rerender({ open: true });
    act(() => vi.advanceTimersByTime(500));
    expect(result.current).toEqual({ mounted: true, state: 'open' });
  });
});
