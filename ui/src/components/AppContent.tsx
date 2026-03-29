'use client';

import { ReactNode, useEffect } from 'react';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/contexts/AuthContext';
import AdminHeader from './AdminHeader';
import Header from './Header';
import LoginPage from './LoginPage';
import Spinner from './Spinner';

export default function AppContent({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { isAuthenticated, isLoading, user } = useAuth();
  const isAdmin = user?.role === 'ADMIN';
  const onAdminPath = pathname?.startsWith('/admin') ?? false;

  useEffect(() => {
    if (isLoading || !isAuthenticated || !user) return;
    if (user.role === 'ADMIN' && !onAdminPath) {
      router.replace('/admin/users');
    }
    if (user.role !== 'ADMIN' && onAdminPath) {
      router.replace('/');
    }
  }, [isLoading, isAuthenticated, user, onAdminPath, router]);

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

  if (isAdmin) {
    if (!onAdminPath) {
      return (
        <div className="flex min-h-screen items-center justify-center">
          <Spinner className="h-8 w-8" />
        </div>
      );
    }
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
        <AdminHeader />
        <main className="container mx-auto max-w-2xl px-4 py-6 sm:py-8">{children}</main>
      </div>
    );
  }

  if (onAdminPath) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Spinner className="h-8 w-8" />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <Header />
      <main className="container mx-auto max-w-2xl px-4 py-6 sm:py-8">{children}</main>
    </div>
  );
}
