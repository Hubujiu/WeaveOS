import {fixtureRequestURL,q36FixtureEnvelope} from './personnel-query-fixtures';
import { test, expect, type Page } from '@playwright/test';

// Independent oracle: user Q35, personnel R3 §5.8, Q25 pagination DTOs,
// official Arca c0319d8 browser measurements and source. All accounts synthetic.
const user = { id: '00000000-0000-4000-8000-000000000071', account: 'synthetic-arca' };
const definition = { id: '00000000-0000-4000-8000-000000000072', name: '共享身份', description: '真实说明', version: 1, templateIds: [], permissionCodes: [], affectedMembers: 1, affectedIdentities: 1 };
const row = (i: number) => ({ ...user, id: 'synthetic-member-' + i, account: ['Zulu', 'Alpha', 'Bravo'][i % 3] + i, status: 'active', bootstrapAdmin: false, version: 1, departmentIds: [], identityIds: [], departments: [], identities: [], permissions: [] });
async function fixture(page: Page, count = 1, total = count) {
  await page.route('**/api/v1/**', async route => {
    const url = fixtureRequestURL(route.request()), path = url.pathname.replace('/api/v1/', '');
    const p = Number(url.searchParams.get('page') || 1), size = Number(url.searchParams.get('pageSize') || 20);
    const members = Array.from({ length: Math.min(count, size) }, (_, i) => row((p - 1) * size + i));
    const events = members.map(m => ({ id: m.id, occurredAt: '2026-10-01T02:00:00Z', actorAccount: m.account, action: 'IDENTITY_UPDATED', objectType: 'identity', objectId: definition.id, outcome: 'success', summary: { before: { name: '旧名称' }, after: { name: definition.name } } }));
    const data = path === 'sessions/current' ? user : path === 'me/access'
      ? { user, bootstrapAdmin: true, personnelManage: true, identities: [], permissions: [], applications: [] }
      : path === 'personnel/departments' || path === 'personnel/permissions' ? { items: [] }
      : path === 'personnel/identities' || path === 'personnel/templates' ? { items: [definition, { ...definition, id: definition.id + '-second', name: '另一配置' }], total: 2, page: p, pageSize: size }
      : { items: path === 'personnel/events' ? events : members, total, page: p, pageSize: size };
    await route.fulfill({ json: q36FixtureEnvelope(data,route.request()) });
  });
  await page.goto('/app/admin');
}

test('Q35 identity and template share the card list and retain dirty edit protection', async ({ page }) => {
  await fixture(page);
  await page.getByRole('tab', { name: '权限模板', exact: true }).click();
  const templateStyle = await page.locator('.definition-item').first().evaluate(n => {
    const s = getComputedStyle(n); return [s.padding, s.borderRadius, s.minHeight, s.display];
  });
  await page.getByRole('tab', { name: '身份', exact: true }).click();
  await expect(page.locator('.definition-list table')).toHaveCount(0);
  const item = page.locator('.definition-item').first();
  await expect(item).toContainText(definition.description);
  expect(await item.evaluate(n => { const s = getComputedStyle(n); return [s.padding, s.borderRadius, s.minHeight, s.display]; })).toEqual(templateStyle);
  await item.focus(); await page.keyboard.press('Enter');
  await page.getByLabel('说明', { exact: true }).fill('尚未保存');
  await page.getByRole('button', { name: '另一配置', exact: true }).click();
  await expect(page.getByRole('dialog', { name: '有未保存的修改', exact: true })).toBeVisible();
  await page.getByRole('button', { name: '继续编辑', exact: true }).click();
  await expect(page.getByLabel('说明', { exact: true })).toHaveValue('尚未保存');
  await expect(item).toHaveClass(/selected/);
});

for (const tab of ['成员与部门', '操作记录']) for (const count of [0, 1]) test(`Q35 ${tab} ${count} real rows fill the body with inert blank grid rows`, async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 }); await fixture(page, count);
  if (tab !== '成员与部门') await page.getByRole('tab', { name: tab, exact: true }).click();
  const area = page.locator(tab === '成员与部门' ? '.member-table' : '.activity-table');
  const scroll = area.locator('.el-scrollbar__wrap'), blanks = area.locator('tr[data-empty-row]');
  await expect(blanks.first()).toBeAttached();
  await expect(scroll).toHaveCSS('max-height', 'none');
  const verify = async () => {
    const metrics = await area.evaluate(n => {
      const viewport = n.querySelector('.el-scrollbar__wrap')!, last = n.querySelector('tbody tr:last-child')!, footer = n.querySelector('.arca-pagination')!;
      return { bodyBottom: viewport.getBoundingClientRect().bottom, lastBottom: last.getBoundingClientRect().bottom, footerTop: footer.getBoundingClientRect().top, footerHeight: footer.getBoundingClientRect().height };
    });
    expect(Math.abs(metrics.lastBottom - metrics.bodyBottom)).toBeLessThanOrEqual(1);
    expect(Math.abs(metrics.footerTop - metrics.bodyBottom)).toBeLessThanOrEqual(1);
    expect(metrics.footerHeight).toBeCloseTo(33, 2);
    expect(await blanks.evaluateAll(nodes => nodes.every(n => n.getAttribute('aria-hidden') === 'true' && !n.querySelector('button,input,a,[tabindex]') && !n.textContent?.trim()))).toBe(true);
    await expect(area.getByRole('checkbox')).toHaveCount(tab === '成员与部门' ? count + 1 : 0);
  };
  await verify();
  if (!count) await expect(area).toContainText(tab === '成员与部门' ? '暂无成员' : '暂无操作记录');
  const before = await blanks.count(); await page.setViewportSize({ width: 1920, height: 800 });
  await expect.poll(() => blanks.count()).toBeLessThan(before); await verify();
});

