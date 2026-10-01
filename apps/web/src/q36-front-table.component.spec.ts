import { test, expect } from '@playwright/test';

test.beforeEach(async ({page}) => { await page.goto('/src/q36-front-fixture.html'); });
test('Q36 Table directly controls 1-based page, arbitrary jump and size reset', async ({page}) => {
  await expect(page.getByRole('button',{name:'Page 1',exact:true})).toHaveAttribute('aria-current','page');
  await page.getByRole('button',{name:'下一页',exact:true}).click();
  await expect(page.getByLabel('当前页')).toHaveText('2');
  await expect(page.getByRole('button',{name:'Page 2',exact:true})).toHaveAttribute('aria-current','page');
  await page.getByRole('spinbutton',{name:'跳至页'}).fill('13');
  await page.getByRole('spinbutton',{name:'跳至页'}).press('Enter');
  await expect(page.getByLabel('当前页')).toHaveText('13');
  await page.getByRole('button',{name:'每页条数'}).click();
  await page.getByRole('option',{name:'50',exact:true}).click();
  await expect(page.getByLabel('当前页')).toHaveText('1');
});
test('Q36 typed text has no sort icon; time/number sorting requests preserve received rows', async ({page}) => {
  const text=page.getByRole('columnheader',{name:/成员/});
  await expect(text.getByRole('button',{name:/排序|筛选/})).toHaveCount(0);
  await page.getByRole('button',{name:'排序 occurredAt',exact:true}).click();
  await page.getByRole('menuitem',{name:'升序',exact:true}).click();
  await expect(page.getByLabel('排序请求')).toHaveText('{"key":"occurredAt","direction":"asc"}');
  await expect(page.locator('tbody tr[data-row-id]').first()).toHaveAttribute('data-row-id','first');
  await page.getByRole('button',{name:'排序 amount',exact:true}).click();
  await page.getByRole('menuitem',{name:'升序',exact:true}).click();
  await expect(page.locator('tbody tr[data-row-id]').first()).toHaveAttribute('data-row-id','first');
});
test('Q36 external column width/order controls survive loading; empty grid remains inert', async ({page}) => {
  await page.getByRole('button',{name:'外部恢复列状态'}).click();
  await expect(page.locator('thead th').nth(1)).toContainText('数字示例');
  await expect(page.locator('colgroup col').nth(1)).toHaveCSS('width','150px');
  await page.getByRole('button',{name:'切换加载'}).click();
  await expect(page.getByRole('table',{name:'受控成员'})).toHaveAttribute('aria-busy','true');
  await page.getByRole('button',{name:'切换加载'}).click();
  await expect(page.locator('thead th').nth(1)).toContainText('数字示例');
  await page.getByRole('button',{name:'切换空表'}).click();
  await expect(page.locator('tr[data-empty-row]')).not.toHaveCount(0);
  await expect(page.locator('tr[data-empty-row] input,tr[data-empty-row] button')).toHaveCount(0);
});
test('Q36 original selection, drag and resize remain interactive',async({page})=>{
  await page.getByRole('checkbox',{name:'选择 Zulu'}).check();
  await expect(page.getByLabel('选择结果')).toHaveText('first');
  const grip=page.getByRole('button',{name:'拖动以调整 amount 列顺序'});
  const from=await grip.boundingBox();const target=await page.getByRole('columnheader',{name:/成员/}).boundingBox();
  if(!from||!target)throw new Error('visible drag targets required');
  await page.mouse.move(from.x+from.width/2,from.y+from.height/2);await page.mouse.down();
  await page.mouse.move(target.x+4,target.y+20,{steps:8});await page.mouse.up();
  await expect(page.locator('thead th').nth(1)).toContainText('数字示例');
  const resize=await page.getByRole('button',{name:'Resize amount column'}).boundingBox();
  const before=await page.locator('colgroup col').nth(1).evaluate(n=>n.getBoundingClientRect().width);
  if(!resize)throw new Error('visible resize target required');
  await page.mouse.move(resize.x+2,resize.y+20);await page.mouse.down();await page.mouse.move(resize.x+42,resize.y+20,{steps:6});await page.mouse.up();
  expect(await page.locator('colgroup col').nth(1).evaluate(n=>n.getBoundingClientRect().width)).toBeGreaterThan(before+20);
});
