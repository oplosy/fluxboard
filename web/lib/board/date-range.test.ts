import { describe, expect, it } from 'vitest';

import { formatMinutes, presetRange, rangeDays } from './date-range';

// Friday 23 Oct 2026, late evening UTC.
const now = new Date('2026-10-23T22:30:00Z');

describe('presetRange', () => {
  it('runs this week Monday to Sunday', () => {
    expect(presetRange('this-week', now)).toEqual({ from: '2026-10-19', to: '2026-10-25' });
  });

  it('treats Sunday as the end of the week, not the start', () => {
    expect(presetRange('this-week', new Date('2026-10-25T08:00:00Z'))).toEqual({ from: '2026-10-19', to: '2026-10-25' });
  });

  it('covers 14 days from today and the whole month', () => {
    expect(presetRange('next-2-weeks', now)).toEqual({ from: '2026-10-23', to: '2026-11-05' });
    expect(presetRange('this-month', now)).toEqual({ from: '2026-10-01', to: '2026-10-31' });
  });
});

describe('helpers', () => {
  it('counts both ends of a range', () => {
    expect(rangeDays({ from: '2026-10-19', to: '2026-10-25' })).toBe(7);
  });

  it('formats logged minutes', () => {
    expect(formatMinutes(0)).toBe('—');
    expect(formatMinutes(45)).toBe('45m');
    expect(formatMinutes(120)).toBe('2h');
    expect(formatMinutes(135)).toBe('2h 15m');
  });
});
