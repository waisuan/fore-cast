import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ToastProvider } from '@/contexts/ToastContext';
import HistoryPage from './page';

vi.mock('@/utils/api', () => ({
  api: {
    get: vi.fn(),
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
    history: '/api/v1/history',
  },
}));

import { api, ApiError } from '@/utils/api';

function renderHistory() {
  return render(
    <ToastProvider>
      <HistoryPage />
    </ToastProvider>,
  );
}

describe('HistoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows empty copy when there is no history', async () => {
    vi.mocked(api.get).mockResolvedValue({ attempts: [] });

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText('No booking history yet.')).toBeInTheDocument();
    });
    expect(api.get).toHaveBeenCalledWith('/api/v1/history');
  });

  it('shows an error toast when the request fails', async () => {
    vi.mocked(api.get).mockRejectedValue(new ApiError('upstream', 503));

    renderHistory();

    await waitFor(() => {
      expect(screen.getByText('upstream')).toBeInTheDocument();
    });
  });

  it('shows the run log steps for an attempt', async () => {
    vi.mocked(api.get).mockResolvedValue({
      attempts: [
        {
          id: 1,
          created_at: '2026-11-27T14:00:03Z',
          course_id: 'PLC',
          txn_date: '2026/12/04',
          tee_time: '',
          tee_box: '',
          booking_id: '',
          status: 'failed',
          message: '[PLC] no slot booked — last club reply: This Flight Is Blocked',
          steps: [
            { at: '2026-11-27T14:00:01Z', text: '7:37 AM tee 1: This Flight Is Blocked' },
            { at: '2026-11-27T14:00:03Z', text: 'Done: nothing booked.' },
          ],
        },
      ],
    });

    renderHistory();

    await waitFor(() => {
      expect(screen.getAllByText('What happened (2 steps)').length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText('7:37 AM tee 1: This Flight Is Blocked').length).toBeGreaterThan(0);
    expect(screen.getAllByText('10:00:01 PM').length).toBeGreaterThan(0);
  });
});
