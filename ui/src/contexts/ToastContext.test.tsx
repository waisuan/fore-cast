import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { ToastProvider, useToast } from './ToastContext';

function Harness() {
  const { addToast, dismissToast, toasts } = useToast();
  return (
    <div>
      <button type="button" onClick={() => addToast('hello', 'info')}>
        add-info
      </button>
      <button type="button" onClick={() => addToast('oops', 'error')}>
        add-error
      </button>
      {toasts.map((t) => (
        <button key={t.id} type="button" onClick={() => dismissToast(t.id)}>
          dismiss-{t.id}
        </button>
      ))}
    </div>
  );
}

function renderHarness() {
  return render(
    <ToastProvider>
      <Harness />
    </ToastProvider>,
  );
}

describe('ToastProvider', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders toast stack with pointer-events-none on the live region so toasts opt back in', () => {
    renderHarness();

    const region = screen.getByRole('status');
    expect(region).toHaveClass('pointer-events-none');

    fireEvent.click(screen.getByRole('button', { name: 'add-info' }));
    const toast = screen.getByText('hello');
    expect(toast.closest('.pointer-events-auto')).toBeTruthy();
  });

  it('auto-dismiss removes info toasts after 4s', () => {
    const { unmount } = renderHarness();

    fireEvent.click(screen.getByRole('button', { name: 'add-info' }));
    expect(screen.getByText('hello')).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(4000);
    });
    expect(screen.queryByText('hello')).not.toBeInTheDocument();

    unmount();
  });

  it('clears timeouts on unmount so setState does not run after teardown', () => {
    const { unmount } = renderHarness();

    fireEvent.click(screen.getByRole('button', { name: 'add-info' }));
    unmount();

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
  });

  it('dismiss clears auto-dismiss timeout for that toast', () => {
    renderHarness();

    fireEvent.click(screen.getByRole('button', { name: 'add-info' }));
    expect(screen.getByText('hello')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /^dismiss-/ }));
    expect(screen.queryByText('hello')).not.toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(screen.queryByText('hello')).not.toBeInTheDocument();
  });

  it('uses longer duration for error toasts', () => {
    renderHarness();

    fireEvent.click(screen.getByRole('button', { name: 'add-error' }));
    expect(screen.getByText('oops')).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(4000);
    });
    expect(screen.getByText('oops')).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.queryByText('oops')).not.toBeInTheDocument();
  });
});
