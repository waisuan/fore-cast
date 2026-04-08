import type { Page, Route } from '@playwright/test';

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
            user: { username: 'e2euser', role: 'NON_ADMIN' },
          }),
        );
      }
      return route.fulfill({ status: 401, body: '{}' });
    }

    if (path.endsWith('/auth/login') && method === 'POST') {
      state.authenticated = true;
      return route.fulfill(
        json({
          user: { username: 'e2euser', role: 'NON_ADMIN' },
        }),
      );
    }

    if (path.endsWith('/auth/logout') && method === 'POST') {
      state.authenticated = false;
      return route.fulfill(json({}));
    }

    if (path.endsWith('/preset') && method === 'GET') {
      return route.fulfill(
        json({
          last_run_status: 'idle',
          enabled: false,
          last_run_message: '',
          last_run_at: null,
        }),
      );
    }

    if (path.includes('/slots') && method === 'GET') {
      return route.fulfill(
        json({
          course: 'PLC',
          slots: [
            {
              TeeTime: '1899-12-30T07:30:00',
              Session: 'Morning',
              TeeBox: '1',
              CourseID: 'PLC',
              CourseName: 'PLC',
            },
          ],
        }),
      );
    }

    return route.fulfill({
      status: 501,
      body: JSON.stringify({ message: `E2E mock: unhandled ${method} ${path}` }),
    });
  });
}
