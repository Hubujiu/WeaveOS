import {test,expect,type Page} from '@playwright/test';

type Events={saves:{operationId:string;values:Record<string,unknown>}[];recoveries:string[];confirmations:{result:{operationId:string};identity:{kind:string;recordId:string}}[];discarded:number};
const inspect=(page:Page)=>page.evaluate(()=>(window as typeof window&{__rootRecovery:Events}).__rootRecovery);
const fieldId='44444444-4444-4444-8444-444444444444';

// Oracle: V017 PRD pending/unknown retain original operation; ADR §5 says
// failed recovery queries do not prove the original write failed.
for(const scenario of ['failed','throw','malformed']){
 test('Root unknown save survives '+scenario+' recovery without a second write',async({page})=>{
  const errors:string[]=[];page.on('pageerror',error=>errors.push(error.message));
  await page.goto('/src/applications/records/root-recovery-fixture.html?scenario='+scenario);
  await page.getByLabel('事由',{exact:true}).fill('报销测试');
  await page.getByRole('button',{name:'保存记录',exact:true}).click();
  await expect(page.getByRole('status')).toHaveText('保存结果待确认');
  const before=await inspect(page);
  expect(before.saves).toHaveLength(1);
  expect(before.saves[0].values).toEqual({[fieldId]:'报销测试'});
  expect(before.confirmations).toHaveLength(0);
  await page.getByRole('button',{name:'恢复保存结果',exact:true}).click();
  await expect.poll(async()=> (await inspect(page)).recoveries.length).toBe(1);
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page.getByRole('status')).toHaveText('保存结果待确认');
  await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();
  await expect(page.getByRole('button',{name:'放弃填写',exact:true})).toBeDisabled();
  await expect(page.getByLabel('事由',{exact:true})).toHaveValue('报销测试');
  await expect(page.getByLabel('事由',{exact:true})).not.toBeEditable();
  await expect(page.getByRole('button',{name:'恢复保存结果',exact:true})).toBeEnabled();
  expect((await inspect(page)).confirmations).toHaveLength(0);
  await page.getByRole('button',{name:'恢复保存结果',exact:true}).click();
  await expect.poll(async()=> (await inspect(page)).confirmations.length).toBe(1);
  const after=await inspect(page);
  expect(after.saves).toHaveLength(1);
  expect(after.recoveries).toEqual([before.saves[0].operationId,before.saves[0].operationId]);
  expect(after.confirmations[0].result.operationId).toBe(before.saves[0].operationId);
  expect(after.confirmations[0].identity).toMatchObject({kind:'record',recordId:'77777777-7777-4777-8777-777777777777'});
  expect(after.discarded).toBe(0);
  await expect(page.getByRole('status')).toHaveCount(0);
  expect(errors).toEqual([]);
 });
}

test('Root definitive first-write rejection allows editing and a fresh submission',async({page})=>{
 await page.goto('/src/applications/records/root-recovery-fixture.html?scenario=initial-failure');
 await page.getByLabel('事由',{exact:true}).fill('初始值');
 await page.getByRole('button',{name:'保存记录',exact:true}).click();
 await expect(page.getByRole('alert')).toHaveText('输入信息不合法，请检查后重试');
 await expect(page.getByRole('status')).toHaveCount(0);
 await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeEnabled();
 await expect(page.getByRole('button',{name:'恢复保存结果',exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'使用同一操作重试',exact:true})).toHaveCount(0);
 await page.getByLabel('事由',{exact:true}).fill('修正值');
 await page.getByRole('button',{name:'保存记录',exact:true}).click();
 await expect.poll(async()=> (await inspect(page)).confirmations.length).toBe(1);
 const result=await inspect(page);
 expect(result.saves).toHaveLength(2);
 expect(result.saves[1].operationId).not.toBe(result.saves[0].operationId);
 expect(result.saves[1].values).toEqual({[fieldId]:'修正值'});
 expect(result.recoveries).toEqual([]);
});
