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
  return d.toLocaleDateString(undefined, {
    weekday: 'short',
    year: 'numeric',
    month: 'short',
    day: 'numeric',
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
