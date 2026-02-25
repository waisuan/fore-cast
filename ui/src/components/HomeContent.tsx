'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { useAuth } from '@/contexts/AuthContext';

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

export default function HomeContent() {
  const { isAuthenticated } = useAuth();
  const [bookings, setBookings] = useState<BookingItem[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isAuthenticated) {
      setBookings(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    api
      .get<BookingResponse>(API_ENDPOINTS.booking)
      .then((res) => {
        if (!cancelled) setBookings(res.Result ?? null);
      })
      .catch((e) => {
        if (!cancelled)
          setError(e instanceof ApiError ? e.message : 'Failed to load');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isAuthenticated]);

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Tee times
      </h1>
      <nav className="flex flex-col gap-3 sm:flex-row sm:gap-4">
        <Link
          href="/slots"
          className="rounded-lg border border-gray-200 bg-white px-4 py-3 text-left font-medium text-gray-900 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:hover:bg-gray-700"
        >
          View slots & book
        </Link>
        <Link
          href="/auto"
          className="rounded-lg border border-gray-200 bg-white px-4 py-3 text-left font-medium text-gray-900 shadow-sm hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-white dark:hover:bg-gray-700"
        >
          Auto-book earliest
        </Link>
      </nav>

      {isAuthenticated && (
        <section>
          <h2 className="mb-3 text-lg font-medium text-gray-900 dark:text-white">
            My bookings
          </h2>
          {loading && (
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Loading…
            </p>
          )}
          {error && (
            <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
          )}
          {!loading && !error && bookings && bookings.length > 0 && (
            <ul className="space-y-3">
              {bookings.map((b) => (
                <li
                  key={b.BookingID}
                  className="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
                >
                  <p className="font-medium text-gray-900 dark:text-white">
                    {b.CourseName} ({b.CourseID})
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    {b.TxnDate} · {b.TeeTime} · Session {b.Session} · TeeBox{' '}
                    {b.TeeBox}
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    Booking ID: {b.BookingID} · {b.Pax} pax · {b.Hole} holes
                  </p>
                </li>
              ))}
            </ul>
          )}
          {!loading && !error && (!bookings || bookings.length === 0) && (
            <p className="text-sm text-gray-600 dark:text-gray-400">
              No bookings yet.
            </p>
          )}
        </section>
      )}
    </div>
  );
}
