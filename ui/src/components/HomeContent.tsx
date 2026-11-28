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
  otherCourse,
  SCHEDULER_FIRE_HOUR_MY,
  SCHEDULER_FIRE_LABEL_MY,
  SCHEDULER_FIRE_MINUTE_MY,
  weekdayCodeForYmd,
} from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import SchedulerRunningBanner from './SchedulerRunningBanner';
import CourseOverrideBanner from './CourseOverrideBanner';
import Spinner from './Spinner';
import RunLog, { type RunStep } from './RunLog';

const LAST_RUN_STYLE: Record<string, { label: string; box: string }> = {
  success: {
    label: 'Last run booked a tee time',
    box: 'border-green-300 bg-green-50 text-green-900 dark:border-green-800 dark:bg-green-900/30 dark:text-green-200',
  },
  cancelled: {
    label: 'Last run was cancelled',
    box: 'border-amber-300 bg-amber-50 text-amber-900 dark:border-amber-800 dark:bg-amber-900/30 dark:text-amber-200',
  },
  failed: {
    label: 'Last run booked nothing',
    box: 'border-red-300 bg-red-50 text-red-900 dark:border-red-800 dark:bg-red-900/30 dark:text-red-200',
  },
};

function LastRunCard({
  status,
  message,
  at,
  steps,
}: {
  status: string;
  message: string;
  at: string;
  steps: RunStep[];
}) {
  const style = LAST_RUN_STYLE[status] ?? LAST_RUN_STYLE.failed;
  return (
    <section aria-label="Last auto-booking run" className={`rounded-lg border px-4 py-3 text-sm ${style.box}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <h2 className="text-base font-semibold">{style.label}</h2>
        <time dateTime={at} className="text-xs">
          {formatDateTimeMY(at)}
        </time>
      </div>
      {message && <p className="mt-1 break-words">{message}</p>}
      {steps.length > 0 && (
        <div className="mt-2 rounded bg-white/70 px-3 pb-2 pt-1 dark:bg-gray-900/40">
          <RunLog steps={steps} />
        </div>
      )}
      <Link href="/history" className="mt-2 inline-block font-medium underline underline-offset-2">
        All runs in History
      </Link>
    </section>
  );
}

function AltCourseSubline({ course, className }: { course: string; className: string }) {
  return (
    <p className={className}>
      Also trying <strong className="font-semibold">{course}</strong> in parallel.
    </p>
  );
}

interface PresetStatus {
  enabled: boolean;
  last_run_status: string;
  last_run_message: string;
  last_run_at: string | null;
  override_course: string;
  override_until: string | null;
  skip_next_run: boolean;
  alt_course_days: string[];
  live_steps: RunStep[];
}

export default function HomeContent() {
  const { addToast } = useToast();
  const [status, setStatus] = useState<PresetStatus | null>(null);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [skipBusy, setSkipBusy] = useState(false);

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
          skip_next_run: res.skip_next_run ?? false,
          alt_course_days: Array.isArray(res.alt_course_days) ? res.alt_course_days : [],
          live_steps: Array.isArray(res.live_steps) ? res.live_steps : [],
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

  const toggleSkipNextRun = useCallback(
    async (queued: boolean) => {
      setSkipBusy(true);
      try {
        if (queued) {
          await api.post(API_ENDPOINTS.presetSkipNext);
          addToast('Next run will be skipped.', 'success');
        } else {
          await api.delete(API_ENDPOINTS.presetSkipNext);
          addToast('Skip cancelled — next run is back on.', 'info');
        }
        await load({ silent: true });
      } catch (e) {
        addToast(
          e instanceof ApiError
            ? e.message
            : queued
              ? 'Failed to skip next run'
              : 'Failed to undo skip',
          'error',
        );
      } finally {
        setSkipBusy(false);
      }
    },
    [addToast, load],
  );

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
    skip_next_run,
    alt_course_days = [],
  } = status ?? {};
  const lastRunAtValid = !!last_run_at && !isNaN(new Date(last_run_at).getTime());
  // Always show the last result: "did tonight's run work?" is the main reason to open the app.
  const showLastRun =
    !!status &&
    lastRunAtValid &&
    !!last_run_status &&
    last_run_status !== 'idle' &&
    last_run_status !== 'running';

  const schedulerRunning = last_run_status === 'running';

  // Next job preview: default course vs override; if the next fire is skipped, shift dates by one day.
  const upcoming = useMemo(() => {
    if (!enabled || schedulerRunning) return null;
    const next = nextSchedulerRunMY();
    const skipped = !!skip_next_run;
    const fireYmd = skipped ? addCalendarDaysYmd(next.ymd, 1) : next.ymd;
    const bookingYmd = addCalendarDaysYmd(fireYmd, 7);
    const fireHH = String(SCHEDULER_FIRE_HOUR_MY).padStart(2, '0');
    const fireMM = String(SCHEDULER_FIRE_MINUTE_MY).padStart(2, '0');
    const fireInstant = new Date(`${fireYmd}T${fireHH}:${fireMM}:00+08:00`).getTime();
    // Inclusive boundary mirrors Go's `!now.After(until)` in
    // `preset.ResolveOverride` — the override is still active when `until`
    // lands exactly on the fire instant.
    const overrideAppliesToNextRun =
      !!override_course && (!override_until || new Date(override_until).getTime() >= fireInstant);
    const primaryCourse = courseForYmd(bookingYmd);
    // Alt course preview mirrors `resolveCoursesForRun` on the backend: never
    // when an override is active for the next fire, otherwise only when the
    // booking date's weekday is opted in.
    const bookingWeekday = weekdayCodeForYmd(bookingYmd);
    const altCourse =
      !overrideAppliesToNextRun && bookingWeekday && alt_course_days.includes(bookingWeekday)
        ? otherCourse(primaryCourse)
        : null;
    return {
      bookingLabel: formatWeekdayDateMY(bookingYmd),
      fireLabel: formatWeekdayDateMY(fireYmd),
      course: overrideAppliesToNextRun && override_course ? override_course : primaryCourse,
      altCourse,
      whenLabel: next.tonight ? 'tonight' : 'tomorrow night',
      isOverride: overrideAppliesToNextRun,
      skipped,
    };
  }, [
    enabled,
    schedulerRunning,
    override_course,
    override_until,
    skip_next_run,
    alt_course_days,
  ]);

  useEffect(() => {
    if (!schedulerRunning) return;
    const id = setInterval(() => {
      void load({ silent: true });
    }, 2000);
    return () => clearInterval(id);
  }, [schedulerRunning, load]);

  return (
    <div className="space-y-4">
      {schedulerRunning && (
        <section aria-label="Run in progress" className="space-y-2">
          <SchedulerRunningBanner cancelLoading={cancelLoading} onCancel={cancelRun} />
          <div className="rounded-lg border border-gray-200 bg-white px-4 py-3 dark:border-gray-700 dark:bg-gray-800">
            <h2 className="text-sm font-semibold text-gray-900 dark:text-white">Live run log</h2>
            <RunLog steps={status?.live_steps} live />
          </div>
        </section>
      )}
      {showLastRun && (
        <LastRunCard
          status={last_run_status!}
          message={last_run_message ?? ''}
          at={last_run_at!}
          steps={status?.live_steps ?? []}
        />
      )}
      {status && !enabled && !schedulerRunning && (
        <div className="flex flex-col gap-3 rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm text-gray-700 sm:flex-row sm:items-center sm:justify-between dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300">
          <p>
            <strong className="font-semibold text-gray-900 dark:text-white">Auto-booking is off.</strong>{' '}
            Turn it on to book a tee time 7 days ahead every night at {SCHEDULER_FIRE_LABEL_MY}.
          </p>
          <Link
            href="/settings"
            className="inline-flex min-h-11 shrink-0 items-center justify-center rounded bg-blue-600 px-4 font-medium text-white hover:bg-blue-700"
          >
            Set up auto-booking
          </Link>
        </div>
      )}
      {upcoming && (
        <div
          className={`flex flex-col gap-3 rounded-lg border px-4 py-3 text-sm sm:flex-row sm:items-center sm:justify-between ${
            upcoming.skipped
              ? 'border-gray-300 bg-gray-50 text-gray-700 dark:border-gray-700 dark:bg-gray-800/60 dark:text-gray-300'
              : 'border-blue-200 bg-blue-50 text-blue-900 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-100'
          }`}
        >
          {upcoming.skipped ? (
            <>
              <div>
                <p>
                  The upcoming auto-booking run is{' '}
                  <strong className="font-semibold">skipped</strong>.
                </p>
                <p className="mt-1 text-xs opacity-80">
                  Auto-booker resumes <strong className="font-semibold">{upcoming.fireLabel}</strong>{' '}
                  at {SCHEDULER_FIRE_LABEL_MY} (Malaysia) and will book{' '}
                  <strong className="font-semibold">{upcoming.bookingLabel}</strong> on{' '}
                  <strong className="font-semibold">{upcoming.course}</strong>
                  {upcoming.isOverride && ' (override)'}.
                </p>
                {upcoming.altCourse && (
                  <AltCourseSubline
                    course={upcoming.altCourse}
                    className="mt-1 text-xs opacity-80"
                  />
                )}
              </div>
              <button
                type="button"
                onClick={() => void toggleSkipNextRun(false)}
                disabled={skipBusy}
                aria-busy={skipBusy}
                className="shrink-0 rounded border border-gray-500 bg-white px-3 py-1.5 font-medium text-gray-800 hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-400 dark:bg-gray-950/50 dark:text-gray-200 dark:hover:bg-gray-900/50"
              >
                {skipBusy ? <Spinner className="h-4 w-4" /> : 'Undo skip'}
              </button>
            </>
          ) : (
            <>
              <div>
                <p>
                  Next auto-booking: <strong className="font-semibold">{upcoming.bookingLabel}</strong>{' '}
                  on <strong className="font-semibold">{upcoming.course}</strong>
                  {upcoming.isOverride && ' (override)'}.
                </p>
                <p className="mt-1 text-xs text-blue-800/80 dark:text-blue-200/70">
                  Scheduler runs {upcoming.whenLabel} at {SCHEDULER_FIRE_LABEL_MY} (Malaysia).
                </p>
                {upcoming.altCourse && (
                  <AltCourseSubline
                    course={upcoming.altCourse}
                    className="mt-1 text-xs text-blue-800/80 dark:text-blue-200/70"
                  />
                )}
              </div>
              <button
                type="button"
                onClick={() => void toggleSkipNextRun(true)}
                disabled={skipBusy}
                aria-busy={skipBusy}
                className="shrink-0 rounded border border-blue-600 bg-white px-3 py-1.5 font-medium text-blue-800 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-blue-400 dark:bg-blue-950/50 dark:text-blue-200 dark:hover:bg-blue-900/50"
              >
                {skipBusy ? <Spinner className="h-4 w-4" /> : 'Skip next run'}
              </button>
            </>
          )}
        </div>
      )}
      {enabled && override_course && (
        <CourseOverrideBanner
          overrideCourse={override_course}
          overrideUntil={override_until ?? null}
        />
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
      <p className="text-xs text-gray-500 dark:text-gray-400">
        Slots or bookings not loading? Log out and sign in again to refresh your club session.
      </p>
    </div>
  );
}
