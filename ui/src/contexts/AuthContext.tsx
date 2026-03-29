'use client';

import React, {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  ReactNode,
} from 'react';
import { api, ApiError, API_ENDPOINTS, setOnUnauthorized } from '@/utils/api';

export type UserRole = 'ADMIN' | 'NON_ADMIN';

interface AuthUser {
  username: string;
  role: UserRole;
}

interface AuthContextType {
  user: AuthUser | null;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<{ success: boolean; error?: string }>;
  logout: () => Promise<void>;
  isAuthenticated: boolean;
  isAdmin: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const loadUser = useCallback(async () => {
    try {
      const data = await api.get<{ user: { username: string; role: UserRole } }>(API_ENDPOINTS.me);
      setUser({
        username: data.user.username,
        role: data.user.role,
      });
    } catch {
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadUser();
    setOnUnauthorized(() => setUser(null));
    return () => setOnUnauthorized(null);
  }, [loadUser]);

  const login = useCallback(
    async (
      username: string,
      password: string
    ): Promise<{ success: boolean; error?: string }> => {
      try {
        const data = await api.post<{ user: { username: string; role: UserRole } }>(API_ENDPOINTS.login, {
          username,
          password,
        });
        setUser({
          username: data.user.username,
          role: data.user.role,
        });
        return { success: true };
      } catch (e) {
        const msg = e instanceof ApiError ? e.message : 'Login failed';
        return { success: false, error: msg };
      }
    },
    []
  );

  const logout = useCallback(async () => {
    try {
      await api.post(API_ENDPOINTS.logout);
    } finally {
      setUser(null);
    }
  }, []);

  const isAdmin = user?.role === 'ADMIN';

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        login,
        logout,
        isAuthenticated: user != null,
        isAdmin,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (ctx === undefined) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}
