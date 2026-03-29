'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';

const navLinks = [
  { href: '/', label: 'Home' },
  { href: '/slots', label: 'Slots' },
  { href: '/booking', label: 'Bookings' },
  { href: '/history', label: 'History' },
  { href: '/settings', label: 'Settings' },
];

function NavLinks({
  pathname,
  className,
  onLinkClick,
}: {
  pathname: string;
  className?: string;
  onLinkClick?: () => void;
}) {
  return (
    <nav className={className}>
      {navLinks.map((link) => (
        <Link
          key={link.href}
          href={link.href}
          onClick={onLinkClick}
          className={`block text-sm ${
            pathname === link.href
              ? 'font-medium text-blue-600 dark:text-blue-400'
              : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'
          }`}
        >
          {link.label}
        </Link>
      ))}
    </nav>
  );
}

export default function Header() {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const [loggingOut, setLoggingOut] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  async function handleLogout() {
    setLoggingOut(true);
    try {
      await logout();
    } finally {
      setLoggingOut(false);
    }
  }

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
          <NavLinks
            pathname={pathname}
            className="hidden items-center gap-4 sm:flex"
          />
        </div>
        <div className="flex items-center gap-4">
          <span className="hidden text-sm text-gray-600 dark:text-gray-400 sm:inline">
            {user?.username}
          </span>
          <button
            type="button"
            onClick={() => setMobileMenuOpen((o) => !o)}
            className="rounded p-2 sm:hidden"
            aria-label={mobileMenuOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={mobileMenuOpen}
          >
            <svg
              className="h-5 w-5 text-gray-600 dark:text-gray-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              {mobileMenuOpen ? (
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              ) : (
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 6h16M4 12h16M4 18h16"
                />
              )}
            </svg>
          </button>
          <button
            type="button"
            onClick={handleLogout}
            disabled={loggingOut}
            className="text-sm text-blue-600 hover:underline disabled:opacity-50 dark:text-blue-400"
          >
            {loggingOut ? 'Logging out…' : 'Log out'}
          </button>
        </div>
      </div>
      {mobileMenuOpen && (
        <div className="border-t border-gray-200 px-4 py-3 dark:border-gray-700 sm:hidden">
          <div className="flex flex-col gap-3">
            <p className="text-xs text-gray-500 dark:text-gray-400">{user?.username}</p>
            <NavLinks
              pathname={pathname}
              className="flex flex-col gap-2"
              onLinkClick={() => setMobileMenuOpen(false)}
            />
          </div>
        </div>
      )}
    </header>
  );
}
