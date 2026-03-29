'use client';

import { useState } from 'react';
import Link from 'next/link';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';

export default function AdminRegisterPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setSuccess('');
    if (!username || !password) {
      setError('Club member ID and password are required');
      return;
    }
    setSubmitting(true);
    try {
      await api.post(API_ENDPOINTS.adminRegister, { username, password });
      setSuccess(`Registered ${username}. They can now log in and configure their preset.`);
      setUsername('');
      setPassword('');
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Registration failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="mx-auto w-full max-w-sm rounded-lg border border-gray-200 bg-white p-6 shadow dark:border-gray-700 dark:bg-gray-800">
        <h1 className="mb-6 text-xl font-semibold text-gray-900 dark:text-white">
          Admin: Register user
        </h1>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <p className="text-sm font-medium text-gray-700 dark:text-gray-300">
              New user (3rd party credentials)
            </p>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Must match exactly what the user uses for the club&apos;s booking system.
            </p>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="Club member ID"
              className="w-full rounded border border-gray-300 px-3 py-2 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              autoComplete="off"
            />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="3rd party password"
              className="w-full rounded border border-gray-300 px-3 py-2 text-gray-900 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
              autoComplete="new-password"
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
          <button
            type="submit"
            disabled={submitting}
            aria-busy={submitting}
            className="w-full rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
          >
            {submitting ? 'Registering…' : 'Register'}
          </button>
        </form>
        <p className="mt-4 text-center text-sm">
          <Link
            href="/admin/users"
            className="text-blue-600 hover:underline dark:text-blue-400"
          >
            View all users
          </Link>
        </p>
    </div>
  );
}
