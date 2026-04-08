import { test, expect } from '@playwright/test';
import { installApiMocks } from './mock-api';

test.describe('auth', () => {
  test('signs in and shows home navigation (mock API)', async ({ page }) => {
    await installApiMocks(page, { startAuthenticated: false });
    await page.goto('/');

    await page.getByLabel(/club member id/i).fill('e2euser');
    await page.getByLabel(/^password$/i).fill('e2epass');
    await page.getByRole('button', { name: /sign in/i }).click();

    await expect(page.getByRole('link', { name: /view slots/i })).toBeVisible({
      timeout: 15_000,
    });
  });
});
