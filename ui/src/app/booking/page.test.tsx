import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ToastProvider } from '@/contexts/ToastContext';
import BookingPage from './page';

vi.mock('@/utils/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
  ApiError: class ApiError extends Error {
    name = 'ApiError';
    constructor(
      message: string,
      public status = 500,
    ) {
      super(message);
    }
  },
  API_ENDPOINTS: {
    preset: '/api/v1/preset',
    booking: '/api/v1/booking',
    bookingCancel: '/api/v1/booking/cancel',
    presetCancel: '/api/v1/preset/cancel',
  },
}));

import { api } from '@/utils/api';

const sampleBooking = {
  BookingID: 'BK-1',
  TxnDate: '2026/03/01',
  CourseID: 'PLC',
  CourseName: 'Test Course',
  TeeTime: '1899-12-30T07:30:00',
  Session: 'Morning',
  TeeBox: '1',
  Pax: 4,
  Hole: 18,
  Name: 'Player',
};

function renderBooking() {
  return render(
    <ToastProvider>
      <BookingPage />
    </ToastProvider>,
  );
}

describe('BookingPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.get).mockImplementation((path: string) => {
      if (path === '/api/v1/preset') {
        return Promise.resolve({ last_run_status: 'idle' });
      }
      if (path === '/api/v1/booking') {
        return Promise.resolve({
          Status: true,
          Result: [sampleBooking],
        });
      }
      return Promise.reject(new Error(`unmocked ${path}`));
    });
    vi.mocked(api.post).mockResolvedValue(undefined);
  });

  it('loads bookings and shows booking id', async () => {
    renderBooking();

    await waitFor(() => {
      expect(screen.getByText(/Booking ID: BK-1/)).toBeInTheDocument();
    });
  });

  it('cancels a booking when confirm returns true', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);

    renderBooking();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /cancel booking/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /cancel booking/i }));

    expect(confirmSpy).toHaveBeenCalled();
    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/api/v1/booking/cancel', {
        bookingID: 'BK-1',
      });
    });

    confirmSpy.mockRestore();
  });
});
