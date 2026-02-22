export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';

export const API_ENDPOINTS = {
  login: '/api/v1/auth/login',
  logout: '/api/v1/auth/logout',
  me: '/api/v1/auth/me',
  slots: '/api/v1/slots',
  booking: '/api/v1/booking',
  bookingCheckStatus: '/api/v1/booking/check-status',
  bookingBook: '/api/v1/booking/book',
  bookingAuto: '/api/v1/booking/auto',
} as const;
