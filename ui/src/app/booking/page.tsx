'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { formatDate, formatTime } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';
import SchedulerRunningBanner from '@/components/SchedulerRunningBanner';
import CourseOverrideBanner from '@/components/CourseOverrideBanner';

interface BookingItem {
  BookingID: string;
  TxnDate: string;
  CourseID: string;
  CourseName: string;
  TeeTime: string;
  Session: string;
  TeeBox: string;
  Pax: number;
  Hole: number;
  Name: string;
}

interface BookingResponse {
  Status: boolean;
  Reason?: string;
  Result?: BookingItem[];
}

interface PresetStatus {
  last_run_status: string;
  override_course: string;
  override_until: string | null;
}

export default function BookingPage() {
  const { addToast } = useToast();
  const [presetStatus, setPresetStatus] = useState<PresetStatus | null>(null);
  const [data, setData] = useState<BookingResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [cancelLoading, setCancelLoading] = useState(false);
  const [deleteBookingId, setDeleteBookingId] = useState<string | null>(null);

  const loadPreset = useCallback(async () => {
    try {
      const res = await api.get<PresetStatus & { defaults?: unknown }>(API_ENDPOINTS.preset);
      setPresetStatus({
        last_run_status: res.last_run_status ?? 'idle',
        override_course: res.override_course ?? '',
        override_until: res.override_until ?? null,
      });
    } catch {
      setPresetStatus(null);
    }
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<BookingResponse>(API_ENDPOINTS.booking);
      setData(res);
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to load', 'error');
    } finally {
      setLoading(false);
    }
  }, [addToast]);

  const cancelRun = useCallback(async () => {
    setCancelLoading(true);
    try {
      await api.post(API_ENDPOINTS.presetCancel);
      addToast('Cancelling run…', 'info');
      await loadPreset();
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to cancel', 'error');
    } finally {
      setCancelLoading(false);
    }
  }, [addToast, loadPreset]);

  const cancelBooking = useCallback(
    async (b: BookingItem) => {
      const when = `${formatDate(b.TxnDate)} · ${formatTime(b.TeeTime)} · ${b.CourseName}`;
      if (
        !window.confirm(
          `Cancel booking ${b.BookingID}?\n\n${when}\n\nThis cannot be undone.`,
        )
      ) {
        return;
      }
      setDeleteBookingId(b.BookingID);
      try {
        await api.post(API_ENDPOINTS.bookingCancel, { bookingID: b.BookingID });
        addToast('Booking cancelled', 'success');
        await load();
      } catch (e) {
        addToast(e instanceof ApiError ? e.message : 'Failed to cancel booking', 'error');
      } finally {
        setDeleteBookingId(null);
      }
    },
    [addToast, load],
  );

  useEffect(() => {
    loadPreset();
  }, [loadPreset]);

  useEffect(() => {
    if (presetStatus?.last_run_status === 'running') {
      setLoading(false);
      return;
    }
    if (presetStatus !== null) {
      load();
    }
  }, [presetStatus, load]);

  const schedulerRunning = presetStatus?.last_run_status === 'running';

  useEffect(() => {
    if (!schedulerRunning) return;
    const id = setInterval(loadPreset, 2000);
    return () => clearInterval(id);
  }, [schedulerRunning, loadPreset]);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        My bookings
      </h1>
      {schedulerRunning && (
        <SchedulerRunningBanner cancelLoading={cancelLoading} onCancel={cancelRun} />
      )}
      {!schedulerRunning && presetStatus?.override_course && (
        <CourseOverrideBanner
          overrideCourse={presetStatus.override_course}
          overrideUntil={presetStatus.override_until}
        />
      )}
      {!schedulerRunning && loading && (
        <div className="flex justify-center py-8">
          <Spinner className="h-6 w-6" />
        </div>
      )}
      {!schedulerRunning && !loading && data && (
        <>
          {!data.Status && data.Reason && (
            <p className="text-gray-600 dark:text-gray-400">{data.Reason}</p>
          )}
          {data.Result && data.Result.length > 0 ? (
            <ul className="space-y-4">
              {data.Result.map((b) => (
                <li
                  key={b.BookingID}
                  className="rounded border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
                >
                  <p className="font-medium text-gray-900 dark:text-white">
                    Booking ID: {b.BookingID}
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    {formatDate(b.TxnDate)} &middot; {b.CourseName} ({b.CourseID})
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    {formatTime(b.TeeTime)} &middot; Session: {b.Session} &middot; TeeBox: {b.TeeBox}
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    Pax: {b.Pax} &middot; Holes: {b.Hole} &middot; {b.Name}
                  </p>
                  <div className="mt-3">
                    <button
                      type="button"
                      onClick={() => void cancelBooking(b)}
                      disabled={deleteBookingId !== null}
                      className="rounded border border-red-200 bg-white px-3 py-1.5 text-sm text-red-700 hover:bg-red-50 disabled:opacity-50 dark:border-red-900 dark:bg-gray-800 dark:text-red-400 dark:hover:bg-red-950/40"
                    >
                      {deleteBookingId === b.BookingID ? 'Cancelling…' : 'Cancel booking'}
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-gray-600 dark:text-gray-400">No bookings found.</p>
          )}
        </>
      )}
    </div>
  );
}
