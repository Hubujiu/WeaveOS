import {fixtureRequestURL,q36FixtureEnvelope} from './personnel-query-fixtures';
import { test, expect, type Page } from '@playwright/test';

async function tableFixture(page: Page, withMember = false) {
  const user = { id: '00000000-0000-4000-8000-000000000071', account: 'synthetic-footer' };
  const member = { ...user, status: 'active', bootstrapAdmin: true, version: 0, departmentIds: [], identityIds: [], departments: [], identities: [], permissions: [] };
  await page.route('**/api/v1/**', async route => {
    const path = fixtureRequestURL(route.request()).pathname.replace('/api/v1/', '');
    const items = withMember && path === 'personnel/members' ? [member] : [];
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : path === 'personnel/departments' || path === 'personnel/permissions' ? { items: [] }
      : { items, total: items.length, page: 1, pageSize: 20 };
    await route.fulfill({ json: q36FixtureEnvelope(data,route.request()) });
  });
}

// Q35 identity uses the same existing definition cards/footer as templates.
// Table-specific Q34 16px inset is superseded; business paging stays accessible.
test('Q35 identity pagination remains accessible in the shared definition list', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await tableFixture(page);
  await page.goto('/app/admin');
  await page.getByRole('tab', { name: '身份', exact: true }).click();
  const footer = page.locator('.definition-list .table-footer');
  await expect(footer).toHaveCSS('padding-left', '0px');
  await expect(footer).toHaveCSS('padding-right', '0px');
  await expect(footer).toHaveCSS('height', '48px');
  await expect(footer.getByRole('button', { name: '上一页', exact: true })).toBeDisabled();
  await expect(footer.getByRole('button', { name: '下一页', exact: true })).toBeDisabled();
});

test('Q34 member status retains its meaning with the confirmed body typography', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await tableFixture(page, true);
  await page.goto('/app/admin');
  const status = page.locator('.personnel-data-table .member-name small');
  await expect(status).toHaveText('正常');
  await expect(status).toHaveCSS('font-size', '14px');
  await expect(status).toHaveCSS('line-height', '20px');
});
