'use client';

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useRef,
  useEffect,
  type ReactNode,
} from 'react';

type ToastType = 'success' | 'error' | 'info';

interface Toast {
  id: number;
  message: string;
  type: ToastType;
}

interface ToastContextType {
  toasts: Toast[];
  addToast: (message: string, type?: ToastType) => void;
  dismissToast: (id: number) => void;
}

const ToastContext = createContext<ToastContextType | undefined>(undefined);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextIdRef = useRef(0);
  const timeoutByToastIdRef = useRef<Map<number, ReturnType<typeof setTimeout>>>(
    new Map(),
  );

  useEffect(() => {
    const timeoutsRef = timeoutByToastIdRef;
    return () => {
      const pending = timeoutsRef.current;
      for (const t of pending.values()) {
        clearTimeout(t);
      }
      pending.clear();
    };
  }, []);

  const addToast = useCallback((message: string, type: ToastType = 'info') => {
    const id = nextIdRef.current++;
    setToasts((prev) => [...prev, { id, message, type }]);
    const duration = type === 'error' ? 6000 : 4000;
    const timeoutId = setTimeout(() => {
      timeoutByToastIdRef.current.delete(id);
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, duration);
    timeoutByToastIdRef.current.set(id, timeoutId);
  }, []);

  const dismissToast = useCallback((id: number) => {
    const pending = timeoutByToastIdRef.current.get(id);
    if (pending !== undefined) {
      clearTimeout(pending);
      timeoutByToastIdRef.current.delete(id);
    }
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ toasts, addToast, dismissToast }}>
      {children}
      <div
        role="status"
        aria-live="polite"
        className="pointer-events-none fixed left-1/2 top-24 z-50 flex w-full max-w-sm -translate-x-1/2 flex-col gap-2 px-4"
      >
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`pointer-events-auto relative flex items-start gap-2 rounded-lg px-4 py-3 pr-10 text-sm font-medium shadow-lg transition-opacity ${
              t.type === 'success'
                ? 'bg-green-600 text-white'
                : t.type === 'error'
                  ? 'bg-red-600 text-white'
                  : 'bg-gray-800 text-white dark:bg-gray-200 dark:text-gray-900'
            }`}
          >
            <span className="flex-1">{t.message}</span>
            <button
              type="button"
              onClick={() => dismissToast(t.id)}
              className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 opacity-80 hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-white/50"
              aria-label="Dismiss"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const ctx = useContext(ToastContext);
  if (ctx === undefined) {
    throw new Error('useToast must be used within ToastProvider');
  }
  return ctx;
}
