import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ToastProvider } from '@/contexts/ToastContext';
import HomeContent from './HomeContent';

vi.mock('@/utils/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    delete: vi.fn(),
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
    presetSkipNext: '/api/v1/preset/skip-next',
  },
}));

import { api } from '@/utils/api';

interface PresetPayload {
  enabled: boolean;
  last_run_status: string;
  last_run_message: string;
  last_run_at: string | null;
  override_course: string;
  override_until: string | null;
  skip_next_run: boolean;
  alt_course_days: string[];
}

function basePayload(over: Partial<PresetPayload> = {}): PresetPayload {
  return {
    enabled: true,
    last_run_status: 'idle',
    last_run_message: '',
    last_run_at: null,
    override_course: '',
    override_until: null,
    skip_next_run: false,
    alt_course_days: [],
    ...over,
  };
}

function renderHome() {
  return render(
    <ToastProvider>
      <HomeContent />
    </ToastProvider>,
  );
}

describe('HomeContent upcoming banner — alt course sub-line', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Pin "now" to 2026-05-09 12:00 UTC = 2026-05-09 20:00 MY (Saturday,
    // before 21:55 fire). Next fire YMD = 2026-05-09 (Sat), booking YMD =
    // 2026-05-16 (Sat). Backend course for Sat = PLC, so alt course = BRC.
    // Only fake `Date` so React Testing Library's `waitFor` polling (which
    // uses real setTimeout/setInterval) still drives the async assertions.
    vi.useFakeTimers({ toFake: ['Date'] });
    vi.setSystemTime(new Date('2026-05-09T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows "Also trying <alt> in parallel." when the booking weekday is opted in', async () => {
    vi.mocked(api.get).mockResolvedValue(basePayload({ alt_course_days: ['SAT'] }));

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/Next auto-booking:/i)).toBeInTheDocument();
    });
    const alt = screen.getByText(/Also trying/i);
    expect(alt).toBeInTheDocument();
    expect(alt.textContent).toMatch(/BRC/);
  });

  it('omits the alt sub-line when the booking weekday is not opted in', async () => {
    vi.mocked(api.get).mockResolvedValue(basePayload({ alt_course_days: ['MON', 'WED'] }));

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/Next auto-booking:/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/Also trying/i)).not.toBeInTheDocument();
  });

  it('omits the alt sub-line when an override is active for the next fire', async () => {
    // Override BRC pinned through next year — applies to the next fire, so the
    // alt path must be suppressed even though SAT is opted in.
    vi.mocked(api.get).mockResolvedValue(
      basePayload({
        alt_course_days: ['SAT'],
        override_course: 'BRC',
        override_until: '2030-01-01T00:00:00Z',
      }),
    );

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/Next auto-booking:/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/\(override\)/i)).toBeInTheDocument();
    expect(screen.queryByText(/Also trying/i)).not.toBeInTheDocument();
  });

  it('omits the alt sub-line when alt_course_days is empty', async () => {
    vi.mocked(api.get).mockResolvedValue(basePayload({ alt_course_days: [] }));

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/Next auto-booking:/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/Also trying/i)).not.toBeInTheDocument();
  });

  it('also shows the alt sub-line in the skipped variant', async () => {
    // skip_next_run shifts the fire date by +1 day -> 2026-05-10 (Sun),
    // booking YMD = 2026-05-17 (Sun). Backend course for Sun = BRC, alt = PLC.
    vi.mocked(api.get).mockResolvedValue(
      basePayload({ skip_next_run: true, alt_course_days: ['SUN'] }),
    );

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/upcoming auto-booking run is/i)).toBeInTheDocument();
    });
    const alt = screen.getByText(/Also trying/i);
    expect(alt).toBeInTheDocument();
    expect(alt.textContent).toMatch(/PLC/);
  });

  it('hides the upcoming card entirely while the scheduler is running', async () => {
    vi.mocked(api.get).mockResolvedValue(
      basePayload({ last_run_status: 'running', alt_course_days: ['SAT'] }),
    );

    renderHome();

    // The running banner replaces the upcoming card; wait for its cancel
    // button to ensure load() has settled before checking absence.
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /cancel run/i })).toBeInTheDocument();
    });
    expect(screen.queryByText(/Next auto-booking:/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Also trying/i)).not.toBeInTheDocument();
  });

  it('hides the upcoming card when auto-booker is disabled', async () => {
    vi.mocked(api.get).mockResolvedValue(
      basePayload({ enabled: false, alt_course_days: ['SAT'] }),
    );

    renderHome();

    // The static "View slots & book" nav link is always rendered; wait on it
    // to confirm the initial load() resolved.
    await waitFor(() => {
      expect(screen.getByRole('link', { name: /view slots/i })).toBeInTheDocument();
    });
    expect(screen.queryByText(/Next auto-booking:/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Also trying/i)).not.toBeInTheDocument();
  });

  it('coerces a null alt_course_days payload to no alt sub-line', async () => {
    // Backend may return null when the SQL column is invalid; the
    // Array.isArray hydration guard should map that to an empty array.
    vi.mocked(api.get).mockResolvedValue(
      basePayload({ alt_course_days: null as unknown as string[] }),
    );

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/Next auto-booking:/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/Also trying/i)).not.toBeInTheDocument();
  });

  it('shows the alt sub-line when an expired override leaves override_course set', async () => {
    // override_until in the past => override is not active for the next fire,
    // so the alt path should re-engage even though override_course is set.
    // (Backend would normally clear this on GET, but the predicate's timed
    // branch is exercised independently here.)
    vi.mocked(api.get).mockResolvedValue(
      basePayload({
        alt_course_days: ['SAT'],
        override_course: 'BRC',
        override_until: '2020-01-01T00:00:00Z',
      }),
    );

    renderHome();

    await waitFor(() => {
      expect(screen.getByText(/Next auto-booking:/i)).toBeInTheDocument();
    });
    expect(screen.queryByText(/\(override\)/i)).not.toBeInTheDocument();
    const alt = screen.getByText(/Also trying/i);
    expect(alt.textContent).toMatch(/BRC/);
  });
});
