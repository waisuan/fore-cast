'use client';

import Spinner from './Spinner';

export type SchedulerRunningBannerProps = {
  cancelLoading: boolean;
  onCancel: () => void;
};

export default function SchedulerRunningBanner({
  cancelLoading,
  onCancel,
}: SchedulerRunningBannerProps) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800 sm:flex-row sm:items-center sm:justify-between dark:border-blue-800 dark:bg-blue-900/30 dark:text-blue-300">
      <p>
        Scheduler is running. Slots and booking require 3rd party access and are unavailable.
        You can cancel the run below.
      </p>
      <button
        type="button"
        onClick={onCancel}
        disabled={cancelLoading}
        aria-busy={cancelLoading}
        className="shrink-0 rounded border border-blue-600 bg-white px-3 py-1.5 font-medium text-blue-800 hover:bg-blue-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-blue-400 dark:bg-blue-950/50 dark:text-blue-200 dark:hover:bg-blue-900/50"
      >
        {cancelLoading ? <Spinner className="h-4 w-4" /> : 'Cancel run'}
      </button>
    </div>
  );
}
