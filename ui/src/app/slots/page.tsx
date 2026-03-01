'use client';

import { useState } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { toApiDate, formatTime } from '@/utils/date';
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

export default function SlotsPage() {
  const { addToast } = useToast();
  const [date, setDate] = useState('');
  const [data, setData] = useState<SlotsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [bookingSlotKey, setBookingSlotKey] = useState<string | null>(null);

  async function loadSlots() {
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
  }

  async function bookSlot(slot: Slot) {
    const key = `${slot.TeeTime}-${slot.TeeBox}`;
    setBookingSlotKey(key);
    try {
      const res = await api.post<{ bookingID: string }>(API_ENDPOINTS.bookingBook, {
        courseID: slot.CourseID,
        txnDate: toApiDate(date),
        session: slot.Session,
        teeBox: slot.TeeBox,
        teeTime: slot.TeeTime,
      });
      addToast(`Booked! ID: ${res.bookingID}`, 'success');
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Booking failed', 'error');
    } finally {
      setBookingSlotKey(null);
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Available slots
      </h1>
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
            className="w-full rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white sm:w-40"
          />
        </div>
        <button
          type="button"
          onClick={loadSlots}
          disabled={loading}
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
      {data && (
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
