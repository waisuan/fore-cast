'use client';

import { useState } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { toApiDate } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';

export default function AutoPage() {
  const { addToast } = useToast();
  const [date, setDate] = useState('');
  const [cutoff, setCutoff] = useState('08:15');
  const [retries, setRetries] = useState(1);
  const [retryIntervalSec, setRetryIntervalSec] = useState(1);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    try {
      const res = await api.post<{ bookingID: string }>(API_ENDPOINTS.bookingAuto, {
        date: toApiDate(date),
        cutoff,
        retries: retries < 1 ? 1 : retries,
        retry_interval_sec: retryIntervalSec < 1 ? 5 : retryIntervalSec,
      });
      addToast(`Booked! ID: ${res.bookingID}`, 'success');
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Auto-book failed', 'error');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Auto-book earliest slot
      </h1>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <label htmlFor="date" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Date *
          </label>
          <input
            id="date"
            type="date"
            value={date}
            onChange={(e) => setDate(e.target.value)}
            required
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="cutoff" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Cutoff time
          </label>
          <input
            id="cutoff"
            type="time"
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
            onChange={(e) => setRetryIntervalSec(parseInt(e.target.value, 10) || 1)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <button
          type="submit"
          disabled={loading}
          className="w-full max-w-xs rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {loading ? <Spinner className="h-4 w-4 text-white" /> : 'Book earliest slot'}
        </button>
      </form>
    </div>
  );
}
