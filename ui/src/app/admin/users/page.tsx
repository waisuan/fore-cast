'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import {
  adminDeletePresetPath,
  adminDeleteUserPath,
  adminUserRolePath,
} from '@/config/api';
import { useAuth } from '@/contexts/AuthContext';

type UserRow = {
  user_name: string;
  role: 'ADMIN' | 'NON_ADMIN';
  created_at: string;
};

export default function AdminUsersPage() {
  const { user: me, logout } = useAuth();
  const [users, setUsers] = useState<UserRow[]>([]);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);

  const load = useCallback(async () => {
    setError('');
    setNotice('');
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
    setNotice('');
    setBusy(`role:${userName}:${role}`);
    try {
      await api.put(adminUserRolePath(userName), { role });
      await load();
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Update failed');
    } finally {
      setBusy(null);
    }
  }

  async function removePreset(userName: string) {
    setError('');
    setNotice('');
    if (
      !window.confirm(
        `Remove the saved preset for "${userName}"? Their login stays; they can set up a new preset later.`,
      )
    ) {
      return;
    }
    setBusy(`preset:${userName}`);
    try {
      await api.delete(adminDeletePresetPath(userName));
      setNotice(`Preset removed for ${userName}.`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to remove preset');
    } finally {
      setBusy(null);
    }
  }

  async function deleteUserAccount(userName: string) {
    setError('');
    setNotice('');
    if (
      !window.confirm(
        `Permanently delete ${userName} (credentials, preset, sessions, booking history)? This cannot be undone.`,
      )
    ) {
      return;
    }
    if (userName === me?.username) {
      if (
        !window.confirm(
          'You are deleting your own account. You will be signed out. Continue?',
        )
      ) {
        return;
      }
    }
    setBusy(`delete:${userName}`);
    try {
      await api.delete(adminDeleteUserPath(userName));
      if (userName === me?.username) {
        await logout();
        window.location.href = '/';
        return;
      }
      await load();
      setNotice(`User ${userName} was removed.`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Failed to delete user');
    } finally {
      setBusy(null);
    }
  }

  return (
    <div>
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
        {notice && (
          <p className="mb-4 text-sm text-green-700 dark:text-green-400" role="status">
            {notice}
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
                      <div className="flex max-w-xs flex-col gap-2">
                        <div className="flex flex-wrap gap-1.5">
                          {u.role === 'NON_ADMIN' ? (
                            <button
                              type="button"
                              disabled={busy !== null}
                              onClick={() => setRole(u.user_name, 'ADMIN')}
                              className="rounded bg-blue-600 px-2 py-1 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                            >
                              {busy === `role:${u.user_name}:ADMIN` ? '…' : 'Make admin'}
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
                              {busy === `role:${u.user_name}:NON_ADMIN` ? '…' : 'Remove admin'}
                            </button>
                          )}
                        </div>
                        <div className="flex flex-wrap gap-1.5 border-t border-gray-100 pt-2 dark:border-gray-700">
                          <button
                            type="button"
                            disabled={busy !== null}
                            onClick={() => void removePreset(u.user_name)}
                            className="rounded border border-amber-600/70 bg-amber-50 px-2 py-1 text-xs font-medium text-amber-950 hover:bg-amber-100 disabled:opacity-50 dark:border-amber-600 dark:bg-amber-950/40 dark:text-amber-100 dark:hover:bg-amber-900/50"
                          >
                            {busy === `preset:${u.user_name}` ? '…' : 'Remove preset'}
                          </button>
                          <button
                            type="button"
                            disabled={busy !== null}
                            onClick={() => void deleteUserAccount(u.user_name)}
                            className="rounded bg-red-700 px-2 py-1 text-xs font-medium text-white hover:bg-red-800 disabled:opacity-50"
                          >
                            {busy === `delete:${u.user_name}` ? '…' : 'Delete user'}
                          </button>
                        </div>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <p className="mt-6 text-center text-sm">
          <Link href="/admin/register" className="text-blue-600 hover:underline dark:text-blue-400">
            Register a user
          </Link>
        </p>
    </div>
  );
}
