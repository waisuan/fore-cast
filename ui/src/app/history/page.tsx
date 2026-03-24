'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { formatDate, formatDateTimeShortMY } from '@/utils/date';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';

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
        <>
          {/* Card layout for mobile */}
          <div className="space-y-3 md:hidden">
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
                  {item.tee_time || '-'}
                  {item.tee_box ? ` / Box ${item.tee_box}` : ''}
                </p>
                <p className="mt-1 text-sm text-gray-600 dark:text-gray-300" title={item.message}>
                  {item.booking_id ? `ID: ${item.booking_id}` : item.message}
                </p>
              </div>
            ))}
          </div>
          {/* Table layout for desktop */}
          <div className="hidden overflow-x-auto md:block">
            <table className="w-full text-left text-sm">
              <thead className="border-b border-gray-200 text-xs uppercase text-gray-500 dark:border-gray-700 dark:text-gray-400">
                <tr>
                  <th className="px-3 py-2">Date</th>
                  <th className="px-3 py-2">Target</th>
                  <th className="px-3 py-2">Time</th>
                  <th className="px-3 py-2">Status</th>
                  <th className="px-3 py-2">Message</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
                {data.map((item) => (
                  <tr key={item.id} className="text-gray-900 dark:text-gray-100">
                    <td className="whitespace-nowrap px-3 py-2 text-gray-500 dark:text-gray-400">
                      {formatDateTimeShortMY(item.created_at)}
                    </td>
                    <td className="px-3 py-2">
                      {formatDate(item.txn_date)} &middot; {item.course_id}
                    </td>
                    <td className="px-3 py-2">
                      {item.tee_time || '-'}
                      {item.tee_box ? ` / Box ${item.tee_box}` : ''}
                    </td>
                    <td className="px-3 py-2">
                      <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${statusBadge(item.status)}`}>
                        {item.status}
                      </span>
                    </td>
                    <td className="max-w-xs truncate px-3 py-2 text-gray-500 dark:text-gray-400" title={item.message}>
                      {item.booking_id ? `ID: ${item.booking_id}` : item.message}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}
