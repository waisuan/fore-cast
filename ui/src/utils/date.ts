/** Malaysia Time (UTC+8, no DST) — used for all user-visible dates/times in the UI. */
export const MY_TIMEZONE = 'Asia/Kuala_Lumpur';

/**
 * Local time at which the scheduler fires daily (Malaysia). Mirrors the cron
 * defined in `railway/scheduler.toml` (21:55 = 9:55 PM MY).
 */
export const SCHEDULER_FIRE_HOUR_MY = 21;
export const SCHEDULER_FIRE_MINUTE_MY = 55;

/** Human-readable scheduler fire time, e.g. "9:55 PM". Derived from the constants
 * above so the homepage and settings labels stay in sync with the cron config. */
export const SCHEDULER_FIRE_LABEL_MY = (() => {
  const h12 = SCHEDULER_FIRE_HOUR_MY % 12 || 12;
  const ampm = SCHEDULER_FIRE_HOUR_MY >= 12 ? 'PM' : 'AM';
  return `${h12}:${String(SCHEDULER_FIRE_MINUTE_MY).padStart(2, '0')} ${ampm}`;
})();

// Module-scope formatters: `Intl.DateTimeFormat` construction is non-trivial and
// these are hit on every render of components that compute upcoming-booking
// previews, so we share one instance per shape.
const MY_YMD_PARTS_FMT = new Intl.DateTimeFormat('en-US', {
  timeZone: MY_TIMEZONE,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
});
const MY_YMDT_PARTS_FMT = new Intl.DateTimeFormat('en-US', {
  timeZone: MY_TIMEZONE,
  hour12: false,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

function ymdFromParts(parts: Intl.DateTimeFormatPart[]): string | null {
  const get = (t: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === t)?.value;
  const y = get('year');
  const m = get('month');
  const d = get('day');
  if (!y || !m || !d) return null;
  return `${y}-${m.padStart(2, '0')}-${d.padStart(2, '0')}`;
}

/**
 * Return today's date in YYYY-MM-DD format (browser local timezone).
 */
export function todayIso(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/**
 * Today's calendar date as YYYY-MM-DD in {@link MY_TIMEZONE} (Malaysia).
 * Use for slot booking windows and other rules tied to club local date, not the viewer's TZ.
 */
export function todayIsoMalaysia(): string {
  return ymdFromParts(MY_YMD_PARTS_FMT.formatToParts(new Date())) ?? todayIso();
}

/**
 * Add calendar days to a YYYY-MM-DD string (proleptic Gregorian).
 * Matches Playwright and the slot `min` attribute when both use Malaysia calendar strings.
 */
export function addCalendarDaysYmd(ymd: string, deltaDays: number): string {
  const seg = ymd.split('-').map(Number);
  if (seg.length !== 3 || seg.some((n) => Number.isNaN(n))) {
    return ymd;
  }
  const [y, m, d] = seg;
  const dt = new Date(Date.UTC(y, m - 1, d + deltaDays));
  const y2 = dt.getUTCFullYear();
  const m2 = dt.getUTCMonth() + 1;
  const d2 = dt.getUTCDate();
  return `${y2}-${String(m2).padStart(2, '0')}-${String(d2).padStart(2, '0')}`;
}

/**
 * Convert HTML date input (YYYY-MM-DD) to API format (YYYY/MM/DD).
 */
export function toApiDate(isoDate: string): string {
  return isoDate ? isoDate.replace(/-/g, '/') : '';
}

/**
 * Format an API date or ISO datetime for display.
 * Handles "YYYY/MM/DD", "YYYY-MM-DD", and "1899-12-30T07:00:00" style strings.
 */
export function formatDate(raw: string): string {
  if (!raw) return '';
  const normalized = raw.replace(/\//g, '-');
  const d = new Date(normalized);
  if (isNaN(d.getTime())) return raw;
  return d.toLocaleDateString('en-MY', {
    timeZone: MY_TIMEZONE,
    weekday: 'short',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

/**
 * Full timestamp in Malaysia time (e.g. last scheduler run).
 */
export function formatDateTimeMY(iso: string | Date): string {
  const d = typeof iso === 'string' ? new Date(iso) : iso;
  if (isNaN(d.getTime())) return typeof iso === 'string' ? iso : '';
  return d.toLocaleString('en-MY', {
    timeZone: MY_TIMEZONE,
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

/**
 * Short timestamp in Malaysia time (e.g. history list).
 */
export function formatDateTimeShortMY(iso: string | Date): string {
  const d = typeof iso === 'string' ? new Date(iso) : iso;
  if (isNaN(d.getTime())) return '';
  return d.toLocaleString('en-MY', {
    timeZone: MY_TIMEZONE,
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Extract time (HH:MM) from a datetime string like "1899-12-30T07:00:00".
 * Falls back to returning the original string if no time portion found.
 */
export function formatTime(raw: string): string {
  if (!raw) return '';
  if (raw.length >= 16 && raw[10] === 'T') return raw.slice(11, 16);
  return raw;
}

/**
 * Convert a YYYY-MM-DD calendar date (interpreted in {@link MY_TIMEZONE}) to an
 * RFC3339 timestamp pinned to the end of that day (23:59:59 +08:00). Use for
 * temporary-override expiry so the override stays active through the local
 * Malaysia evening, not just the picker's midnight.
 */
export function endOfDayMalaysiaIso(ymd: string): string {
  const seg = ymd.split('-').map(Number);
  if (seg.length !== 3 || seg.some((n) => Number.isNaN(n))) return '';
  const [y, m, d] = seg;
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${y}-${pad(m)}-${pad(d)}T23:59:59+08:00`;
}

/**
 * Format a calendar date (YYYY-MM-DD) for short display, e.g. "5 Apr".
 */
export function formatShortDateMY(ymd: string): string {
  if (!ymd) return '';
  const seg = ymd.split('-').map(Number);
  if (seg.length !== 3 || seg.some((n) => Number.isNaN(n))) return ymd;
  const [y, m, d] = seg;
  const dt = new Date(Date.UTC(y, m - 1, d));
  if (isNaN(dt.getTime())) return ymd;
  return dt.toLocaleDateString('en-MY', {
    timeZone: MY_TIMEZONE,
    day: 'numeric',
    month: 'short',
  });
}

/**
 * Returns the Malaysia calendar day on which the scheduler will next fire,
 * along with whether that fire is happening tonight or tomorrow night relative
 * to `now`. The booking target date is this day + 7.
 */
export function nextSchedulerRunMY(now: Date = new Date()): {
  ymd: string;
  tonight: boolean;
} {
  const parts = MY_YMDT_PARTS_FMT.formatToParts(now);
  const get = (t: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === t)?.value ?? '00';
  const today = `${get('year')}-${get('month').padStart(2, '0')}-${get('day').padStart(2, '0')}`;
  const minutes = Number(get('hour')) * 60 + Number(get('minute'));
  const fireMins = SCHEDULER_FIRE_HOUR_MY * 60 + SCHEDULER_FIRE_MINUTE_MY;
  if (minutes < fireMins) {
    return { ymd: today, tonight: true };
  }
  return { ymd: addCalendarDaysYmd(today, 1), tonight: false };
}

/**
 * BRC on Sun/Mon/Tue, PLC otherwise. Mirrors `slotutil.CourseForDate` on the
 * Go backend so the homepage can preview the next-run course without a server
 * roundtrip.
 */
export function courseForYmd(ymd: string): 'BRC' | 'PLC' {
  const seg = ymd.split('-').map(Number);
  if (seg.length !== 3 || seg.some((n) => Number.isNaN(n))) return 'PLC';
  const [y, m, d] = seg;
  const wd = new Date(Date.UTC(y, m - 1, d)).getUTCDay();
  return wd === 0 || wd === 1 || wd === 2 ? 'BRC' : 'PLC';
}

/**
 * Format a YYYY-MM-DD date with weekday and month abbreviation (no year),
 * e.g. "Mon, 6 May". Suitable for inline labels.
 */
export function formatWeekdayDateMY(ymd: string): string {
  if (!ymd) return '';
  const seg = ymd.split('-').map(Number);
  if (seg.length !== 3 || seg.some((n) => Number.isNaN(n))) return ymd;
  const [y, m, d] = seg;
  // Noon UTC keeps the calendar day stable when formatted in MY.
  const dt = new Date(Date.UTC(y, m - 1, d, 12));
  if (isNaN(dt.getTime())) return ymd;
  return dt.toLocaleDateString('en-MY', {
    timeZone: MY_TIMEZONE,
    weekday: 'short',
    day: 'numeric',
    month: 'short',
  });
}

/**
 * Extract the YYYY-MM-DD calendar date (in {@link MY_TIMEZONE}) from a full
 * RFC3339/ISO timestamp. Returns empty string if the input does not parse.
 */
export function isoToYmdMalaysia(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  return ymdFromParts(MY_YMD_PARTS_FMT.formatToParts(d)) ?? '';
}
