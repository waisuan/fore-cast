'use client';

import { ReactNode } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import Header from './Header';
import LoginPage from './LoginPage';

export default function AppContent({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-foreground">Loading...</p>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <LoginPage />;
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <Header />
      <main className="container mx-auto max-w-2xl px-4 py-6 sm:py-8">
        {children}
      </main>
    </div>
  );
}
