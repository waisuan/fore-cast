'use client';

import Link from 'next/link';
import { useAuth } from '@/contexts/AuthContext';

export default function Header() {
  const { user, logout } = useAuth();

  return (
    <header className="sticky top-0 z-10 border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
      <div className="container mx-auto flex max-w-2xl items-center justify-between px-4 py-3">
        <Link
          href="/"
          className="text-lg font-semibold text-gray-900 dark:text-white"
        >
          Alfred
        </Link>
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray-600 dark:text-gray-400">
            {user?.username}
          </span>
          <button
            type="button"
            onClick={() => logout()}
            className="text-sm text-blue-600 hover:underline dark:text-blue-400"
          >
            Log out
          </button>
        </div>
      </div>
    </header>
  );
}
