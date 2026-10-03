import {test,expect} from '@playwright/test';

test('record form keeps precise and false values, excludes denied fields, and recovers the same unknown operation',async({page})=>{
 await page.goto('/src/applications/records/record-form-fixture.html');
 await expect(page.getByLabel('金额')).toHaveValue('9007199254740993.01');
 await expect(page.getByLabel('同意')).not.toBeChecked();
 await expect(page.getByLabel('无权字段')).toHaveCount(0);
 await expect(page.getByText('未选择')).toBeVisible();
 await page.getByLabel('金额').fill('9007199254740993.02');
 await expect(page.getByLabel('form-state')).toContainText('"dirty":true');
 await page.getByRole('button',{name:'保存记录'}).click();
 await expect(page.getByText('保存结果待确认')).toBeVisible();
 await expect(page.getByRole('button',{name:'保存记录'})).toBeDisabled();
 const before=await page.evaluate(()=>(window as typeof window&{__recordFormFixture:{saves:{operationId:string;values:Record<string,unknown>}[]}}).__recordFormFixture.saves);
 expect(before).toHaveLength(1);
 expect(before[0].values).toEqual({'44444444-4444-4444-8444-444444444444':'9007199254740993.02','55555555-5555-4555-8555-555555555555':false});
 await page.getByRole('button',{name:'恢复保存结果'}).click();
 const after=await page.evaluate(()=>(window as typeof window&{__recordFormFixture:{saves:{operationId:string}[];recoveries:string[];confirmations:{identity:{recordId:string}}[]}}).__recordFormFixture);
 expect(after.recoveries).toEqual([before[0].operationId]);
 expect(after.confirmations[0].identity.recordId).toBe('77777777-7777-4777-8777-777777777777');
});

test('record form uses scoped ordinary candidate selector and keeps selected label only in unsaved state',async({page})=>{
 await page.goto('/src/applications/records/record-form-fixture.html');
 await page.getByRole('button',{name:'选择成员'}).click();
 await page.getByRole('searchbox',{name:'搜索成员'}).fill('可选');
 await page.getByRole('button',{name:'可选成员'}).click();
 await expect(page.getByText('可选成员')).toBeVisible();
 await page.getByRole('button',{name:'更改结构版本'}).click();
 await expect(page.getByText('引用信息不可用（需修复）')).toBeVisible();
});

test('dirty form preserves input and blocks save when runtime control version changes',async({page})=>{
 await page.goto('/src/applications/records/record-form-fixture.html');
 await page.getByLabel('金额').fill('15.30');
 await page.getByRole('button',{name:'更改结构版本'}).click();
 await expect(page.getByLabel('金额')).toHaveValue('15.30');
 await expect(page.getByText('结构或权限已变化，请核对当前输入')).toBeVisible();
 await expect(page.getByRole('button',{name:'保存记录'})).toBeDisabled();
});
