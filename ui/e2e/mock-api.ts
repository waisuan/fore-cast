import type { Page, Route } from '@playwright/test';
import { mockPresetFull, mockSlotsResponse, mockUser } from './fixtures';

const json = (data: unknown) => ({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify(data),
});

/**
 * Intercepts same-origin `/api/v1/*` calls so tests do not need a Go backend.
 * Register before navigation.
 */
export async function installApiMocks(
  page: Page,
  opts: { startAuthenticated?: boolean } = {},
) {
  const state = { authenticated: opts.startAuthenticated ?? false };

  await page.route('**/api/v1/**', async (route: Route) => {
    const req = route.request();
    const url = new URL(req.url());
    const path = url.pathname;
    const method = req.method();

    if (path.endsWith('/auth/me') && method === 'GET') {
      if (state.authenticated) {
        return route.fulfill(
          json({
            user: mockUser('NON_ADMIN', 'e2euser'),
          }),
        );
      }
      return route.fulfill({ status: 401, body: '{}' });
    }

    if (path.endsWith('/auth/login') && method === 'POST') {
      state.authenticated = true;
      return route.fulfill(
        json({
          user: mockUser('NON_ADMIN', 'e2euser'),
        }),
      );
    }

    if (path.endsWith('/auth/logout') && method === 'POST') {
      state.authenticated = false;
      return route.fulfill(json({}));
    }

    if (path.endsWith('/preset') && method === 'GET') {
      return route.fulfill(json(mockPresetFull()));
    }

    if (path.includes('/slots') && method === 'GET') {
      return route.fulfill(json(mockSlotsResponse));
    }

    return route.fulfill({
      status: 501,
      body: JSON.stringify({ message: `E2E mock: unhandled ${method} ${path}` }),
    });
  });
}
