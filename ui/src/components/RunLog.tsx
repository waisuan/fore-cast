'use client';

import { useEffect, useRef } from 'react';
import { formatTimeSecMY } from '@/utils/date';

export interface RunStep {
  at: string;
  text: string;
  course?: string;
}

function StepList({ steps, live }: { steps: RunStep[]; live?: boolean }) {
  const listRef = useRef<HTMLOListElement>(null);
  // Tag lines with their course only when BRC and PLC ran together.
  const multiCourse = new Set(steps.map((s) => s.course).filter(Boolean)).size > 1;

  useEffect(() => {
    if (live && listRef.current) listRef.current.scrollTop = listRef.current.scrollHeight;
  }, [live, steps.length]);

  return (
    <ol
      ref={listRef}
      aria-live={live ? 'polite' : undefined}
      aria-label={live ? 'Live run log' : undefined}
      className={`mt-2 space-y-1 border-l-2 border-gray-200 pl-3 dark:border-gray-700 ${live ? 'max-h-72 overflow-y-auto' : ''}`}
    >
      {steps.map((s, i) => (
        <li key={i} className="flex gap-2 text-gray-700 dark:text-gray-300">
          <time dateTime={s.at} className="shrink-0 font-mono text-xs leading-5 text-gray-500 dark:text-gray-400">
            {formatTimeSecMY(s.at)}
          </time>
          <span className="break-words">
            {multiCourse && s.course && (
              <span className="mr-1 rounded border border-gray-300 px-1 text-xs font-medium text-gray-700 dark:border-gray-600 dark:text-gray-300">
                {s.course}
              </span>
            )}
            {s.text}
          </span>
        </li>
      ))}
    </ol>
  );
}

/**
 * Step-by-step log of what the scheduler did. Collapsed by default; `live`
 * renders it expanded as a polite live region that follows the newest step.
 */
export default function RunLog({ steps, live }: { steps?: RunStep[] | null; live?: boolean }) {
  if (live) {
    return (
      <div className="text-sm">
        {steps?.length ? (
          <StepList steps={steps} live />
        ) : (
          <p className="mt-2 text-gray-600 dark:text-gray-400">Starting…</p>
        )}
      </div>
    );
  }
  if (!steps?.length) return null;
  return (
    <details className="mt-2 text-sm">
      <summary className="cursor-pointer text-blue-700 hover:underline dark:text-blue-400">
        What happened ({steps.length} steps)
      </summary>
      <StepList steps={steps} />
    </details>
  );
}
