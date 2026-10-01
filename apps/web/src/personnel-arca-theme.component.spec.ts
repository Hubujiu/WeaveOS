import {fixtureRequestURL,q36FixtureEnvelope} from './personnel-query-fixtures';
import { expect, test } from '@playwright/test';

// Fixed official CSS uses @custom-variant dark (&:is(.dark *)); the approved
// light workspace must not switch individual source buttons for OS preference.
test('Q35 original dark variant leaves the light pager unchanged without a dark class', async ({ page }) => {
  const user = { id: 'synthetic-theme-user', account: 'synthetic-theme' };
  await page.route('**/api/v1/**', async route => {
    const path = fixtureRequestURL(route.request()).pathname.replace('/api/v1/', '');
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : { items: [], total: path === 'personnel/members' ? 125 : 0, page: 1, pageSize: 20 };
    await route.fulfill({ json: q36FixtureEnvelope(data,route.request()) });
  });
  await page.emulateMedia({ colorScheme: 'light' }); await page.goto('/app/admin');
  const current = page.getByRole('button', { name: 'Page 1', exact: true });
  await expect(current).toHaveCSS('background-color', 'oklch(1 0 0)');
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(page.locator('.dark')).toHaveCount(0);
  await expect(current).toHaveCSS('background-color', 'oklch(1 0 0)');
});
