'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { formatDateTimeMY } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import SchedulerRunningBanner from './SchedulerRunningBanner';

interface PresetStatus {
  enabled: boolean;
  last_run_status: string;
  last_run_message: string;
  last_run_at: string | null;
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

  const { enabled, last_run_status, last_run_message, last_run_at } = status ?? {};
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
      {enabled && (
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Scheduler runs daily at 9:55–10:00 PM (Malaysia).
        </p>
      )}
    </div>
  );
}
