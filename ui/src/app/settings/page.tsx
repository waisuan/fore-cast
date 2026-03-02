'use client';

import { useState, useEffect, useCallback } from 'react';
import { api, ApiError, API_ENDPOINTS } from '@/utils/api';
import { useToast } from '@/contexts/ToastContext';
import Spinner from '@/components/Spinner';

interface PresetDefaults {
  course: string;
  cutoff: string;
  retry_interval: string;
  min_retry_interval: string;
  timeout: string;
}

interface PresetData {
  user_name: string;
  course: string;
  cutoff: string;
  retry_interval: string;
  timeout: string;
  ntfy_topic: string;
  enable_notifications: boolean;
  enabled: boolean;
  defaults: PresetDefaults;
  last_run_status: string;
  last_run_message: string;
  last_run_at: string | null;
}

export default function SettingsPage() {
  const { addToast } = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [defaults, setDefaults] = useState<PresetDefaults | null>(null);
  const [course, setCourse] = useState('');
  const [cutoff, setCutoff] = useState('');
  const [retryIntervalVal, setRetryIntervalVal] = useState('');
  const [timeoutVal, setTimeoutVal] = useState('');
  const [ntfyTopic, setNtfyTopic] = useState('');
  const [enableNotifications, setEnableNotifications] = useState(false);
  const [enabled, setEnabled] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.get<PresetData>(API_ENDPOINTS.preset);
      setDefaults(res.defaults ?? null);
      setCourse(res.course ?? '');
      setCutoff(res.cutoff ?? '');
      setRetryIntervalVal(res.retry_interval ?? res.defaults?.retry_interval ?? '1s');
      setTimeoutVal(res.timeout ?? '');
      setNtfyTopic(res.ntfy_topic ?? '');
      setEnableNotifications(res.enable_notifications ?? false);
      setEnabled(res.enabled ?? false);
    } catch (e) {
      addToast(e instanceof ApiError ? e.message : 'Failed to load settings', 'error');
    } finally {
      setLoading(false);
    }
  }, [addToast]);

  useEffect(() => {
    load();
  }, [load]);

  const MIN_RETRY_MS = 500;

  function parseDurationMs(s: string): number | null {
    const match = s.trim().match(/^(\d+(?:\.\d+)?)\s*(ms|s|m|h)$/i);
    if (!match) return null;
    const n = parseFloat(match[1]);
    const unit = match[2].toLowerCase();
    if (unit === 'ms') return n;
    if (unit === 's') return n * 1000;
    if (unit === 'm') return n * 60 * 1000;
    if (unit === 'h') return n * 3600 * 1000;
    return null;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      let retryInterval = retryIntervalVal.trim();
      const retryMs = parseDurationMs(retryInterval);
      if (retryMs !== null && retryMs < MIN_RETRY_MS) {
        retryInterval = '500ms';
        addToast('Retry interval must be at least 500ms, adjusted to 500ms', 'info');
      } else if (retryMs === null && retryInterval !== '') {
        addToast('Invalid retry interval format (e.g. 1s, 500ms)', 'error');
        setSaving(false);
        return;
      }
      const timeout = timeoutVal.trim();
      if (timeout !== '' && parseDurationMs(timeout) === null) {
        addToast('Invalid timeout format (e.g. 10m, 1h, 30s)', 'error');
        setSaving(false);
        return;
      }
      await api.put(API_ENDPOINTS.preset, {
        course,
        cutoff,
        retry_interval: retryInterval || undefined,
        timeout: timeoutVal,
        enable_notifications: enableNotifications,
        enabled,
      });
      addToast('Settings saved', 'success');
      await load();
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
        <fieldset disabled={saving} className="flex flex-col gap-4" aria-busy={saving}>
          <legend className="sr-only">Auto-booker configuration</legend>
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
            Retry interval
          </label>
          <p className="mb-1 text-xs text-gray-500 dark:text-gray-400">
            Pause between booking attempts (e.g. 1s, 500ms). Min: {defaults?.min_retry_interval ?? '500ms'}. Default: {defaults?.retry_interval}
          </p>
          <input
            id="retryInterval"
            type="text"
            value={retryIntervalVal}
            onChange={(e) => setRetryIntervalVal(e.target.value)}
            placeholder={defaults?.retry_interval}
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
            value={timeoutVal}
            onChange={(e) => setTimeoutVal(e.target.value)}
            placeholder={defaults?.timeout}
            className="w-full max-w-xs rounded border border-gray-300 px-3 py-2 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
          />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <input
              id="enableNotifications"
              type="checkbox"
              checked={enableNotifications}
              onChange={(e) => setEnableNotifications(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <label htmlFor="enableNotifications" className="text-sm text-gray-700 dark:text-gray-300">
              Enable push notifications
            </label>
          </div>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Receive notifications on booking success or failure via{' '}
            <a
              href="https://ntfy.sh"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 underline dark:text-blue-400"
            >
              ntfy.sh
            </a>
            . Download the{' '}
            <a
              href="https://ntfy.sh/#subscribe-phone"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 underline dark:text-blue-400"
            >
              ntfy app
            </a>
            {' '}on your device and subscribe to your topic below.
          </p>
          {enableNotifications && ntfyTopic && (
            <div className="mt-2 rounded border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-600 dark:bg-gray-700">
              <p className="text-xs text-gray-500 dark:text-gray-400">Your topic:</p>
              <p className="font-mono text-sm text-gray-900 dark:text-white">{ntfyTopic}</p>
            </div>
          )}
          {enableNotifications && !ntfyTopic && (
            <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
              A unique topic will be generated when you save.
            </p>
          )}
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
        </fieldset>
        <button
          type="submit"
          disabled={saving}
          aria-busy={saving}
          className="w-full max-w-xs rounded bg-blue-600 px-4 py-2 font-medium text-white hover:bg-blue-700 disabled:opacity-50"
        >
          {saving ? <Spinner className="h-4 w-4 text-white" /> : 'Save settings'}
        </button>
      </form>
    </div>
  );
}
