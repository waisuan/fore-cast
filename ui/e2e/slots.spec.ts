import { test, expect } from '@playwright/test';
import { installApiMocks } from './mock-api';

test.describe('slots', () => {
  test('loads mocked slots after picking a date', async ({ page }) => {
    await installApiMocks(page, { startAuthenticated: true });
    await page.goto('/slots');

    const tomorrow = new Date();
    tomorrow.setUTCDate(tomorrow.getUTCDate() + 1);
    const iso = tomorrow.toISOString().slice(0, 10);

    await page.locator('#date').fill(iso);
    await page.getByRole('button', { name: /load slots/i }).click();

    await expect(page.getByText(/\d+ slot\(s\)/)).toBeVisible({ timeout: 15_000 });
  });
});
