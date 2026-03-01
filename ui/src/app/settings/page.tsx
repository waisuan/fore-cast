'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';

interface PresetDefaults {
  course: string;
  cutoff: string;
  retry_interval: number;
  timeout: string;
}

interface PresetData {
  user_name: string;
  course: string;
  cutoff: string;
  retry_interval: number;
  timeout: string;
  ntfy_topic: string;
  enabled: boolean;
  has_password: boolean;
  defaults: PresetDefaults;
}

export default function SettingsPage() {
  const { addToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [defaults, setDefaults] = useState<PresetDefaults | null>(null);
  const [password, setPassword] = useState('');
  const [course, setCourse] = useState('');
  const [cutoff, setCutoff] = useState('');
  const [retryInterval, setRetryInterval] = useState(1);
  const [timeout, setTimeout] = useState('');
  const [ntfyTopic, setNtfyTopic] = useState('');
  const [enabled, setEnabled] = useState(false);
  const [hasPassword, setHasPassword] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<PresetData>(API_ENDPOINTS.preset);
      setDefaults(res.defaults);
      setCourse(res.course ?? '');
      setCutoff(res.cutoff ?? '');
      setRetryInterval(res.retry_interval || res.defaults.retry_interval);
      setTimeout(res.timeout ?? '');
      setNtfyTopic(res.ntfy_topic ?? '');
      setEnabled(res.enabled ?? false);
      setHasPassword(res.has_password ?? false);
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to load settings', 'error');
    } finally {
      setLoading(false);
    }
  }, [addToast]);

  useEffect(() => {
    load();
  }, [load]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!hasPassword && !password) {
      addToast('Password is required for auto-booker', 'error');
      return;
    }
    setSaving(true);
    try {
      await api.put(API_ENDPOINTS.preset, {
        password: password || undefined,
        course,
        cutoff,
        retry_interval: retryInterval,
        timeout,
        ntfy_topic: ntfyTopic,
        enabled,
      });
      setPassword('');
      setHasPassword(true);
      addToast('Settings saved', 'success');
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to save settings', 'error');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Spinner className="h-6 w-6" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-gray-900 dark:text-white">
        Auto-booker settings
      </h1>
      <p className="text-sm text-gray-600 dark:text-gray-400">
        Configure your auto-booking preset. When enabled, the scheduler will
        automatically attempt to book a slot for <strong>1 week ahead</strong> on
        your behalf each time it runs.
      </p>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div>
          <label htmlFor="password" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Password {hasPassword ? '(saved — leave blank to keep current)' : '*'}
          </label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={hasPassword ? '••••••••' : 'Required'}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            autoComplete="new-password"
          />
        </div>
        <div>
          <label htmlFor="course" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Course override
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Default: {defaults?.course}
          </p>
          <select
            id="course"
            value={course}
            onChange={(e) => setCourse(e.target.value)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          >
            <option value="">Auto (by day of week)</option>
            <option value="BRC">BRC</option>
            <option value="PLC">PLC</option>
          </select>
        </div>
        <div>
          <label htmlFor="cutoff" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Cutoff time
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Only book slots before this time. Default: {defaults?.cutoff}
          </p>
          <input
            id="cutoff"
            type="time"
            value={cutoff}
            onChange={(e) => setCutoff(e.target.value)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="retryInterval" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Retry interval (seconds)
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Pause between booking attempts. Default: {defaults?.retry_interval}
          </p>
          <input
            id="retryInterval"
            type="number"
            min={1}
            value={retryInterval}
            onChange={(e) => setRetryInterval(parseInt(e.target.value, 10) || 1)}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="timeout" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            Timeout
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Stop retrying after this duration. Default: {defaults?.timeout}
          </p>
          <input
            id="timeout"
            type="text"
            value={timeout}
            onChange={(e) => setTimeout(e.target.value)}
            placeholder={defaults?.timeout}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <label htmlFor="ntfyTopic" className="mb-1 block text-sm text-gray-700 dark:text-gray-300">
            ntfy.sh topic
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Optional. Receive push notifications on success/failure.
          </p>
          <input
            id="ntfyTopic"
            type="text"
            value={ntfyTopic}
            onChange={(e) => setNtfyTopic(e.target.value)}
            placeholder="e.g. fore-cast-prod-abc123"
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div className="flex items-center gap-2">
          <input
            id="enabled"
            type="checkbox"
            checked={enabled}
            onChange={(e) => setEnabled(e.target.checked)}
            className="h-4 w-4 rounded border-gray-300"
          />
          <label htmlFor="enabled" className="text-sm text-gray-700 dark:text-gray-300">
            Enable auto-booking
          </label>
        </div>
        <button
          type="submit"
          disabled={saving}
          className="w-full max-w-xs rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? <Spinner className="h-4 w-4 text-white" /> : 'Save settings'}
        </button>
      </form>
    </div>
  );
}