test('Q35 full original pager and page size control are present', async ({ page }) => {
  await fixture(page, 1, 125);
  const area = page.locator('.member-table');
  const footer = area.locator('.arca-pagination, .table-footer');
  await expect.soft(footer).toHaveCSS('height', '33px');
  await expect.soft(area.getByRole('combobox', { name: '每页条数', exact: true })).toHaveCount(1);
  await expect.soft(area.getByRole('button', { name: 'Page 2', exact: true })).toHaveCount(1);
});

test('Q35 original resize and reorder handles are enabled', async ({ page }) => {
  await fixture(page);
  const area = page.locator('.member-table');
  await expect.soft(area.getByRole('button', { name: 'Resize departments column', exact: true })).toHaveCount(1);
  await expect.soft(area.getByRole('button', { name: '拖动以调整 departments 列顺序', exact: true })).toHaveCount(1);
});

test('Q35 official animated checkbox and full pager use real server page sizes without double slicing', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 }); await fixture(page, 20, 125);
  const table = page.locator('.member-table'), all = table.getByRole('checkbox', { name: '选择当前页成员', exact: true });
  await expect(all).toHaveJSProperty('tagName', 'BUTTON');
  await expect(all.locator('svg')).toHaveCount(0);
  await table.getByRole('checkbox', { name: '选择成员：Zulu0', exact: true }).check();
  await expect(all).toHaveAttribute('aria-checked', 'mixed');
  await expect(table.getByRole('checkbox', { name: '选择成员：Zulu0', exact: true }).locator('svg')).toHaveCSS('width', '12px');
  await expect(table.locator('.arca-pagination')).toHaveCSS('height', '33px');
  const size = table.getByRole('combobox', { name: '每页条数', exact: true });
  await expect(size).toHaveCSS('height', '24px'); await expect(size).toHaveCSS('width', '64px');
  await size.click(); await expect(page.getByRole('option')).toHaveText(['5', '10', '20', '25', '50', '100']);
  const request = page.waitForRequest(r => r.url().includes('personnel/members/search') && fixtureRequestURL(r).searchParams.get('pageSize') === '5');
  await page.getByRole('option', { name: '5', exact: true }).click(); expect(fixtureRequestURL(await request).searchParams.get('page')).toBe('1');
  await expect(table.locator('tbody tr[data-row-id]')).toHaveCount(5);
  await table.getByRole('button', { name: '下一页', exact: true }).click();
  await expect(table).toContainText('Bravo5'); await expect(table.locator('tbody tr[data-row-id]')).toHaveCount(5);
  await expect(table.getByRole('checkbox', { checked: true })).toHaveCount(0);
  await all.check(); await expect(table.getByRole('checkbox', { checked: true })).toHaveCount(6);
  await expect(table.locator('tr[data-empty-row] button')).toHaveCount(0);
});

test('Q36 typed member headers preserve received order, resize and reorder without business writes', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 }); await fixture(page, 3);
  const table = page.locator('.member-table'); let writes = 0;
  page.on('request', r => { if (r.url().includes('/api/') && r.method() !== 'GET' && !r.url().endsWith('/search')) writes++; });
  // Q36 approved PLAN §1 supersedes member text sorting.
  await expect(table.getByRole('button', {name:'筛选 成员',exact:true})).toHaveCount(0);
  await expect(table.locator('tbody tr[data-row-id] .member-name strong')).toHaveText(['Zulu0','Alpha1','Bravo2']);
  await expect(table.locator('th[aria-sort]')).toHaveCount(0);
  const resize = table.getByRole('button', { name: 'Resize departments column', exact: true }), box = (await resize.boundingBox())!;
  const department = table.getByRole('columnheader').filter({ hasText: '部门' }), oldWidth = (await department.boundingBox())!.width;
  await page.mouse.move(box.x + box.width / 2, box.y + 20); await page.mouse.down(); await page.mouse.move(box.x + box.width / 2 + 60, box.y + 20, { steps: 5 }); await page.mouse.up();
  expect((await department.boundingBox())!.width).toBeGreaterThan(oldWidth + 40);
  const grip = table.getByRole('button', { name: '拖动以调整 departments 列顺序', exact: true }), start = (await grip.boundingBox())!;
  const target = (await table.getByRole('columnheader').filter({ hasText: '身份' }).boundingBox())!;
  await page.mouse.move(start.x + 10, start.y + 20); await page.mouse.down(); await page.mouse.move(target.x + target.width - 8, start.y + 20, { steps: 8 }); await page.mouse.up();
  await expect(table.getByRole('columnheader')).toHaveText(['', '成员', '身份', '部门', '人员管理', '操作']);
  expect(writes).toBe(0);
});

