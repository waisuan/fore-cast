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
    booking_open: '21:59',
    ntfy_topic: '',
    enable_notifications: false,
    enabled: false,
    defaults: {
      course: 'PLC',
      cutoff: '08:15',
      retry_interval: '1s',
      min_retry_interval: '0s',
      timeout: '10m',
      min_timeout: '30s',
      booking_open: '21:59',
    },
    last_run_status: 'idle',
    last_run_message: '',
    last_run_at: null,
    override_course: '',
    override_until: null as string | null,
    skip_next_run: false,
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
      steps: [
        { at: '2026-04-08T14:00:00Z', text: 'PLC on 2026/04/15: 12 tee time(s) before your 8:15 AM cutoff.' },
        { at: '2026-04-08T14:00:01Z', text: '7:30 AM tee 1: open.' },
        { at: '2026-04-08T14:00:01Z', text: '7:30 AM tee 1: waiting 3s before booking (the club requires it).' },
        { at: '2026-04-08T14:00:04Z', text: '7:30 AM tee 1: booked. Booking ID MOCK-001.' },
        { at: '2026-04-08T14:00:04Z', text: 'Done: booked.' },
      ],
    },
    {
      id: 2,
      created_at: '2026-04-07T14:00:33Z',
      course_id: 'BRC',
      txn_date: '2026-04-14',
      tee_time: '',
      tee_box: '',
      booking_id: '',
      status: 'failed',
      message: '[BRC] no slot booked — last club reply: Rapid attempts detected. Please wait a moment before trying again.',
      steps: [
        { at: '2026-04-07T14:00:00Z', text: 'BRC on 2026/04/14: 8 tee time(s) before your 8:15 AM cutoff.' },
        { at: '2026-04-07T14:00:01Z', text: '7:37 AM tee 1: This Flight Is Blocked' },
        { at: '2026-04-07T14:00:01Z', text: '7:37 AM tee 1: booking failed: Tee Time was not reserve' },
        { at: '2026-04-07T14:00:02Z', text: '7:38 AM tee 10: Rapid attempts detected. Please wait a moment before trying again.' },
        { at: '2026-04-07T14:00:02Z', text: 'Club rate limit hit; waiting 3s before trying again.' },
        { at: '2026-04-07T14:00:33Z', text: 'Done: nothing booked. [BRC] no slot booked — last club reply: Rapid attempts detected. Please wait a moment before trying again.' },
      ],
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
