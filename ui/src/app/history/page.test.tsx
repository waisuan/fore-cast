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
});
