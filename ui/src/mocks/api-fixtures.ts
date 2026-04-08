/**
 * Shared mock JSON for `npm run dev:mock` (middleware) and Playwright `e2e/mock-api.ts`.
 * Not used in production unless NEXT_PUBLIC_USE_MOCK_API is set at build time (don’t do that).
 */

export const MOCK_SESSION_COOKIE = 'forecast_mock_session';

export function mockUser(role: 'ADMIN' | 'NON_ADMIN', username = 'mockuser') {
  return { username, role };
}

export function mockPresetFull() {
  return {
    user_name: 'mockuser',
    course: 'PLC',
    cutoff: '08:15',
    retry_interval: '1s',
    timeout: '10m',
    ntfy_topic: '',
    enable_notifications: false,
    enabled: false,
    defaults: {
      course: 'PLC',
      cutoff: '08:15',
      retry_interval: '1s',
      min_retry_interval: '0s',
      timeout: '10m',
    },
    last_run_status: 'idle',
    last_run_message: '',
    last_run_at: null,
  };
}

export const mockSlotsResponse = {
  course: 'PLC',
  slots: [
    {
      TeeTime: '1899-12-30T07:30:00',
      Session: 'Morning',
      TeeBox: '1',
      CourseID: 'PLC',
      CourseName: 'Palmer Course',
    },
    {
      TeeTime: '1899-12-30T09:00:00',
      Session: 'Morning',
      TeeBox: '10',
      CourseID: 'PLC',
      CourseName: 'Palmer Course',
    },
  ],
};

export const mockBookingResponse = {
  Status: true,
  Result: [
    {
      BookingID: 'MOCK-001',
      TxnDate: '2026-04-08',
      CourseID: 'PLC',
      CourseName: 'Palmer Course',
      TeeTime: '1899-12-30T07:30:00',
      Session: 'Morning',
      TeeBox: '1',
      Pax: 4,
      Hole: 18,
      Name: 'Mock Golfer',
    },
  ],
};

export const mockHistoryResponse = {
  attempts: [
    {
      id: 1,
      created_at: '2026-04-08T10:00:00Z',
      course_id: 'PLC',
      txn_date: '2026-04-08',
      tee_time: '07:30',
      tee_box: '1',
      booking_id: 'MOCK-001',
      status: 'success',
      message: 'Booked (mock)',
    },
  ],
};

export const mockAdminUsers = {
  users: [
    {
      user_name: 'mockuser',
      role: 'NON_ADMIN' as const,
      created_at: '2026-01-01T00:00:00Z',
    },
    {
      user_name: 'admin',
      role: 'ADMIN' as const,
      created_at: '2026-01-02T00:00:00Z',
    },
  ],
};
