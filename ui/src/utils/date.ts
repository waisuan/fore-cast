/** Malaysia Time (UTC+8, no DST) — used for all user-visible dates/times in the UI. */
export const MY_TIMEZONE = 'Asia/Kuala_Lumpur';

/**
 * Return today's date in YYYY-MM-DD format (local timezone).
 */
export function todayIso(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
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
