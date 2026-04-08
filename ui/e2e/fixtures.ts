/** JSON fixtures for Playwright `mock-api.ts` only — not used by the production app. */

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
