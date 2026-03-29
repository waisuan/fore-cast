'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { formatDate, formatTime } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';

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
}

export default function BookingPage() {
  const { addToast } = useToast();
  const [presetStatus, setPresetStatus] = useState<PresetStatus | null>(null);
  const [data, setData] = useState<BookingResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [cancelLoading, setCancelLoading] = useState(false);

  const loadPreset = useCallback(async () => {
    try {
      const res = await api.get<PresetStatus & { defaults?: unknown }>(API_ENDPOINTS.preset);
      setPresetStatus({ last_run_status: res.last_run_status ?? 'idle' });
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
        <div className="flex flex-col gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 sm:flex-row sm:items-center sm:justify-between dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300">
          <p>
            Scheduler is running. Slots and bookings require 3rd party access and are unavailable.
            You can cancel the run below.
          </p>
          <button
            type="button"
            onClick={cancelRun}
            disabled={cancelLoading}
            aria-busy={cancelLoading}
            className="shrink-0 rounded border border-blue-600 bg-white px-3 py-1.5 font-medium text-blue-800 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-blue-400 dark:bg-blue-950/50 dark:text-blue-200 dark:hover:bg-blue-900/50"
          >
            {cancelLoading ? <Spinner className="h-4 w-4" /> : 'Cancel run'}
          </button>
        </div>
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
