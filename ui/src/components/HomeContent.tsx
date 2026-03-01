'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { formatDate, formatTime } from '@/utils/date';
import { useAuth } from '@/contexts/AuthContext';
import Spinner from './Spinner';

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

  const load = useCallback(async () => {
    if (!isAuthenticated) {
      setBookings(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const res = await api.get<BookingResponse>(API_ENDPOINTS.booking);
      setBookings(res.Result ?? null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    load();
  }, [load]);

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
          View slots &amp; book
        </Link>
      </nav>

      {isAuthenticated && (
        <section>
          <h2 className="mb-3 text-lg font-medium text-gray-900 dark:text-white">
            My bookings
          </h2>
          {loading && (
            <div className="py-4">
              <Spinner />
            </div>
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
                    {formatDate(b.TxnDate)} &middot; {formatTime(b.TeeTime)} &middot; Session {b.Session} &middot; TeeBox{' '}
                    {b.TeeBox}
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    Booking ID: {b.BookingID} &middot; {b.Pax} pax &middot; {b.Hole} holes
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
