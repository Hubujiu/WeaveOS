import {test,expect} from '@playwright/test';
test('Root configured group and field order override schema field order',async({page})=>{
 await page.goto('/src/applications/records/root-layout-fixture.html');
 const group=page.getByRole('group',{name:'基本信息',exact:true});
 await expect(group).toBeVisible();
 await expect(group.getByRole('textbox')).toHaveCount(2);
 await expect(group.getByRole('textbox').nth(0)).toHaveAccessibleName('金额');
 await expect(group.getByRole('textbox').nth(1)).toHaveAccessibleName('事由');
 await expect(page.getByRole('separator')).toHaveCount(1);
});
test('Root layout does not expose unplaced or denied fields',async({page})=>{
 await page.goto('/src/applications/records/root-layout-fixture.html');
 await expect(page.getByLabel('事由',{exact:true})).toBeVisible();
 await expect(page.getByLabel('未放入布局',{exact:true})).toHaveCount(0);
 await expect(page.getByLabel('无权字段',{exact:true})).toHaveCount(0);
});
test('Root read layout preserves exact values and disables edits',async({page})=>{
 await page.goto('/src/applications/records/root-layout-fixture.html?mode=read');
 await expect(page.getByLabel('事由',{exact:true})).toHaveValue('差旅报销');
 await expect(page.getByLabel('金额',{exact:true})).toHaveValue('9007199254740993.01');
 await expect(page.getByLabel('金额',{exact:true})).not.toBeEditable();
 await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();
 await expect(page.getByLabel('未放入布局',{exact:true})).toHaveCount(0);
 await expect(page.getByLabel('无权字段',{exact:true})).toHaveCount(0);
});
test('Root system record version comes from actual record metadata',async({page})=>{
 await page.goto('/src/applications/records/root-layout-fixture.html?mode=read');
 await expect(page.getByLabel('记录版本',{exact:true})).toHaveValue('4');
 await expect(page.getByLabel('记录版本',{exact:true})).not.toBeEditable();
});
