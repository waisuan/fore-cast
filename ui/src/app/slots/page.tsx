'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { toApiDate, formatTime, todayIso } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';

interface Slot {
  TeeTime: string;
  Session: string;
  TeeBox: string;
  CourseID: string;
  CourseName?: string;
}

interface SlotsResponse {
  course: string;
  slots: Slot[];
}

interface PresetStatus {
  last_run_status: string;
}

export default function SlotsPage() {
  const { addToast } = useToast();
  const [presetStatus, setPresetStatus] = useState<PresetStatus | null>(null);
  const [date, setDate] = useState('');
  const [data, setData] = useState<SlotsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [bookingSlotKey, setBookingSlotKey] = useState<string | null>(null);

  const loadPreset = useCallback(async () => {
    try {
      const res = await api.get<PresetStatus & { defaults?: unknown }>(API_ENDPOINTS.preset);
      setPresetStatus({ last_run_status: res.last_run_status ?? 'idle' });
    } catch {
      setPresetStatus(null);
    }
  }, []);

  const loadSlots = useCallback(async () => {
    if (presetStatus?.last_run_status === 'running') return;
    if (!date) {
      addToast('Please pick a date', 'error');
      return;
    }
    setLoading(true);
    setData(null);
    try {
      const params = new URLSearchParams({ date: toApiDate(date) });
      const res = await api.get<SlotsResponse>(
        `${API_ENDPOINTS.slots}?${params.toString()}`,
      );
      setData(res);
      if (res.slots.length === 0) {
        addToast('No slots available for this date', 'info');
      }
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to load slots', 'error');
    } finally {
      setLoading(false);
    }
  }, [date, addToast, presetStatus?.last_run_status]);

  useEffect(() => {
    loadPreset();
  }, [loadPreset]);

  const schedulerRunning = presetStatus?.last_run_status === 'running';

  const bookSlot = useCallback(
    async (slot: Slot) => {
      if (schedulerRunning) return;
      const key = `${slot.TeeTime}-${slot.TeeBox}`;
      setBookingSlotKey(key);
      try {
        const res = await api.post<{ bookingID?: string }>(API_ENDPOINTS.bookingBook, {
          courseID: slot.CourseID,
          txnDate: toApiDate(date),
          session: slot.Session,
          teeBox: slot.TeeBox,
          teeTime: slot.TeeTime,
        });
        if (res?.bookingID) {
          addToast(`Booked! ID: ${res.bookingID}`, 'success');
        } else {
          addToast('Booking failed — no confirmation received', 'error');
        }
      } catch (e) {
        addToast(e instanceof ApiError ? e.message : 'Booking failed', 'error');
      } finally {
        setBookingSlotKey(null);
      }
    },
    [date, addToast, schedulerRunning]
  );

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Available slots
      </h1>
      {schedulerRunning && (
        <div className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300">
          Scheduler is running. Slots and booking require 3rd party access and are unavailable. Check back later.
        </div>
      )}
      {!schedulerRunning && (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          <div>
            <label htmlFor="date" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
              Date
            </label>
            <input
              id="date"
              type="date"
              value={date}
              onChange={(e) => setDate(e.target.value)}
              min={todayIso()}
              className="w-full rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white sm:w-40"
            />
          </div>
          <button
            type="button"
            onClick={loadSlots}
            disabled={loading}
            aria-busy={loading}
            className="rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
          >
            {loading ? <Spinner className="h-4 w-4 text-white" /> : 'Load slots'}
          </button>
          {data && (
            <button
              type="button"
              onClick={loadSlots}
              disabled={loading}
              className="rounded border border-gray-300 px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
            >
              Refresh
            </button>
          )}
        </div>
      )}
      {!schedulerRunning && data && (
        <div>
          <p className="mb-2 text-sm text-gray-600 dark:text-gray-400">
            Course: {data.course} &middot; {data.slots.length} slot(s)
          </p>
          <ul className="space-y-2">
            {data.slots.map((slot) => (
              <li
                key={`${slot.TeeTime}-${slot.TeeBox}`}
                className="flex flex-wrap items-center justify-between gap-2 rounded border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800"
              >
                <span className="font-medium text-gray-900 dark:text-white">
                  {formatTime(slot.TeeTime)} {slot.Session} &middot; TeeBox {slot.TeeBox}
                </span>
                <button
                  type="button"
                  onClick={() => bookSlot(slot)}
                  disabled={bookingSlotKey !== null}
                  aria-busy={bookingSlotKey === `${slot.TeeTime}-${slot.TeeBox}`}
                  className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {bookingSlotKey === `${slot.TeeTime}-${slot.TeeBox}` ? (
                    <Spinner className="h-4 w-4 text-white" />
                  ) : (
                    'Book'
                  )}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
