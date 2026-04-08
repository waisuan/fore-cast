import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ToastProvider } from '@/contexts/ToastContext';
import SlotsPage from './page';

vi.mock('@/utils/date', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/utils/date')>();
  return {
    ...actual,
    todayIso: () => '2030-06-01',
  };
});

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
    slots: '/api/v1/slots',
    bookingBook: '/api/v1/booking/book',
    presetCancel: '/api/v1/preset/cancel',
  },
}));

import { api, ApiError } from '@/utils/api';

function renderSlots() {
  return render(
    <ToastProvider>
      <SlotsPage />
    </ToastProvider>,
  );
}

function mockPresetIdle() {
  vi.mocked(api.get).mockImplementation((path: string) => {
    if (path === '/api/v1/preset') {
      return Promise.resolve({ last_run_status: 'idle' });
    }
    return Promise.reject(new Error(`unmocked GET ${path}`));
  });
}

describe('SlotsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockPresetIdle();
  });

  it('loads preset on mount and renders date field inside a label with min=todayIso', async () => {
    renderSlots();

    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith('/api/v1/preset');
    });

    expect(screen.getByRole('heading', { name: /available slots/i })).toBeInTheDocument();

    const dateInput = document.querySelector('input[type="date"]') as HTMLInputElement | null;
    expect(dateInput).toBeTruthy();
    expect(dateInput?.min).toBe('2030-06-01');

    const label = dateInput?.closest('label');
    expect(label).toBeTruthy();
    expect(label?.textContent).toContain('Date');
  });

  it('shows scheduler banner and hides slot form when preset is running', async () => {
    vi.mocked(api.get).mockResolvedValue({ last_run_status: 'running' });
    renderSlots();

    await waitFor(() => {
      expect(screen.getByText(/scheduler is running/i)).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: /load slots/i })).not.toBeInTheDocument();
  });

  it('shows a toast when Load slots is clicked without a date', async () => {
    renderSlots();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/api/v1/preset'));

    fireEvent.click(screen.getByRole('button', { name: /load slots/i }));

    await waitFor(() => {
      expect(screen.getByText('Please pick a date')).toBeInTheDocument();
    });
  });

  it('requests slots with API-formatted date when a date is set', async () => {
    vi.mocked(api.get).mockImplementation((path: string) => {
      if (path === '/api/v1/preset') {
        return Promise.resolve({ last_run_status: 'idle' });
      }
      if (path.startsWith('/api/v1/slots')) {
        return Promise.resolve({ course: 'BRC', slots: [] });
      }
      return Promise.reject(new Error(`unmocked GET ${path}`));
    });

    renderSlots();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/api/v1/preset'));

    fireEvent.change(document.querySelector('#date')!, {
      target: { value: '2030-06-15' },
    });
    fireEvent.click(screen.getByRole('button', { name: /load slots/i }));

    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith(
        expect.stringMatching(/\/api\/v1\/slots\?date=2030%2F06%2F15/),
      );
    });
  });

  it('includes course in the query when selected', async () => {
    vi.mocked(api.get).mockImplementation((path: string) => {
      if (path === '/api/v1/preset') {
        return Promise.resolve({ last_run_status: 'idle' });
      }
      if (path.startsWith('/api/v1/slots')) {
        return Promise.resolve({ course: 'PLC', slots: [] });
      }
      return Promise.reject(new Error(`unmocked GET ${path}`));
    });

    renderSlots();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/api/v1/preset'));

    fireEvent.change(document.querySelector('#date')!, {
      target: { value: '2030-08-01' },
    });
    fireEvent.change(screen.getByLabelText(/course/i), { target: { value: 'PLC' } });
    fireEvent.click(screen.getByRole('button', { name: /load slots/i }));

    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith(
        expect.stringMatching(/[?&]course=PLC/),
      );
    });
  });

  it('renders loaded slots and can book with the selected date', async () => {
    vi.mocked(api.get).mockImplementation((path: string) => {
      if (path === '/api/v1/preset') {
        return Promise.resolve({ last_run_status: 'idle' });
      }
      if (path.startsWith('/api/v1/slots')) {
        return Promise.resolve({
          course: 'PLC',
          slots: [
            {
              TeeTime: '1899-12-30T07:00:00',
              Session: 'Morning',
              TeeBox: '1',
              CourseID: 'PLC',
            },
          ],
        });
      }
      return Promise.reject(new Error(`unmocked GET ${path}`));
    });
    vi.mocked(api.post).mockResolvedValue({ bookingID: 'bid-99' });

    renderSlots();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/api/v1/preset'));

    fireEvent.change(document.querySelector('#date')!, {
      target: { value: '2030-12-20' },
    });
    fireEvent.click(screen.getByRole('button', { name: /load slots/i }));

    await waitFor(() => {
      expect(screen.getByText(/1 slot\(s\)/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /^book$/i }));

    await waitFor(() => {
      expect(screen.getByText(/Booked! ID: bid-99/)).toBeInTheDocument();
    });

    expect(api.post).toHaveBeenCalledWith(
      '/api/v1/booking/book',
      expect.objectContaining({
        courseID: 'PLC',
        txnDate: '2030/12/20',
        session: 'Morning',
        teeBox: '1',
        teeTime: '1899-12-30T07:00:00',
      }),
    );
  });

  it('shows an error toast when the slots request fails', async () => {
    vi.mocked(api.get).mockImplementation((path: string) => {
      if (path === '/api/v1/preset') {
        return Promise.resolve({ last_run_status: 'idle' });
      }
      if (path.startsWith('/api/v1/slots')) {
        return Promise.reject(new ApiError('server said no', 502));
      }
      return Promise.reject(new Error(`unmocked GET ${path}`));
    });

    renderSlots();
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/api/v1/preset'));

    fireEvent.change(document.querySelector('#date')!, {
      target: { value: '2030-01-01' },
    });
    fireEvent.click(screen.getByRole('button', { name: /load slots/i }));

    await waitFor(() => {
      expect(screen.getByText('server said no')).toBeInTheDocument();
    });
  });
});
