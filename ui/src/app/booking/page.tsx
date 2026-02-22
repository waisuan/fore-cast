'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';

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

export default function BookingPage() {
  const [data, setData] = useState<BookingResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    api
      .get<BookingResponse>(API_ENDPOINTS.booking)
      .then((res) => {
        if (!cancelled) setData(res);
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
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link href="/" className="text-blue-600 hover:underline dark:text-blue-400">
          ← Home
        </Link>
      </div>
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        My booking
      </h1>
      {loading && <p className="text-gray-600 dark:text-gray-400">Loading…</p>}
      {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
      {data && !loading && (
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
                    Date: {b.TxnDate} · Course: {b.CourseName} ({b.CourseID})
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    Time: {b.TeeTime} · Session: {b.Session} · TeeBox: {b.TeeBox}
                  </p>
                  <p className="text-sm text-gray-600 dark:text-gray-400">
                    Pax: {b.Pax} · Holes: {b.Hole} · {b.Name}
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
