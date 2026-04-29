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

// react-day-picker pulls in CSS that vitest's PostCSS config rejects, so we
// substitute a plain text input with the same value/onChange contract.
vi.mock('@/components/DatePicker', () => ({
  default: ({
    id,
    value,
    onChange,
    min,
    'aria-label': ariaLabel,
  }: {
    id?: string;
    value: string;
    onChange: (v: string) => void;
    min?: string;
    'aria-label'?: string;
  }) => (
    <input
      id={id}
      data-testid="date-picker"
      type="text"
      value={value}
      data-min={min}
      aria-label={ariaLabel}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
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
  override_course: '',
  override_until: null as string | null,
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

  it('submits the form via PUT /preset, always clearing course (auto by day-of-week)', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save settings/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /save settings/i }));

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith(
        '/api/v1/preset',
        expect.objectContaining({
          course: '',
          cutoff: '10:00',
          enabled: false,
          override_course: '',
          override_until: null,
        }),
      );
    });
  });

  it('does not render a default-course selector', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText(/use this course instead/i)).toBeInTheDocument();
    });
    expect(
      screen.queryByRole('combobox', { name: /default course/i }),
    ).not.toBeInTheDocument();
  });

  it('shows the default course-by-booking-day schedule and the 1-week-ahead rule', async () => {
    renderSettings();

    await waitFor(() => {
      expect(
        screen.getByRole('region', { name: /default course by booking day/i }),
      ).toBeInTheDocument();
    });
    expect(screen.getByText(/Sun, Mon, Tue/i)).toBeInTheDocument();
    expect(screen.getByText(/Wed, Thu, Fri, Sat/i)).toBeInTheDocument();
    expect(screen.getByText(/7 days ahead/i)).toBeInTheDocument();
    expect(screen.getByText(/Next run targets/i)).toBeInTheDocument();
  });

  it('saves a "next run only" override when course is picked without a duration change', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText(/use this course instead/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/use this course instead/i), {
      target: { value: 'PLC' },
    });

    fireEvent.click(screen.getByRole('button', { name: /save settings/i }));

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith(
        '/api/v1/preset',
        expect.objectContaining({
          override_course: 'PLC',
          override_until: null,
        }),
      );
    });
  });

  it('saves a 7-day override with an end-of-day Malaysia expiry', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText(/use this course instead/i)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/use this course instead/i), {
      target: { value: 'BRC' },
    });
    fireEvent.click(screen.getByLabelText(/next 7 days/i));

    fireEvent.click(screen.getByRole('button', { name: /save settings/i }));

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith(
        '/api/v1/preset',
        expect.objectContaining({
          override_course: 'BRC',
          override_until: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T23:59:59\+08:00$/),
        }),
      );
    });
  });

  it('hydrates an existing active override from the GET payload', async () => {
    vi.mocked(api.get).mockResolvedValue({
      ...presetPayload,
      override_course: 'PLC',
      override_until: '2099-12-31T23:59:59+08:00',
    });
    renderSettings();

    await waitFor(() => {
      expect(screen.getByLabelText(/until a specific date/i)).toBeChecked();
    });
    expect(screen.getByLabelText(/use this course instead/i)).toHaveValue('PLC');
  });
});