test('Q35 page size retains arrow, Home and Enter keyboard operation', async ({ page }) => {
  await fixture(page, 20, 125);
  const trigger = page.getByRole('combobox', { name: '每页条数', exact: true });
  await trigger.focus(); await page.keyboard.press('ArrowUp');
  await expect(trigger).toHaveAttribute('aria-expanded', 'true');
  await expect(page.getByRole('option', { name: '20', exact: true })).toBeFocused();
  await page.keyboard.press('Home'); await expect(page.getByRole('option', { name: '5', exact: true })).toBeFocused();
  const request = page.waitForRequest(r => r.url().includes('personnel/members/search') && fixtureRequestURL(r).searchParams.get('pageSize') === '5');
  await page.keyboard.press('Enter'); await request;
  await expect(trigger).toHaveAttribute('aria-expanded', 'false'); await expect(trigger).toBeFocused();
});

test('Q36 shrinking full result blocks pagination until explicit refresh instead of silently clamping', async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await fixture(page, 20, 21);
  let recoveryRequests = 0;
  await page.route('**/api/v1/personnel/members/search', async route => {
    const p = Number(fixtureRequestURL(route.request()).searchParams.get('page') || 1);
    if (p === 1) recoveryRequests++;
    // Q36 PLAN + query-drafts: 21 → 20 invalidates the old result context.
    if(route.request().postDataJSON().queryVersion)return route.fulfill({status:409,json:{code:'COMMON_QUERY_CHANGED',message:'changed',data:null,meta:null}});
    await route.fulfill({ json: q36FixtureEnvelope({ items: Array.from({ length: 20 }, (_, i) => row(i)), total: 20, page: p, pageSize: 20 },route.request()) });
  });
  await page.getByRole('button', { name: '下一页', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('查询结果已变化');
  expect(recoveryRequests).toBe(0);
  await expect(page.getByLabel('跳至页',{exact:true})).toHaveValue('1');
  await page.getByRole('button',{name:'刷新查询',exact:true}).click();
  await expect.poll(() => recoveryRequests).toBe(1);
  await expect(page.locator('.member-table tbody tr[data-row-id]')).toHaveCount(20);
  await expect(page.locator('.member-table')).toContainText('Zulu0');
});

test('Q35 narrow viewport keeps the original pager controls fully reachable', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 844 }); await fixture(page, 1, 125);
  const area = page.locator('.member-table');
  for (const control of [area.getByRole('combobox', { name: '每页条数', exact: true }), area.getByRole('button', { name: '下一页', exact: true })]) {
    await control.scrollIntoViewIfNeeded();
    const bounds = (await area.boundingBox())!, box = (await control.boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(bounds.x);
    expect(box.x + box.width).toBeLessThanOrEqual(bounds.x + bounds.width);
  }
  await area.getByRole('combobox', { name: '每页条数', exact: true }).click();
  await page.getByRole('option', { name: '5', exact: true }).click();
  await expect(area.getByRole('combobox', { name: '每页条数', exact: true })).toHaveText('5');
});

test('Q35 source font uses Geist with the existing Chinese fallback inside its scope', async ({ page }) => {
  await fixture(page);
  const family = await page.locator('.member-table table').evaluate(n => getComputedStyle(n).fontFamily);
  expect(family).toContain('Geist Variable'); expect(family).toContain('Noto Sans SC');
  const outer = await page.locator('.personnel-page-heading').evaluate(n => getComputedStyle(n).fontFamily);
  expect(outer).toContain('Noto Sans SC'); expect(outer).not.toContain('Geist');
});

test('Q35 sorting menu stays open during its own scrolling and closes on table scrolling', async ({ page }) => {
  await page.setViewportSize({width:900,height:844});
  await fixture(page, 3);
  // Q36: the server time sort is the only eligible column.
  await page.getByRole('tab',{name:'操作记录',exact:true}).click();
  const table = page.locator('.activity-table');
  const trigger = table.getByRole('button', { name: '排序 occurredAt', exact: true });
  await trigger.click();
  const menu = page.getByRole('menu');
  await expect(menu).toBeVisible();
  await menu.dispatchEvent('scroll');
  await expect(trigger).toHaveAttribute('aria-expanded', 'true');
  await menu.getByRole('menuitem', { name: '升序', exact: true }).click();
  await expect(table.getByRole('columnheader').filter({hasText:'时间'})).toHaveAttribute('aria-sort','ascending');
  await trigger.click();
  await expect(trigger).toHaveAttribute('aria-expanded', 'true');
  await table.locator('.el-scrollbar__wrap').evaluate(n=>{n.scrollLeft+=20;});
  await expect(trigger).toHaveAttribute('aria-expanded', 'false');
});
