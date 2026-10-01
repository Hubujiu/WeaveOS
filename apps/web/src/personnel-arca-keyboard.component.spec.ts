import { expect, test } from '@playwright/test';

// Independent oracle: confirmed R3 §5.8 keyboard and visible focus support.
// This fixture isolates rendered UI; it does not claim real backend acceptance.
test('Q35 sort menu supports direction keys, visible focus and Escape return', async ({ page }) => {
  const user = { id: 'synthetic-keyboard-user', account: 'synthetic-keyboard' };
  await page.route('**/api/v1/**', async route => {
    const path = new URL(route.request().url()).pathname.replace('/api/v1/', '');
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : { items: [], total: 0, page: 1, pageSize: 20 };
    await route.fulfill({ json: { code: 'OK', message: 'success', data, meta: null } });
  });
  await page.goto('/app/admin');
  const trigger = page.getByRole('button', { name: '筛选 成员', exact: true });
  await trigger.focus(); await trigger.press('ArrowDown');
  const ascending = page.getByRole('menuitem', { name: '升序', exact: true });
  const descending = page.getByRole('menuitem', { name: '降序', exact: true });
  await expect(trigger).toHaveAttribute('aria-expanded', 'true');
  await expect(ascending).toBeFocused();
  await expect(ascending).toHaveCSS('outline-style', 'solid');
  await expect(ascending).toHaveCSS('outline-width', '2px');
  await page.keyboard.press('ArrowDown'); await expect(descending).toBeFocused();
  await page.keyboard.press('Home'); await expect(ascending).toBeFocused();
  await page.keyboard.press('End'); await expect(descending).toBeFocused();
  await page.keyboard.press('Escape');
  await expect(trigger).toHaveAttribute('aria-expanded', 'false'); await expect(trigger).toBeFocused();
  await trigger.press('ArrowUp'); await expect(descending).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(trigger).toHaveAttribute('aria-expanded', 'false'); await expect(trigger).toBeFocused();
  await expect(page.getByRole('columnheader').filter({ hasText: '成员' })).toHaveAttribute('aria-sort', 'descending');
});
