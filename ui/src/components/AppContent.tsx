'use client';

import { ReactNode, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import Header from './Header';
import LoginPage from './LoginPage';
import Spinner from './Spinner';

export default function AppContent({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { isAuthenticated, isLoading, user } = useAuth();
  const isAdminRoute = pathname?.startsWith('/admin');

  useEffect(() => {
    if (!isLoading && isAdminRoute && isAuthenticated && user && user.role !== 'ADMIN') {
      router.replace('/');
    }
  }, [isLoading, isAdminRoute, isAuthenticated, user, router]);

  if (isAdminRoute) {
    if (isLoading) {
      return (
        <div className="flex min-h-screen items-center justify-center">
          <Spinner className="h-8 w-8" />
        </div>
      );
    }
    if (!isAuthenticated) {
      return <LoginPage />;
    }
    if (user?.role !== 'ADMIN') {
      return (
        <div className="flex min-h-screen items-center justify-center">
          <Spinner className="h-8 w-8" />
        </div>
      );
    }
    return <>{children}</>;
  }

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Spinner className="h-8 w-8" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <LoginPage />;
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <Header />
      <main className="container mx-auto max-w-2xl px-4 py-6 sm:py-8">{children}</main>
    </div>
  );
}
