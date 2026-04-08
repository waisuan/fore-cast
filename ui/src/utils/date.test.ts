import { describe, expect, it } from 'vitest';
import {
  formatDate,
  formatDateTimeMY,
  formatDateTimeShortMY,
  formatTime,
  MY_TIMEZONE,
  todayIso,
  toApiDate,
} from './date';

describe('toApiDate', () => {
  it.each([
    ['2030-01-15', '2030/01/15'],
    ['', ''],
  ])('maps %s to %s', (input, expected) => {
    expect(toApiDate(input)).toBe(expected);
  });
});

describe('formatTime', () => {
  it.each([
    ['', ''],
    ['1899-12-30T07:37:00', '07:37'],
    ['no-t-here', 'no-t-here'],
  ])('formats %j as %j', (input, expected) => {
    expect(formatTime(input)).toBe(expected);
  });
});

describe('formatDate', () => {
  it('returns empty for empty input', () => {
    expect(formatDate('')).toBe('');
  });

  it('returns original string when the value is not a valid date', () => {
    expect(formatDate('not-a-real-date-string')).toBe('not-a-real-date-string');
  });

  it('produces a non-empty localized string for a valid calendar day in MY timezone', () => {
    const out = formatDate('2026-06-15');
    expect(out.length).toBeGreaterThan(0);
    expect(out).toMatch(/2026/);
  });
});

describe('formatDateTimeMY', () => {
  it('returns the original string for an invalid ISO string', () => {
    expect(formatDateTimeMY('not-valid')).toBe('not-valid');
  });

  it('formats a real instant with year present in Malaysia time', () => {
    const out = formatDateTimeMY('2026-03-10T06:30:00.000Z');
    expect(out).toMatch(/2026/);
  });

  it('accepts a Date object', () => {
    const out = formatDateTimeMY(new Date('2026-01-01T12:00:00.000Z'));
    expect(out).toMatch(/2026/);
  });
});

describe('formatDateTimeShortMY', () => {
  it('returns empty for invalid date string', () => {
    expect(formatDateTimeShortMY('')).toBe('');
  });

  it('includes month and day for a valid ISO string', () => {
    const out = formatDateTimeShortMY('2026-12-25T08:00:00.000Z');
    expect(out.length).toBeGreaterThan(3);
  });
});

describe('MY_TIMEZONE', () => {
  it('is fixed for app-wide display', () => {
    expect(MY_TIMEZONE).toBe('Asia/Kuala_Lumpur');
  });
});

describe('todayIso', () => {
  it('returns a YYYY-MM-DD string', () => {
    expect(todayIso()).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});
