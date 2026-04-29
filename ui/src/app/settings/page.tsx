'use client';

import { useState, useEffect, useCallback, useMemo } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import {
  addCalendarDaysYmd,
  courseForYmd,
  endOfDayMalaysiaIso,
  formatShortDateMY,
  formatWeekdayDateMY,
  isoToYmdMalaysia,
  nextSchedulerRunMY,
  SCHEDULER_FIRE_LABEL_MY,
  todayIsoMalaysia,
} from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';
import SchedulerRunningBanner from '@/components/SchedulerRunningBanner';
import DatePicker from '@/components/DatePicker';

async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return true;
  }
  // Fallback for older browsers
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  try {
    document.execCommand('copy');
    return true;
  } finally {
    document.body.removeChild(textarea);
  }
}

interface PresetDefaults {
  course: string;
  cutoff: string;
  retry_interval: string;
  min_retry_interval: string;
  timeout: string;
}

interface PresetData {
  user_name: string;
  course: string;
  cutoff: string;
  retry_interval: string;
  timeout: string;
  ntfy_topic: string;
  enable_notifications: boolean;
  enabled: boolean;
  defaults: PresetDefaults;
  last_run_status: string;
  last_run_message: string;
  last_run_at: string | null;
  override_course: string;
  override_until: string | null;
}

type OverrideMode = 'none' | 'once' | 'days7' | 'until';

type OverridePayload = { course: string; until: string | null };

const COURSE_OPTIONS = ['BRC', 'PLC'] as const;

function buildOverridePayload(
  mode: OverrideMode,
  course: string,
  untilYmd: string,
): OverridePayload | 'invalid' {
  if (!course) return { course: '', until: null };
  switch (mode) {
    case 'none':
      return { course: '', until: null };
    case 'once':
      return { course, until: null };
    case 'days7':
      return { course, until: endOfDayMalaysiaIso(addCalendarDaysYmd(todayIsoMalaysia(), 7)) };
    case 'until':
      return untilYmd ? { course, until: endOfDayMalaysiaIso(untilYmd) } : 'invalid';
  }
}

// The default course is always "auto by day-of-week"; the override summary
// references that explicitly so users know what they're reverting to.
const DEFAULT_COURSE_LABEL = "the day's default course";

function summarizeOverride(
  mode: OverrideMode,
  overrideCrs: string,
  untilYmd: string,
): string {
  if (mode === 'none' || !overrideCrs) {
    return `Scheduler will book ${DEFAULT_COURSE_LABEL} (BRC on Sun/Mon/Tue, PLC otherwise).`;
  }
  const back = `, then back to ${DEFAULT_COURSE_LABEL}`;
  if (mode === 'once') {
    return `Scheduler will book ${overrideCrs} on the next run only${back}.`;
  }
  if (mode === 'days7') {
    const expiry = formatShortDateMY(addCalendarDaysYmd(todayIsoMalaysia(), 7));
    return `Scheduler will book ${overrideCrs} for the next 7 days (through ${expiry}, Malaysia time)${back}.`;
  }
  if (mode === 'until' && untilYmd) {
    return `Scheduler will book ${overrideCrs} until end of ${formatShortDateMY(untilYmd)} (Malaysia time)${back}.`;
  }
  return 'Pick an end date for the override.';
}

// Mirrors slotutil.CourseForDate on the backend: BRC on Sun/Mon/Tue, PLC otherwise.
const DEFAULT_COURSE_BY_DAY: ReadonlyArray<{ days: string; course: string }> = [
  { days: 'Sun, Mon, Tue', course: 'BRC' },
  { days: 'Wed, Thu, Fri, Sat', course: 'PLC' },
];

