import {test,expect} from '@playwright/test';
test.beforeEach(async({page})=>{await page.goto('/src/q36-front-fixture.html');});
test('Q36 filter opens beside search, returns focus on Esc and applies no filter for empty root',async({page})=>{
  const trigger=page.getByRole('button',{name:/自定义筛选/});await trigger.focus();await page.keyboard.press('Enter');
  const panel=page.getByRole('dialog',{name:'自定义筛选'});await expect(panel).toBeVisible();
  await expect(panel.getByRole('combobox',{name:'组 1 匹配方式'})).toBeFocused();
  await page.keyboard.press('Escape');await expect(panel).not.toBeVisible();await expect(trigger).toBeFocused();
  await trigger.click();await page.getByRole('button',{name:'应用筛选',exact:true}).click();
  await expect(page.getByLabel('筛选提交')).toHaveText('无筛选');
});
test('Q36 builds nested AND/OR with exact case-sensitive text and relation NOT EXISTS DTO',async({page})=>{
  await page.getByRole('button',{name:/自定义筛选/}).click();
  await page.getByRole('button',{name:'组 1 添加条件',exact:true}).click();
  await page.getByLabel('条件 1 字段', {exact:true}).selectOption('account');
  await page.getByLabel('条件 1 值',{exact:true}).fill('Alice');
  await page.getByRole('button',{name:'组 1 添加分组',exact:true}).click();
  await page.getByLabel('组 1.2 匹配方式',{exact:true}).selectOption('or');
  await page.getByRole('button',{name:'组 1.2 添加条件',exact:true}).click();
  await page.getByLabel('条件 2.1 字段',{exact:true}).selectOption('identityIds');
  await page.getByLabel('条件 2.1 比较',{exact:true}).selectOption('neq');
  await page.getByLabel('条件 2.1 值',{exact:true}).selectOption('00000000-0000-4000-8000-000000000001');
  await page.getByRole('button',{name:'应用筛选',exact:true}).click();
  await expect(page.getByLabel('筛选提交')).toHaveText('{"operator":"and","children":[{"field":"account","operator":"eq","value":"Alice"},{"operator":"or","children":[{"field":"identityIds","operator":"neq","value":"00000000-0000-4000-8000-000000000001"}]}]}');
});
test('Q36 empty nested group is blocked visibly and removable without discarding other input',async({page})=>{
  await page.getByRole('button',{name:'载入空子组'}).click();await page.getByRole('button',{name:/自定义筛选/}).click();
  await expect(page.getByRole('alert')).toContainText('分组不能为空');
  await expect(page.getByRole('button',{name:'应用筛选',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'删除组 1.1',exact:true}).click();
  await page.getByRole('button',{name:'应用筛选',exact:true}).click();await expect(page.getByLabel('筛选提交')).toHaveText('无筛选');
});
test('Q36 global 20-leaf limit prevents adding leaves; removing one restores capacity',async({page})=>{
  await page.getByRole('button',{name:'载入20条件'}).click();await page.getByRole('button',{name:/自定义筛选/}).click();
  await expect(page.getByRole('button',{name:'组 1 添加条件',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'删除条件 1',exact:true}).click();
  await expect(page.getByRole('button',{name:'组 1 添加条件',exact:true})).toBeEnabled();
});
test('Q36 groups stop at third level; time accepts explicit zone and only text eq/neq',async({page})=>{
  await page.getByRole('button',{name:'切换记录筛选'}).click();await page.getByRole('button',{name:/自定义筛选/}).click();
  await page.getByRole('button',{name:'组 1 添加分组',exact:true}).click();
  await page.getByRole('button',{name:'组 1.1 添加分组',exact:true}).click();
  await expect(page.getByRole('button',{name:'组 1.1.1 添加分组',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'删除组 1.1',exact:true}).click();
  await page.getByRole('button',{name:'组 1 添加条件',exact:true}).click();
  await expect(page.getByLabel('条件 1 比较').locator('option')).toHaveCount(2);
  await page.getByLabel('条件 1 字段',{exact:true}).selectOption('occurredAt');
  await page.getByLabel('条件 1 比较',{exact:true}).selectOption('gte');
  await page.getByLabel('条件 1 值',{exact:true}).fill('2026-10-01T10:00:00.123456+08:00');
  await page.getByRole('button',{name:'应用筛选',exact:true}).click();
  await expect(page.getByLabel('筛选提交')).toHaveText('{"operator":"and","children":[{"field":"occurredAt","operator":"gte","value":"2026-10-01T10:00:00.123456+08:00"}]}');
});
test('Q36 rapid reversal, outside close and reduced motion leave no popup or focus blocker',async({page})=>{
  await page.emulateMedia({reducedMotion:'reduce'});const trigger=page.getByRole('button',{name:/自定义筛选/});
  await trigger.click();await expect(page.getByRole('dialog',{name:'自定义筛选'})).toBeVisible();
  await page.keyboard.press('Escape');await trigger.click();await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog',{name:'自定义筛选'})).not.toBeVisible();await expect(trigger).toBeFocused();
  await trigger.click();await page.getByRole('heading',{name:'人员管理'}).click();
  await expect(page.getByRole('dialog',{name:'自定义筛选'})).not.toBeVisible();
  await page.getByRole('button',{name:'切换加载'}).click();
});
