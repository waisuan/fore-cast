'use client';

import { useState, useEffect, useCallback, useMemo } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import {
  addCalendarDaysYmd,
  courseForYmd,
  formatDateTimeMY,
  formatWeekdayDateMY,
  nextSchedulerRunMY,
  SCHEDULER_FIRE_HOUR_MY,
  SCHEDULER_FIRE_LABEL_MY,
  SCHEDULER_FIRE_MINUTE_MY,
} from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import SchedulerRunningBanner from './SchedulerRunningBanner';
import CourseOverrideBanner from './CourseOverrideBanner';

interface PresetStatus {
  enabled: boolean;
  last_run_status: string;
  last_run_message: string;
  last_run_at: string | null;
  override_course: string;
  override_until: string | null;
}

export default function HomeContent() {
  const { addToast } = useToast();
  const [status, setStatus] = useState<PresetStatus | null>(null);
  const [dismissedId, setDismissedId] = useState<string | null>(null);
  const [cancelLoading, setCancelLoading] = useState(false);

  const load = useCallback(
    async (opts?: { silent?: boolean }) => {
      const silent = opts?.silent ?? false;
      try {
        const res = await api.get<PresetStatus & { defaults?: unknown }>(API_ENDPOINTS.preset);
        setStatus({
          enabled: res.enabled ?? false,
          last_run_status: res.last_run_status ?? 'idle',
          last_run_message: res.last_run_message ?? '',
          last_run_at: res.last_run_at ?? null,
          override_course: res.override_course ?? '',
          override_until: res.override_until ?? null,
        });
      } catch (e) {
        setStatus(null);
        if (!silent) {
          addToast(e instanceof ApiError ? e.message : 'Failed to load status', 'error');
        }
      }
    },
    [addToast],
  );

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
    load();
  }, [load]);

  const {
    enabled,
    last_run_status,
    last_run_message,
    last_run_at,
    override_course,
    override_until,
  } = status ?? {};
  const isRecent = last_run_at
    ? Date.now() - new Date(last_run_at).getTime() < 23 * 60 * 60 * 1000
    : false;
  const bannerId =
    last_run_status === 'running'
      ? 'running'
      : last_run_at && last_run_status
        ? `${last_run_at}-${last_run_status}`
        : null;
  const showBanner =
    enabled &&
    last_run_status &&
    last_run_status !== 'idle' &&
    last_run_status !== 'running' &&
    isRecent &&
    bannerId !== dismissedId;

  const schedulerRunning = last_run_status === 'running';

  // Preview the next scheduled job: which day will be booked and on which
  // course (override wins if it would still be active when the scheduler fires).
  const upcoming = useMemo(() => {
    if (!enabled || schedulerRunning) return null;
    const next = nextSchedulerRunMY();
    const bookingYmd = addCalendarDaysYmd(next.ymd, 7);
    const fireHH = String(SCHEDULER_FIRE_HOUR_MY).padStart(2, '0');
    const fireMM = String(SCHEDULER_FIRE_MINUTE_MY).padStart(2, '0');
    const fireInstant = new Date(`${next.ymd}T${fireHH}:${fireMM}:00+08:00`).getTime();
    const overrideAppliesToNextRun =
      !!override_course && (!override_until || new Date(override_until).getTime() > fireInstant);
    return {
      bookingLabel: formatWeekdayDateMY(bookingYmd),
      course: overrideAppliesToNextRun && override_course ? override_course : courseForYmd(bookingYmd),
      whenLabel: next.tonight ? 'tonight' : 'tomorrow night',
      isOverride: overrideAppliesToNextRun,
    };
  }, [enabled, schedulerRunning, override_course, override_until]);

  useEffect(() => {
    if (!schedulerRunning) return;
    const id = setInterval(() => {
      void load({ silent: true });
    }, 2000);
    return () => clearInterval(id);
  }, [schedulerRunning, load]);

  return (
    <div className="space-y-8">
      <p className="text-sm text-gray-600 dark:text-gray-400">
        Having trouble loading slots or your bookings? Try logging out (menu above) and signing in again to refresh your session.
      </p>
      {schedulerRunning && (
        <SchedulerRunningBanner cancelLoading={cancelLoading} onCancel={cancelRun} />
      )}
      {upcoming && (
        <div className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-100">
          <p>
            Next auto-booking: <strong className="font-semibold">{upcoming.bookingLabel}</strong>{' '}
            on <strong className="font-semibold">{upcoming.course}</strong>
            {upcoming.isOverride && ' (override)'}.
          </p>
          <p className="mt-1 text-xs text-blue-800/80 dark:text-blue-200/70">
            Scheduler runs {upcoming.whenLabel} at {SCHEDULER_FIRE_LABEL_MY} (Malaysia).
          </p>
        </div>
      )}
      {enabled && override_course && (
        <CourseOverrideBanner
          overrideCourse={override_course}
          overrideUntil={override_until ?? null}
        />
      )}
      {showBanner && (
        <div
          className={`relative rounded-lg border px-4 py-3 pr-10 text-sm ${
            last_run_status === 'success'
              ? 'border-green-200 bg-green-50 text-green-800 dark:border-green-800 dark:bg-green-900/30 dark:text-green-300'
              : 'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-900/30 dark:text-red-300'
          }`}
        >
          <button
            type="button"
            onClick={() => bannerId && setDismissedId(bannerId)}
            className="absolute right-2 top-2 rounded p-1 opacity-70 hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-offset-1"
            aria-label="Dismiss"
          >
            <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          <div className="flex items-center gap-2">
            <span className="font-medium">
              {last_run_status === 'success'
                ? 'Last run: booked successfully'
                : 'Last run: failed'}
            </span>
          </div>
          {last_run_message && (
            <p className="mt-1 text-xs opacity-80">{last_run_message}</p>
          )}
          {last_run_at && !isNaN(new Date(last_run_at).getTime()) && (
            <p className="mt-1 text-xs opacity-60">
              {formatDateTimeMY(last_run_at)}
            </p>
          )}
        </div>
      )}
      <nav className="flex flex-col gap-3 sm:flex-row sm:gap-4">
        <Link
          href="/slots"
          className="rounded-lg border border-gray-200 bg-white px-4 py-3 text-left font-medium text-gray-900 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:hover:bg-gray-700"
        >
          View slots &amp; book
        </Link>
        <Link
          href="/booking"
          className="rounded-lg border border-gray-200 bg-white px-4 py-3 text-left font-medium text-gray-900 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:hover:bg-gray-700"
        >
          My bookings
        </Link>
      </nav>
    </div>
  );
}
