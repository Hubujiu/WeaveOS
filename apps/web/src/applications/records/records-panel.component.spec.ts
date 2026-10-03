import {test,expect} from '@playwright/test';

test('records use one server ordered Table with stable IDs, guarded cells and manual numeric sort',async({page})=>{
 await page.goto('/src/applications/records/records-fixture.html');
 const table=page.getByRole('table',{name:'记录'});
 await expect(table.locator('tbody tr:not([data-empty-row])').first()).toContainText('10.02');
 await expect(table.locator('tbody tr:not([data-empty-row])').last()).toContainText('2.001');
 await expect(table.locator('tbody')).not.toContainText('不可见');
 await expect(table.locator('tbody')).toContainText('本人内容');
 await expect(table.locator('tbody')).toContainText('引用信息不可用');
 await table.getByRole('button',{name:'排序 55555555-5555-4555-8555-555555555555'}).click();
 await page.getByRole('menuitem',{name:'升序'}).click();
 const actions=await page.evaluate(()=>(window as typeof window & {__recordsFixture:{sorts:unknown[]}}).__recordsFixture);
 expect(actions.sorts).toEqual([{fieldId:'55555555-5555-4555-8555-555555555555',direction:'asc'}]);
 await expect(table.locator('tbody tr:not([data-empty-row])').first()).toContainText('10.02');
});

test('hidden column and arbitrary empty deep page stay controlled without erasing scope',async({page})=>{
 await page.goto('/src/applications/records/records-fixture.html');
 const table=page.getByRole('table',{name:'记录'});
 await page.getByRole('button',{name:'隐藏金额'}).click();
 await expect(table.locator('thead')).not.toContainText('金额');
 await expect(table.locator('thead')).toContainText('标题');
 await page.getByRole('button',{name:'跳至空深页'}).click();
 await expect(page.getByLabel('records-state')).toContainText('"page":99');
 await expect(table.locator('tbody')).not.toContainText('先返回');
 await expect(page.getByText('暂无记录')).toBeVisible();
});
