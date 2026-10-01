import { test, expect } from '@playwright/test';

// Independent visual oracle: synced Figma Q34 identity footer 279:1894 has
// a 48px outer height and 16px horizontal inset. API fixtures are synthetic.
test('Q34 identity pagination preserves the confirmed footer inset', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  const user = { id: '00000000-0000-4000-8000-000000000071', account: 'synthetic-footer' };
  await page.route('**/api/v1/**', async route => {
    const path = new URL(route.request().url()).pathname.replace('/api/v1/', '');
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : path === 'personnel/departments' || path === 'personnel/permissions' ? { items: [] }
      : { items: [], total: 0, page: 1, pageSize: 20 };
    await route.fulfill({ json: { code: 'OK', message: 'success', data, meta: null } });
  });
  await page.goto('/app/admin');
  await page.getByRole('tab', { name: '身份', exact: true }).click();
  const footer = page.locator('.identity-table .table-footer');
  await expect(footer).toHaveCSS('padding-left', '16px');
  await expect(footer).toHaveCSS('padding-right', '16px');
  await expect(footer).toHaveCSS('height', '48px');
  await expect(footer.getByRole('button', { name: '上一页', exact: true })).toBeDisabled();
  await expect(footer.getByRole('button', { name: '下一页', exact: true })).toBeDisabled();
});
