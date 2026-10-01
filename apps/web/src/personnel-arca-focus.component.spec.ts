import { expect, test } from '@playwright/test';

// Independent oracle: official c0319d8 base outline-ring/50 and source trigger
// classes; existing workspace focus is scoped away from imported source UI.
test('Q35 original column focus excludes the workspace outline while retaining outer focus', async ({ page, browserName }) => {
  const user = { id: 'synthetic-focus-user', account: 'synthetic-focus' };
  await page.route('**/api/v1/**', async route => {
    const path = new URL(route.request().url()).pathname.replace('/api/v1/', '');
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : { items: [], total: 0, page: 1, pageSize: 20 };
    await route.fulfill({ json: { code: 'OK', message: 'success', data, meta: null } });
  });
  await page.goto('/app/admin');
  await page.keyboard.press('Tab');
  for (const name of ['筛选 成员', '拖动以调整 部门 列顺序']) {
    const control = page.getByRole('button', { name, exact: true });
    await control.focus();
    expect(await control.evaluate(n => n.matches(':focus-visible'))).toBe(true);
    // Official Firefox uses the original preflight :-moz-focusring outline:auto
    // shorthand, which resolves to each original class's currentColor.
    const color = browserName === 'firefox'
      ? name === '筛选 成员' ? 'oklab(0.556 0 0 / 0.7)' : 'oklab(0.556 0 0 / 0.8)'
      : 'oklab(0.708 0 0 / 0.5)';
    await expect(control).toHaveCSS('outline-color', color);
    await expect(control).toHaveCSS('outline-offset', '0px');
  }
  const outer = page.getByRole('button', { name: '退出', exact: true });
  await outer.focus();
  await expect(outer).toHaveCSS('outline-style', 'solid');
  await expect(outer).toHaveCSS('outline-width', '2px');
  await expect(outer).toHaveCSS('outline-offset', '3px');
});
