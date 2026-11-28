'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { formatDate, formatDateTimeShortMY, formatTime } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';
import RunLog, { type RunStep } from '@/components/RunLog';

interface HistoryItem {
  id: number;
  created_at: string;
  course_id: string;
  txn_date: string;
  tee_time: string;
  tee_box: string;
  booking_id: string;
  status: string;
  message: string;
  steps?: RunStep[] | null;
}

interface HistoryResponse {
  attempts: HistoryItem[];
}

const statusBadge = (status: string) => {
  switch (status) {
    case 'success':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300';
    case 'failed':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300';
    case 'cancelled':
      return 'bg-amber-100 text-amber-900 dark:bg-amber-900/30 dark:text-amber-200';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300';
  }
};

export default function HistoryPage() {
  const { addToast } = useToast();
  const [data, setData] = useState<HistoryItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<HistoryResponse>(API_ENDPOINTS.history);
      setData(res.attempts ?? []);
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to load history', 'error');
    } finally {
      setLoading(false);
    }
  }, [addToast]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Booking history
      </h1>
      {loading && (
        <div className="flex justify-center py-8">
          <Spinner className="h-6 w-6" />
        </div>
      )}
      {!loading && data.length === 0 && (
        <p className="text-gray-600 dark:text-gray-400">No booking history yet.</p>
      )}
      {!loading && data.length > 0 && (
          <div className="space-y-3">
            {data.map((item) => (
              <div
                key={item.id}
                className="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">
                    {formatDate(item.txn_date)} &middot; {item.course_id}
                  </span>
                  <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${statusBadge(item.status)}`}>
                    {item.status}
                  </span>
                </div>
                <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {formatDateTimeShortMY(item.created_at)}
                  {' · '}
                  {formatTime(item.tee_time) || '-'}
                  {item.tee_box ? ` · Tee ${item.tee_box}` : ''}
                </p>
                <p className="mt-1 text-sm text-gray-600 dark:text-gray-300">
                  {item.booking_id ? `ID: ${item.booking_id}` : item.message}
                </p>
                <RunLog steps={item.steps} />
              </div>
            ))}
          </div>
      )}
    </div>
  );
}
