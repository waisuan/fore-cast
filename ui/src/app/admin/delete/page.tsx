'use client';

import { useState } from 'react';
import Link from 'next/link';
import { api, ApiError } from '@/utils/api';
import { adminDeletePresetPath, adminDeleteUserPath } from '@/config/api';

export default function AdminDeletePage() {
  const [username, setUsername] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [submitting, setSubmitting] = useState<'preset' | 'user' | null>(null);

  async function deletePreset() {
    setError('');
    setSuccess('');
    if (!username.trim()) {
      setError('Target username is required');
      return;
    }
    if (
      !window.confirm(
        `Remove the saved preset for "${username.trim()}"? Their login will stay; they can set up a new preset after signing in.`,
      )
    ) {
      return;
    }
    setSubmitting('preset');
    try {
      await api.delete(adminDeletePresetPath(username.trim()));
      setSuccess(`Preset removed for ${username.trim()}.`);
      setUsername('');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Request failed');
    } finally {
      setSubmitting(null);
    }
  }

  async function deleteUser() {
    setError('');
    setSuccess('');
    if (!username.trim()) {
      setError('Target username is required');
      return;
    }
    if (
      !window.confirm(
        `Permanently delete user "${username.trim()}" (credentials, preset, sessions, and booking history)? This cannot be undone.`,
      )
    ) {
      return;
    }
    setSubmitting('user');
    try {
      await api.delete(adminDeleteUserPath(username.trim()));
      setSuccess(`User ${username.trim()} was fully removed.`);
      setUsername('');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Request failed');
    } finally {
      setSubmitting(null);
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-4 dark:bg-gray-900">
      <div className="w-full max-w-sm rounded-lg border border-gray-200 bg-white p-6 shadow dark:border-gray-700 dark:bg-gray-800">
        <h1 className="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
          Admin: Remove user or preset
        </h1>
        <p className="mb-6 text-sm text-gray-600 dark:text-gray-400">
          Choose whether to drop only the saved preset or remove the account entirely.
        </p>
        <div className="space-y-4">
          <div>
            <label
              htmlFor="target-user"
              className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
            >
              Target club member ID
            </label>
            <input
              id="target-user"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="e.g. A1234-0"
              className="w-full rounded border border-gray-300 px-3 py-2 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              autoComplete="off"
            />
          </div>
          {error && (
            <p role="alert" className="text-sm text-red-600 dark:text-red-400">
              {error}
            </p>
          )}
          {success && (
            <p className="text-sm text-green-600 dark:text-green-400">{success}</p>
          )}
          <div className="flex flex-col gap-3">
            <button
              type="button"
              onClick={deletePreset}
              disabled={submitting !== null}
              aria-busy={submitting === 'preset'}
              className="w-full rounded border border-amber-600 bg-amber-50 px-4 py-2 font-medium text-amber-900 hover:bg-amber-100 focus:outline-none focus:ring-2 focus:ring-amber-500 disabled:opacity-50 dark:border-amber-500 dark:bg-amber-950/40 dark:text-amber-100 dark:hover:bg-amber-900/50"
            >
              {submitting === 'preset' ? 'Removing…' : 'Remove preset only'}
            </button>
            <button
              type="button"
              onClick={deleteUser}
              disabled={submitting !== null}
              aria-busy={submitting === 'user'}
              className="w-full rounded bg-red-700 px-4 py-2 font-medium text-white hover:bg-red-800 focus:outline-none focus:ring-2 focus:ring-red-500 disabled:opacity-50"
            >
              {submitting === 'user' ? 'Deleting…' : 'Delete user completely'}
            </button>
          </div>
        </div>
        <p className="mt-6 space-y-2 text-center text-sm">
          <Link
            href="/admin/users"
            className="block text-blue-600 hover:underline dark:text-blue-400"
          >
            View all users
          </Link>
          <Link
            href="/admin/register"
            className="block text-blue-600 hover:underline dark:text-blue-400"
          >
            Register a user
          </Link>
          <Link href="/" className="block text-blue-600 hover:underline dark:text-blue-400">
            ← Back to app
          </Link>
        </p>
      </div>
    </div>
  );
}
