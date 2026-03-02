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

export default function BookingPage() {
  const { addToast } = useToast();
  const [data, setData] = useState<BookingResponse | null>(null);
  const [loading, setLoading] = useState(true);

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

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
          My bookings
        </h1>
        <button
          type="button"
          onClick={load}
          disabled={loading}
          aria-busy={loading}
          className="rounded border border-gray-300 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
        >
          Refresh
        </button>
      </div>
      {loading && (
        <div className="flex justify-center py-8">
          <Spinner className="h-6 w-6" />
        </div>
      )}
      {!loading && data && (
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
