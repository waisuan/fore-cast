import { describe, expect, it } from 'vitest';
import {
  addCalendarDaysYmd,
  courseForYmd,
  endOfDayMalaysiaIso,
  formatDate,
  formatDateTimeMY,
  formatDateTimeShortMY,
  formatShortDateMY,
  formatTime,
  formatWeekdayDateMY,
  isoToYmdMalaysia,
  MY_TIMEZONE,
  nextSchedulerRunMY,
  todayIso,
  todayIsoMalaysia,
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

describe('todayIsoMalaysia', () => {
  it('returns a YYYY-MM-DD string', () => {
    expect(todayIsoMalaysia()).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});

describe('endOfDayMalaysiaIso', () => {
  it('pins to 23:59:59 +08:00 in Malaysia time', () => {
    expect(endOfDayMalaysiaIso('2026-04-29')).toBe('2026-04-29T23:59:59+08:00');
  });

  it('returns empty string for invalid input', () => {
    expect(endOfDayMalaysiaIso('not-a-date')).toBe('');
    expect(endOfDayMalaysiaIso('')).toBe('');
  });
});

describe('formatShortDateMY', () => {
  it('returns a short date like "29 Apr"', () => {
    const out = formatShortDateMY('2026-04-29');
    expect(out).toMatch(/29/);
    expect(out).toMatch(/Apr/);
  });

  it('returns original string for invalid input', () => {
    expect(formatShortDateMY('garbage')).toBe('garbage');
  });
});

describe('isoToYmdMalaysia', () => {
  it('extracts the calendar day in Malaysia time', () => {
    expect(isoToYmdMalaysia('2026-04-29T23:59:59+08:00')).toBe('2026-04-29');
  });

  it('rolls forward when UTC instant lands the next day in MY time', () => {
    expect(isoToYmdMalaysia('2026-04-29T20:00:00Z')).toBe('2026-04-30');
  });

  it('returns empty string for invalid input', () => {
    expect(isoToYmdMalaysia('not-an-iso')).toBe('');
    expect(isoToYmdMalaysia('')).toBe('');
  });
});

describe('courseForYmd', () => {
  // 2026-04-26 is a Sunday; matches slotutil.CourseForDate.
  it.each([
    ['2026-04-26', 'BRC'], // Sun
    ['2026-04-27', 'BRC'], // Mon
    ['2026-04-28', 'BRC'], // Tue
    ['2026-04-29', 'PLC'], // Wed
    ['2026-04-30', 'PLC'], // Thu
    ['2026-05-01', 'PLC'], // Fri
    ['2026-05-02', 'PLC'], // Sat
  ])('returns %s for %s', (input, expected) => {
    expect(courseForYmd(input)).toBe(expected);
  });

  it('falls back to PLC for invalid input', () => {
    expect(courseForYmd('not-a-date')).toBe('PLC');
  });
});

describe('formatWeekdayDateMY', () => {
  it('formats with weekday + day + short month', () => {
    const out = formatWeekdayDateMY('2026-05-06');
    expect(out).toMatch(/Wed/);
    expect(out).toMatch(/6/);
    expect(out).toMatch(/May/);
  });

  it('returns empty string for empty input', () => {
    expect(formatWeekdayDateMY('')).toBe('');
  });
});

describe('nextSchedulerRunMY', () => {
  it('targets today when now is before 21:55 MY', () => {
    // 2026-04-29 12:00 UTC = 2026-04-29 20:00 MY (before 21:55)
    const now = new Date('2026-04-29T12:00:00Z');
    const out = nextSchedulerRunMY(now);
    expect(out.tonight).toBe(true);
    expect(out.ymd).toBe('2026-04-29');
  });

  it('targets tomorrow when now is at/after 21:55 MY', () => {
    // 2026-04-29 14:00 UTC = 2026-04-29 22:00 MY (past 21:55)
    const now = new Date('2026-04-29T14:00:00Z');
    const out = nextSchedulerRunMY(now);
    expect(out.tonight).toBe(false);
    expect(out.ymd).toBe('2026-04-30');
  });
});

describe('addCalendarDaysYmd', () => {
  it('adds one day within a month', () => {
    expect(addCalendarDaysYmd('2030-06-01', 1)).toBe('2030-06-02');
  });

  it('rolls month and year', () => {
    expect(addCalendarDaysYmd('2030-01-31', 1)).toBe('2030-02-01');
    expect(addCalendarDaysYmd('2024-02-28', 1)).toBe('2024-02-29');
  });

  it('returns original string for invalid input', () => {
    expect(addCalendarDaysYmd('not-a-date', 1)).toBe('not-a-date');
  });
});
