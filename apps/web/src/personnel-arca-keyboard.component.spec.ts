import {fixtureRequestURL,q36FixtureEnvelope} from './personnel-query-fixtures';
import { expect, test } from '@playwright/test';

// Independent oracle: confirmed R3 §5.8 keyboard and visible focus support.
// This fixture isolates rendered UI; it does not claim real backend acceptance.
test('Q35 sort menu supports direction keys, visible focus and Escape return', async ({ page }) => {
  const user = { id: 'synthetic-keyboard-user', account: 'synthetic-keyboard' };
  await page.route('**/api/v1/**', async route => {
    const path = fixtureRequestURL(route.request()).pathname.replace('/api/v1/', '');
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : { items: [], total: 0, page: 1, pageSize: 20 };
    await route.fulfill({ json: q36FixtureEnvelope(data,route.request()) });
  });
  await page.goto('/app/admin');
  // Q36 time column retains the original keyboard menu; text is unsortable.
  await page.getByRole('tab',{name:'操作记录',exact:true}).click();
  const trigger = page.getByRole('button', { name: '排序 occurredAt', exact: true });
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
  // The initial explicit descending sort is already selected. Choosing it again clears custom sorting.
  await page.keyboard.press('Enter');
  await expect(trigger).toHaveAttribute('aria-expanded', 'false'); await expect(trigger).toBeFocused();
  await expect(page.getByRole('columnheader').filter({ hasText: '时间' })).not.toHaveAttribute('aria-sort');
  await trigger.press('Enter'); await expect(ascending).toBeFocused();
  await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
  await trigger.press('Space'); await expect(ascending).toBeFocused();
  await page.keyboard.press('Escape'); await expect(trigger).toBeFocused();
});

test('Q35 original base foreground and smoothing are scoped to the table', async ({ page }) => {
  const user = { id: 'synthetic-font-user', account: 'synthetic-font' };
  await page.route('**/api/v1/**', async route => {
    const path = fixtureRequestURL(route.request()).pathname.replace('/api/v1/', '');
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : { items: [], total: path === 'personnel/members' ? 125 : 0, page: 1, pageSize: 20 };
    await route.fulfill({ json: q36FixtureEnvelope(data,route.request()) });
  });
  await page.goto('/app/admin');
  const table = page.locator('.member-table .personnel-source-table');
  await expect(table).toHaveCSS('color', 'oklch(0.145 0 0)');
  await expect(page.getByRole('button', { name: 'Page 2', exact: true })).toHaveCSS('color', 'oklch(0.145 0 0)');
  if (await page.evaluate(() => CSS.supports('-webkit-font-smoothing', 'antialiased'))) {
    expect(await table.evaluate(n => getComputedStyle(n).getPropertyValue('-webkit-font-smoothing'))).toBe('antialiased');
  }
  await expect(page.locator('.personnel-page-heading')).toHaveCSS('color', 'rgb(37, 37, 37)');
});
