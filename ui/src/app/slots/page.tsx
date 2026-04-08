'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { toApiDate, formatTime, todayIsoMalaysia } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';
import SchedulerRunningBanner from '@/components/SchedulerRunningBanner';
import DatePicker from '@/components/DatePicker';

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
  const [course, setCourse] = useState<string>('');
  const [data, setData] = useState<SlotsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [bookingSlotKey, setBookingSlotKey] = useState<string | null>(null);
  const [cancelLoading, setCancelLoading] = useState(false);

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
      if (course) params.set('course', course);
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
  }, [date, course, addToast, presetStatus?.last_run_status]);

  useEffect(() => {
    loadPreset();
  }, [loadPreset]);

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

  const schedulerRunning = presetStatus?.last_run_status === 'running';

  useEffect(() => {
    if (!schedulerRunning) return;
    const id = setInterval(loadPreset, 2000);
    return () => clearInterval(id);
  }, [schedulerRunning, loadPreset]);

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
        <SchedulerRunningBanner cancelLoading={cancelLoading} onCancel={cancelRun} />
      )}
      {!schedulerRunning && (
        <div className="flex flex-col gap-4 sm:flex-row sm:items-stretch sm:gap-4">
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <label htmlFor="date" className="text-sm font-medium text-gray-700 dark:text-gray-300">Date</label>
            <DatePicker
              id="date"
              value={date}
              onChange={setDate}
              min={todayIsoMalaysia()}
            />
          </div>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <label htmlFor="course" className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Course
            </label>
            <select
              id="course"
              value={course}
              onChange={(e) => setCourse(e.target.value)}
              className="min-h-12 w-full min-w-0 rounded border border-gray-300 bg-white px-3 py-2.5 text-base text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            >
              <option value="">Auto (default for day)</option>
              <option value="BRC">BRC</option>
              <option value="PLC">PLC</option>
            </select>
          </div>
          <div className="flex flex-col justify-end sm:shrink-0">
            <button
              type="button"
              onClick={loadSlots}
              disabled={loading}
              aria-busy={loading}
              className="min-h-12 w-full rounded bg-blue-600 px-4 py-2.5 font-medium text-white hover:bg-blue-700 disabled:opacity-50 sm:w-auto sm:min-w-[9.5rem]"
            >
              {loading ? <Spinner className="h-4 w-4 text-white" /> : 'Load slots'}
            </button>
          </div>
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
