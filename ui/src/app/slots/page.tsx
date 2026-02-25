'use client';

import { useState } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';

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

// API expects date YYYY/MM/DD; input type="date" uses YYYY-MM-DD
function toApiDate(isoDate: string) {
  return isoDate ? isoDate.replace(/-/g, '/') : '';
}

export default function SlotsPage() {
  const [date, setDate] = useState(''); // YYYY-MM-DD for input type="date"
  const [data, setData] = useState<SlotsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [bookingSlotKey, setBookingSlotKey] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [bookingId, setBookingId] = useState<string | null>(null);

  async function loadSlots() {
    if (!date) {
      setError('Please pick a date');
      return;
    }
    setError('');
    setLoading(true);
    setData(null);
    try {
      const apiDate = toApiDate(date);
      const params = new URLSearchParams({ date: apiDate });
      const res = await api.get<SlotsResponse>(
        `${API_ENDPOINTS.slots}?${params.toString()}`
      );
      setData(res);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load slots');
    } finally {
      setLoading(false);
    }
  }

  async function bookSlot(slot: Slot) {
    const key = `${slot.TeeTime}-${slot.TeeBox}`;
    setBookingSlotKey(key);
    setError('');
    try {
      const res = await api.post<{ bookingID: string }>(API_ENDPOINTS.bookingBook, {
        courseID: slot.CourseID,
        txnDate: toApiDate(date),
        session: slot.Session,
        teeBox: slot.TeeBox,
        teeTime: slot.TeeTime,
      });
      setBookingId(res.bookingID);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Booking failed');
    } finally {
      setBookingSlotKey(null);
    }
  }

  const timeDisplay = (t: string) => {
    if (t.length >= 19) return t.slice(11, 16);
    return t;
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link href="/" className="text-blue-600 hover:underline dark:text-blue-400">
          ← Home
        </Link>
      </div>
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
          {loading ? 'Loading…' : 'Load slots'}
        </button>
      </div>
      {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
      {bookingId && (
        <p className="rounded bg-green-100 p-3 text-green-800 dark:bg-green-900/30 dark:text-green-200">
          Booked. Booking ID: {bookingId}
        </p>
      )}
      {data && (
        <div>
          <p className="mb-2 text-sm text-gray-600 dark:text-gray-400">
            Course: {data.course} · {data.slots.length} slot(s)
          </p>
          <ul className="space-y-2">
            {data.slots.map((slot) => (
              <li
                key={`${slot.TeeTime}-${slot.TeeBox}`}
                className="flex flex-wrap items-center justify-between gap-2 rounded border border-gray-200 bg-white p-3 dark:border-gray-700 dark:bg-gray-800"
              >
                <span className="font-medium text-gray-900 dark:text-white">
                  {timeDisplay(slot.TeeTime)} {slot.Session} · TeeBox {slot.TeeBox}
                </span>
                <button
                  type="button"
                  onClick={() => bookSlot(slot)}
                  disabled={bookingSlotKey !== null}
                  className="rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-70 disabled:cursor-not-allowed"
                >
                  {bookingSlotKey === `${slot.TeeTime}-${slot.TeeBox}` ? (
                    <>Booking…</>
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
