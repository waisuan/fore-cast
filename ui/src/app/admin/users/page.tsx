'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { adminUserRolePath } from '@/config/api';

type UserRow = {
  user_name: string;
  role: 'ADMIN' | 'NON_ADMIN';
  created_at: string;
};

export default function AdminUsersPage() {
  const [users, setUsers] = useState<UserRow[]>([]);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError('');
    setLoading(true);
    try {
      const data = await api.get<{ users: UserRow[] }>(API_ENDPOINTS.adminUsers);
      setUsers(data.users);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to load users');
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function setRole(userName: string, role: 'ADMIN' | 'NON_ADMIN') {
    setError('');
    setBusy(userName + role);
    try {
      await api.put(adminUserRolePath(userName), { role });
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Update failed');
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-gray-50 px-4 py-8 dark:bg-gray-900">
      <div className="mx-auto w-full max-w-2xl">
        <h1 className="mb-2 text-xl font-semibold text-gray-900 dark:text-white">Users</h1>
        <p className="mb-6 text-sm text-gray-600 dark:text-gray-400">
          Club member accounts. Promote a user to ADMIN so they can manage registrations and
          removals.
        </p>
        {error && (
          <p role="alert" className="mb-4 text-sm text-red-600 dark:text-red-400">
            {error}
          </p>
        )}
        {loading ? (
          <p className="text-sm text-gray-500">Loading…</p>
        ) : (
          <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-gray-200 dark:border-gray-600">
                  <th className="px-4 py-3 font-medium text-gray-700 dark:text-gray-300">
                    Member ID
                  </th>
                  <th className="px-4 py-3 font-medium text-gray-700 dark:text-gray-300">Role</th>
                  <th className="px-4 py-3 font-medium text-gray-700 dark:text-gray-300">
                    Added
                  </th>
                  <th className="px-4 py-3 font-medium text-gray-700 dark:text-gray-300">Actions</th>
                </tr>
              </thead>
              <tbody>
                {users.map((u) => (
                  <tr
                    key={u.user_name}
                    className="border-b border-gray-100 last:border-0 dark:border-gray-700"
                  >
                    <td className="px-4 py-3 font-mono text-gray-900 dark:text-white">
                      {u.user_name}
                    </td>
                    <td className="px-4 py-3 text-gray-800 dark:text-gray-200">{u.role}</td>
                    <td className="px-4 py-3 text-gray-600 dark:text-gray-400">
                      {new Date(u.created_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-3">
                      {u.role === 'NON_ADMIN' ? (
                        <button
                          type="button"
                          disabled={busy !== null}
                          onClick={() => setRole(u.user_name, 'ADMIN')}
                          className="rounded bg-blue-600 px-2 py-1 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                        >
                          {busy === u.user_name + 'ADMIN' ? '…' : 'Make admin'}
                        </button>
                      ) : (
                        <button
                          type="button"
                          disabled={busy !== null}
                          onClick={() => {
                            if (
                              window.confirm(
                                `Remove ADMIN from ${u.user_name}? They will lose access to admin tools.`,
                              )
                            ) {
                              void setRole(u.user_name, 'NON_ADMIN');
                            }
                          }}
                          className="rounded border border-gray-300 px-2 py-1 text-xs text-gray-800 hover:bg-gray-50 dark:border-gray-600 dark:text-gray-200 dark:hover:bg-gray-700"
                        >
                          {busy === u.user_name + 'NON_ADMIN' ? '…' : 'Remove admin'}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <p className="mt-6 space-y-2 text-center text-sm">
          <Link href="/admin/register" className="block text-blue-600 hover:underline dark:text-blue-400">
            Register a user
          </Link>
          <Link href="/admin/delete" className="block text-blue-600 hover:underline dark:text-blue-400">
            Remove user or preset
          </Link>
          <Link href="/" className="block text-blue-600 hover:underline dark:text-blue-400">
            ← Back to app
          </Link>
        </p>
      </div>
    </div>
  );
}