function DefaultCourseSchedule() {
  // Concrete "today → +7" preview so the 1-week-ahead rule is unmissable. Computed
  // from the same scheduler-fire logic used on the homepage so both stay in sync.
  const next = nextSchedulerRunMY();
  const bookingYmd = addCalendarDaysYmd(next.ymd, 7);
  const bookingCourse = courseForYmd(bookingYmd);

  return (
    <section
      aria-label="Default course by booking day"
      className="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-gray-800/60"
    >
      <p className="mb-1 text-sm font-medium text-gray-900 dark:text-white">
        Default course (booking 1 week ahead)
      </p>
      <p className="mb-3 text-xs text-gray-500 dark:text-gray-400">
        The scheduler always books exactly <strong>7 days ahead</strong> in Malaysia time. Unless
        overridden below, the course is picked by the booking date&rsquo;s day of week:
      </p>
      <ul className="space-y-1 text-sm">
        {DEFAULT_COURSE_BY_DAY.map(({ days, course }) => (
          <li
            key={course}
            className="flex items-center gap-2 text-gray-700 dark:text-gray-300"
          >
            <span className="rounded bg-white px-2 py-0.5 font-mono text-xs text-gray-900 ring-1 ring-gray-200 dark:bg-gray-900 dark:text-white dark:ring-gray-700">
              {course}
            </span>
            <span>{days}</span>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-xs text-gray-500 dark:text-gray-400">
        Runs nightly at <strong>{SCHEDULER_FIRE_LABEL_MY}</strong> (Malaysia). Next run targets{' '}
        <strong>{formatWeekdayDateMY(bookingYmd)}</strong> &rarr;{' '}
        <span className="font-mono">{bookingCourse}</span>.
      </p>
    </section>
  );
}

function parseDurationMs(s: string): number | null {
  const match = s.trim().match(/^(\d+(?:\.\d+)?)\s*(ms|s|m|h)$/i);
  if (!match) return null;
  const n = parseFloat(match[1]);
  const unit = match[2].toLowerCase();
  if (unit === 'ms') return n;
  if (unit === 's') return n * 1000;
  if (unit === 'm') return n * 60 * 1000;
  if (unit === 'h') return n * 3600 * 1000;
  return null;
}

export default function SettingsPage() {
  const { addToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [defaults, setDefaults] = useState<PresetDefaults | null>(null);
  const [cutoff, setCutoff] = useState('');
  const [retryIntervalVal, setRetryIntervalVal] = useState('');
  const [timeoutVal, setTimeoutVal] = useState('');
  const [ntfyTopic, setNtfyTopic] = useState('');
  const [enableNotifications, setEnableNotifications] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [copied, setCopied] = useState(false);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [lastRunStatus, setLastRunStatus] = useState<string>('idle');
  const [overrideMode, setOverrideMode] = useState<OverrideMode>('none');
  const [overrideCourse, setOverrideCourse] = useState<string>('');
  const [overrideUntilYmd, setOverrideUntilYmd] = useState<string>('');

  const load = useCallback(async (opts?: { silent?: boolean }) => {
    const silent = opts?.silent ?? false;
    if (!silent) setLoading(true);
    try {
      const res = await api.get<PresetData>(API_ENDPOINTS.preset);
      setDefaults(res.defaults ?? null);
      setCutoff(res.cutoff ?? '');
      setRetryIntervalVal(res.retry_interval ?? res.defaults?.retry_interval ?? '1s');
      setTimeoutVal(res.timeout ?? '');
      setNtfyTopic(res.ntfy_topic ?? '');
      setEnableNotifications(res.enable_notifications ?? false);
      setEnabled(res.enabled ?? false);
      setLastRunStatus(res.last_run_status ?? 'idle');
      const oc = res.override_course ?? '';
      const ou = res.override_until ?? null;
      setOverrideCourse(oc);
      if (!oc) {
        setOverrideMode('none');
        setOverrideUntilYmd('');
      } else if (!ou) {
        setOverrideMode('once');
        setOverrideUntilYmd('');
      } else {
        setOverrideMode('until');
        setOverrideUntilYmd(isoToYmdMalaysia(ou));
      }
    } catch (e) {
      if (!silent) {
        addToast(e instanceof ApiError ? e.message : 'Failed to load settings', 'error');
      }
    } finally {
      if (!silent) setLoading(false);
    }
  }, [addToast]);

  useEffect(() => {
    load();
  }, [load]);

  const schedulerRunning = lastRunStatus === 'running';

  const cancelRun = useCallback(async () => {
    setCancelLoading(true);
    try {
      await api.post(API_ENDPOINTS.presetCancel);
      addToast('Cancelling run…', 'info');
      await load({ silent: true });
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to cancel', 'error');
    } finally {
      setCancelLoading(false);
    }
  }, [addToast, load]);

  useEffect(() => {
    if (!schedulerRunning) return;
    const id = setInterval(() => {
      void load({ silent: true });
    }, 2000);
    return () => clearInterval(id);
  }, [schedulerRunning, load]);

  const MIN_RETRY_MS = 0;

  const overrideSummary = useMemo(
    () => summarizeOverride(overrideMode, overrideCourse, overrideUntilYmd),
    [overrideMode, overrideCourse, overrideUntilYmd],
  );
  // Recompute every render so the date picker's `min` stays correct if a session
  // crosses Malaysia midnight. Cost is negligible — both helpers are O(1).
  const minOverrideDate = addCalendarDaysYmd(todayIsoMalaysia(), 1);

  async function handleCopyTopic() {
    if (!ntfyTopic) return;
    try {
      const ok = await copyToClipboard(ntfyTopic);
      if (ok) {
        setCopied(true);
        addToast('Topic copied to clipboard', 'success');
        setTimeout(() => setCopied(false), 2000);
      } else {
        addToast('Failed to copy', 'error');
      }
    } catch {
      addToast('Failed to copy', 'error');
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (schedulerRunning) return;
    setSaving(true);
    try {
      let retryInterval = retryIntervalVal.trim();
      const retryMs = parseDurationMs(retryInterval);
      if (retryMs !== null && retryMs < MIN_RETRY_MS) {
        retryInterval = '0s';
        addToast('Retry interval must be at least 0s, adjusted to 0s', 'info');
      } else if (retryMs === null && retryInterval !== '') {
        addToast('Invalid retry interval format (e.g. 1s, 100ms)', 'error');
        setSaving(false);
        return;
      }
      const timeout = timeoutVal.trim();
      if (timeout !== '' && parseDurationMs(timeout) === null) {
        addToast('Invalid timeout format (e.g. 10m, 1h, 30s)', 'error');
        setSaving(false);
        return;
      }
      const overridePayload = buildOverridePayload(overrideMode, overrideCourse, overrideUntilYmd);
      if (overridePayload === 'invalid') {
        addToast('Pick a date for the override', 'error');
        setSaving(false);
        return;
      }
      await api.put(API_ENDPOINTS.preset, {
        course: '',
        cutoff,
        retry_interval: retryInterval || undefined,
        timeout: timeoutVal,
        enable_notifications: enableNotifications,
        enabled,
        override_course: overridePayload.course,
        override_until: overridePayload.until,
      });
      addToast('Settings saved', 'success');
      await load();
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to save settings', 'error');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Auto-booker settings
      </h1>
      {schedulerRunning && (
        <SchedulerRunningBanner cancelLoading={cancelLoading} onCancel={cancelRun} />
      )}
      <p className="text-sm text-gray-600 dark:text-gray-400">
        Configure your nightly auto-booking preset.
      </p>
      <DefaultCourseSchedule />
      {schedulerRunning && (
        <p className="text-sm text-amber-800 dark:text-amber-200/90">
          Settings are read-only while the scheduler is running. Use <strong>Cancel run</strong> above
          to stop, or wait until the run finishes.
        </p>
      )}
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <fieldset
          disabled={saving || schedulerRunning}
          className="flex flex-col gap-4"
          aria-busy={saving}
        >
          <legend className="sr-only">Auto-booker configuration</legend>
        <div className="rounded-lg border-2 border-amber-300 bg-amber-50 p-4 dark:border-amber-700/70 dark:bg-amber-900/20">
          <p className="mb-1 text-sm font-semibold text-amber-900 dark:text-amber-100">
            Temporary course override
          </p>
          <p className="mb-3 text-xs text-amber-800/80 dark:text-amber-200/70">
            Book a different course than the default schedule above, then revert automatically.
          </p>
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-1">
              <label
                htmlFor="overrideCourse"
                className="text-xs text-amber-900 dark:text-amber-200"
              >
                Use this course instead
              </label>
              <select
                id="overrideCourse"
                value={overrideCourse}
                onChange={(e) => {
                  const next = e.target.value;
                  setOverrideCourse(next);
                  if (!next) {
                    setOverrideMode('none');
                  } else if (overrideMode === 'none') {
                    setOverrideMode('once');
                  }
                }}
                className="w-full max-w-xs rounded border border-amber-300 bg-white px-3 py-2 text-gray-900 dark:border-amber-700/70 dark:bg-gray-800 dark:text-white"
              >
                <option value="">No override</option>
                {COURSE_OPTIONS.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </div>
            {overrideCourse && (
              <fieldset className="flex flex-col gap-2">
                <legend className="text-xs text-amber-900 dark:text-amber-200">How long</legend>
                <label className="flex items-center gap-2 text-sm text-amber-900 dark:text-amber-100">
                  <input
                    type="radio"
                    name="overrideMode"
                    value="once"
                    checked={overrideMode === 'once'}
                    onChange={() => setOverrideMode('once')}
                  />
                  Next run only
                </label>
                <label className="flex items-center gap-2 text-sm text-amber-900 dark:text-amber-100">
                  <input
                    type="radio"
                    name="overrideMode"
                    value="days7"
                    checked={overrideMode === 'days7'}
                    onChange={() => setOverrideMode('days7')}
                  />
                  Next 7 days
                </label>
                <label className="flex items-center gap-2 text-sm text-amber-900 dark:text-amber-100">
                  <input
                    type="radio"
                    name="overrideMode"
                    value="until"
                    checked={overrideMode === 'until'}
                    onChange={() => {
                      setOverrideMode('until');
                      if (!overrideUntilYmd) {
                        setOverrideUntilYmd(addCalendarDaysYmd(todayIsoMalaysia(), 7));
                      }
                    }}
                  />
                  Until a specific date
                </label>
                {overrideMode === 'until' && (
                  <div className="w-full max-w-xs">
                    <DatePicker
                      aria-label="Override expiry date"
                      value={overrideUntilYmd}
                      min={minOverrideDate}
                      onChange={setOverrideUntilYmd}
                    />
                  </div>
                )}
              </fieldset>
            )}
            <p className="text-xs text-amber-800/80 dark:text-amber-200/70">{overrideSummary}</p>
          </div>
        </div>
        <div>
          <label htmlFor="cutoff" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Cutoff time
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Only book slots before this time. Default: {defaults?.cutoff}
          </p>
          <input
            id="cutoff"
            type="time"
            value={cutoff}
            onChange={(e) => setCutoff(e.target.value)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="retryInterval" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Delay between passes
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            After each full walk through cutoff slots, wait this long before the next pass (e.g. 1s, 100ms). Min: {defaults?.min_retry_interval ?? '0s'}. Default: {defaults?.retry_interval}
          </p>
          <input
            id="retryInterval"
            type="text"
            value={retryIntervalVal}
            onChange={(e) => setRetryIntervalVal(e.target.value)}
            placeholder={defaults?.retry_interval}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="timeout" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Timeout
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Maximum time to keep repeating full passes before giving up. Default: {defaults?.timeout}
          </p>
          <input
            id="timeout"
            type="text"
            value={timeoutVal}
            onChange={(e) => setTimeoutVal(e.target.value)}
            placeholder={defaults?.timeout}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <input
              id="enableNotifications"
              type="checkbox"
              checked={enableNotifications}
              onChange={(e) => setEnableNotifications(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <label htmlFor="enableNotifications" className="text-sm text-gray-700 dark:text-gray-300">
              Enable push notifications
            </label>
          </div>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Receive notifications on booking success or failure via{' '}
            <a
              href="https://ntfy.sh"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 underline dark:text-blue-400"
            >
              ntfy.sh
            </a>
            . Download the{' '}
            <a
              href="https://ntfy.sh/#subscribe-phone"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 underline dark:text-blue-400"
            >
              ntfy app
            </a>
            {' '}on your device and subscribe to your topic below.
          </p>
          {enableNotifications && ntfyTopic && (
            <div className="mt-2 rounded border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-600 dark:bg-gray-700">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <p className="text-xs text-gray-500 dark:text-gray-400">Your topic:</p>
                  <p className="font-mono text-sm text-gray-900 dark:text-white break-all">{ntfyTopic}</p>
                </div>
                <button
                  type="button"
                  onClick={handleCopyTopic}
                  aria-label="Copy topic to clipboard"
                  className="shrink-0 rounded border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 min-h-[44px] dark:border-gray-500 dark:bg-gray-600 dark:text-gray-200 dark:hover:bg-gray-500"
                >
                  {copied ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>
          )}
          {enableNotifications && !ntfyTopic && (
            <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              A unique topic will be generated when you save.
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          <input
            id="enabled"
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="h-4 w-4 rounded border-gray-300"
          />
          <label htmlFor="enabled" className="text-sm text-gray-700 dark:text-gray-300">
            Enable auto-booking
          </label>
        </div>
        </fieldset>
        <button
          type="submit"
          disabled={saving || schedulerRunning}
          aria-busy={saving}
          className="w-full max-w-xs rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? <Spinner className="h-4 w-4 text-white" /> : 'Save settings'}
        </button>
      </form>
    </div>
  );
}
