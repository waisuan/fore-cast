'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';

const navLinks = [
  { href: '/', label: 'Home' },
  { href: '/slots', label: 'Slots' },
  { href: '/auto', label: 'Auto-book' },
  { href: '/booking', label: 'Bookings' },
  { href: '/history', label: 'History' },
  { href: '/settings', label: 'Settings' },
];

export default function Header() {
  const { user, logout } = useAuth();
  const pathname = usePathname();

  return (
    <header className="sticky top-0 z-10 border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
      <div className="container mx-auto flex max-w-2xl items-center justify-between px-4 py-3">
        <div className="flex items-center gap-6">
          <Link
            href="/"
            className="text-lg font-semibold text-gray-900 dark:text-white"
          >
            fore-cast
          </Link>
          <nav className="hidden items-center gap-4 sm:flex">
            {navLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={`text-sm ${
                  pathname === link.href
                    ? 'font-medium text-blue-600 dark:text-blue-400'
                    : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'
                }`}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
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
