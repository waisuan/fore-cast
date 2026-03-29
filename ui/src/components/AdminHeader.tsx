'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';

const adminNavLinks = [
  { href: '/admin/users', label: 'Users' },
  { href: '/admin/register', label: 'Register user' },
  { href: '/admin/delete', label: 'Remove user / preset' },
] as const;

export default function AdminHeader() {
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
    <header className="sticky top-0 z-10 border-b border-amber-300/90 bg-gradient-to-b from-amber-100 to-amber-50 shadow-sm dark:border-amber-800 dark:from-amber-950 dark:to-zinc-950 dark:shadow-[inset_0_-1px_0_0_rgba(251,191,36,0.12)]">
      <div className="container mx-auto flex max-w-2xl items-center justify-between px-4 py-3">
        <div className="flex flex-wrap items-center gap-4 sm:gap-6">
          <Link
            href="/admin/users"
            className="text-lg font-semibold tracking-tight text-amber-950 dark:text-amber-50"
          >
            fore-cast
          </Link>
          <span className="rounded-md border border-amber-400/60 bg-white/80 px-2 py-0.5 text-xs font-semibold uppercase tracking-wide text-amber-950 shadow-sm dark:border-amber-600/50 dark:bg-amber-900/50 dark:text-amber-100">
            Admin
          </span>
          <nav className="hidden items-center gap-4 sm:flex">
            {adminNavLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={`text-sm ${
                  pathname === link.href || pathname?.startsWith(link.href + '/')
                    ? 'font-semibold text-amber-900 dark:text-amber-200'
                    : 'text-amber-900/75 hover:text-amber-950 dark:text-amber-200/80 dark:hover:text-amber-100'
                }`}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
        <div className="flex items-center gap-4">
          <span className="hidden max-w-[10rem] truncate text-sm text-amber-900/85 dark:text-amber-200/90 sm:inline">
            {user?.username}
          </span>
          <button
            type="button"
            onClick={() => setMobileMenuOpen((o) => !o)}
            className="rounded p-2 text-amber-900 hover:bg-amber-200/50 dark:text-amber-200 dark:hover:bg-amber-900/60 sm:hidden"
            aria-label={mobileMenuOpen ? 'Close menu' : 'Open menu'}
            aria-expanded={mobileMenuOpen}
          >
            <svg
              className="h-5 w-5"
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
            className="text-sm font-medium text-amber-950 underline-offset-2 hover:underline disabled:opacity-50 dark:text-amber-100"
          >
            {loggingOut ? 'Logging out…' : 'Log out'}
          </button>
        </div>
      </div>
      {mobileMenuOpen && (
        <div className="border-t border-amber-300/80 bg-amber-50/80 px-4 py-3 dark:border-amber-800 dark:bg-amber-950/80 sm:hidden">
          <p className="mb-2 text-xs text-amber-800/90 dark:text-amber-300/90">{user?.username}</p>
          <nav className="flex flex-col gap-2">
            {adminNavLinks.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                onClick={() => setMobileMenuOpen(false)}
                className={`text-sm ${
                  pathname === link.href
                    ? 'font-semibold text-amber-950 dark:text-amber-100'
                    : 'text-amber-900/80 hover:text-amber-950 dark:text-amber-200/85 dark:hover:text-amber-50'
                }`}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        </div>
      )}
    </header>
  );
}
