export const API_BASE_URL =
  typeof window !== 'undefined' ? '' : process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080';

export const API_ENDPOINTS = {
  login: '/api/v1/auth/login',
  logout: '/api/v1/auth/logout',
  me: '/api/v1/auth/me',
  slots: '/api/v1/slots',
  booking: '/api/v1/booking',
  bookingCheckStatus: '/api/v1/booking/check-status',
  bookingBook: '/api/v1/booking/book',
  bookingCancel: '/api/v1/booking/cancel',
  history: '/api/v1/history',
  preset: '/api/v1/preset',
  presetCancel: '/api/v1/preset/cancel',
  presetSkipNext: '/api/v1/preset/skip-next',
  adminRegister: '/api/v1/admin/register',
  adminUsers: '/api/v1/admin/users',
} as const;

/** Path for DELETE /api/v1/admin/users/{username} */
export function adminDeleteUserPath(username: string) {
  return `/api/v1/admin/users/${encodeURIComponent(username)}`;
}

/** Path for PUT /api/v1/admin/users/{username}/role */
export function adminUserRolePath(username: string) {
  return `/api/v1/admin/users/${encodeURIComponent(username)}/role`;
}

/** Path for DELETE /api/v1/admin/presets/{username} */
export function adminDeletePresetPath(username: string) {
  return `/api/v1/admin/presets/${encodeURIComponent(username)}`;
}
