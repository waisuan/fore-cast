'use client';

import { useState } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';

export default function AutoPage() {
  const [date, setDate] = useState('');
  const [cutoff, setCutoff] = useState('8:15');
  const [retries, setRetries] = useState(1);
  const [retryIntervalSec, setRetryIntervalSec] = useState(5);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [bookingId, setBookingId] = useState<string | null>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setBookingId(null);
    setLoading(true);
    try {
      const res = await api.post<{ bookingID: string }>(API_ENDPOINTS.bookingAuto, {
        date,
        cutoff,
        retries: retries < 1 ? 1 : retries,
        retry_interval_sec: retryIntervalSec < 1 ? 5 : retryIntervalSec,
      });
      setBookingId(res.bookingID);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Auto-book failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Link href="/" className="text-blue-600 hover:underline dark:text-blue-400">
          ← Home
        </Link>
      </div>
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Auto-book earliest slot
      </h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <label htmlFor="date" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Date (YYYY/MM/DD) *
          </label>
          <input
            id="date"
            type="text"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            placeholder="2026/02/25"
            required
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="cutoff" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Cutoff time (e.g. 8:15)
          </label>
          <input
            id="cutoff"
            type="text"
            value={cutoff}
            onChange={(e) => setCutoff(e.target.value)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="retries" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Max retry rounds
          </label>
          <input
            id="retries"
            type="number"
            min={1}
            value={retries}
            onChange={(e) => setRetries(parseInt(e.target.value, 10) || 1)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="interval" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Seconds between rounds
          </label>
          <input
            id="interval"
            type="number"
            min={1}
            value={retryIntervalSec}
            onChange={(e) => setRetryIntervalSec(parseInt(e.target.value, 10) || 5)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
        {bookingId && (
          <p className="rounded bg-green-100 p-3 text-green-800 dark:bg-green-900/30 dark:text-green-200">
            Booked. Booking ID: {bookingId}
          </p>
        )}
        <button
          type="submit"
          disabled={loading}
          className="w-full max-w-xs rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? 'Booking…' : 'Book earliest slot'}
        </button>
      </form>
    </div>
  );
}
