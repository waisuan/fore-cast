import { test, expect } from '@playwright/test';
import { addCalendarDaysYmd, todayIsoMalaysia } from '../src/utils/date';
import { installApiMocks } from './mock-api';

test.describe('slots', () => {
  test('loads mocked slots after picking a date', async ({ page }) => {
    await installApiMocks(page, { startAuthenticated: true });
    await page.goto('/slots');

    const tomorrow = addCalendarDaysYmd(todayIsoMalaysia(), 1);
    const [y, m, d] = tomorrow.split('-').map(Number);
    const tomorrowDate = new Date(y, m - 1, d);
    const dayLabel = tomorrowDate.getDate().toString();

    // Open the DatePicker calendar via the calendar icon button
    await page.locator('[aria-label="Open calendar"]').click();

    // If tomorrow is in the next month, click the "next month" nav button
    const todayParts = todayIsoMalaysia().split('-').map(Number);
    if (m !== todayParts[1] || y !== todayParts[0]) {
      await page.locator('button[name="next_month"]').click();
    }

    // Click the day in the calendar grid
    await page
      .locator('.rdp-day_button')
      .filter({ hasText: new RegExp(`^${dayLabel}$`) })
      .first()
      .click();

    await page.getByRole('button', { name: /load slots/i }).click();

    await expect(page.getByText(/\d+ slot\(s\)/)).toBeVisible({ timeout: 15_000 });
  });
});
