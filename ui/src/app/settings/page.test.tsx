import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ToastProvider } from '@/contexts/ToastContext';
import SettingsPage from './page';

vi.mock('@/utils/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
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
    presetCancel: '/api/v1/preset/cancel',
  },
}));

import { api } from '@/utils/api';

const presetPayload = {
  user_name: 'tester',
  course: '',
  cutoff: '10:00',
  retry_interval: '1s',
  timeout: '10m',
  ntfy_topic: '',
  enable_notifications: false,
  enabled: false,
  defaults: {
    course: 'BRC',
    cutoff: '09:00',
    retry_interval: '1s',
    min_retry_interval: '0s',
    timeout: '10m',
  },
  last_run_status: 'idle',
  last_run_message: '',
  last_run_at: null as string | null,
};

function renderSettings() {
  return render(
    <ToastProvider>
      <SettingsPage />
    </ToastProvider>,
  );
}

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.get).mockResolvedValue(presetPayload);
    vi.mocked(api.put).mockResolvedValue(undefined);
  });

  it('loads preset and shows the settings heading', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /auto-booker settings/i })).toBeInTheDocument();
    });
    expect(api.get).toHaveBeenCalledWith('/api/v1/preset');
  });

  it('submits the form via PUT /preset', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save settings/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /save settings/i }));

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith(
        '/api/v1/preset',
        expect.objectContaining({
          cutoff: '10:00',
          enabled: false,
        }),
      );
    });
  });
});
