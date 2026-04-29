'use client';

import Link from 'next/link';
import { formatShortDateMY, isoToYmdMalaysia } from '@/utils/date';

interface CourseOverrideBannerProps {
  overrideCourse: string;
  overrideUntil: string | null;
}

export default function CourseOverrideBanner({
  overrideCourse,
  overrideUntil,
}: CourseOverrideBannerProps) {
  if (!overrideCourse) return null;

  const revertCopy = overrideUntil
    ? `until ${formatShortDateMY(isoToYmdMalaysia(overrideUntil))}`
    : 'for the next run only';

  return (
    <div className="rounded-lg border-2 border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-700/70 dark:bg-amber-900/20 dark:text-amber-100">
      <p>
        Auto-booker will use{' '}
        <strong className="font-semibold">{overrideCourse}</strong> {revertCopy}, then revert to
        the day&rsquo;s default course.{' '}
        <Link
          href="/settings"
          className="font-medium underline underline-offset-2 hover:no-underline"
        >
          Change
        </Link>
      </p>
    </div>
  );
}
