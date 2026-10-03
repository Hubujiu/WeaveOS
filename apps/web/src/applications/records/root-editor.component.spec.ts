import {test,expect,type Page} from '@playwright/test';
const actor='11111111-1111-4111-8111-111111111111',app='22222222-2222-4222-8222-222222222222',view='33333333-3333-4333-8333-333333333333';
const title='44444444-4444-4444-8444-444444444444',amount='55555555-5555-4555-8555-555555555555',flag='66666666-6666-4666-8666-666666666666',record='77777777-7777-4777-8777-777777777777';
const base='/api/v1/applications/'+app+'/forms/'+view+'/records';
const ok=(data:unknown)=>({code:'OK',data,meta:null});
const receipt=(operationId:string,version=1)=>({operationId,id:record,recordVersion:version,schemaVersion:2,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z'});
const events=(page:Page)=>page.evaluate(()=>(window as any).__rootEditor);
async function open(page:Page,mode='create'){
 await page.route('**/api/v1/sessions/current',route=>route.fulfill({json:ok({id:actor,account:'root'})}));
 await page.goto('/src/applications/records/root-editor-fixture.html?mode='+mode);
}
test('Root record form sends one canonical create and confirms only after the receipt',async({page})=>{
 let entered!:()=>void,release!:()=>void;const started=new Promise<void>(r=>entered=r),gate=new Promise<void>(r=>release=r);const writes:any[]=[];
 await page.route('**'+base,async route=>{const body=route.request().postDataJSON();writes.push({body,actor:route.request().headers()['x-expected-actor-id'],method:route.request().method()});entered();await gate;await route.fulfill({status:201,headers:{Location:base+'/'+record},json:ok(receipt(body.operationId))});});
 await open(page);
 await page.getByLabel('事由',{exact:true}).fill('差旅报销');
 await expect(page.getByLabel('金额',{exact:true})).toHaveValue('9007199254740993.01');
 await expect(page.getByLabel('无权字段',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'保存记录',exact:true}).click();
 try{await started;await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();expect((await events(page)).confirmations).toEqual([]);expect(await page.evaluate(()=>(window as any).__rootGuardStatus())).toContain('write_in_flight');}
 finally{release();}
 await expect.poll(async()=> (await events(page)).confirmations.length).toBe(1);
 expect(writes).toHaveLength(1);const {operationId,...body}=writes[0].body;
 expect(operationId).toMatch(/^[0-9a-f-]{36}$/i);expect(writes[0].actor).toBe(actor);expect(writes[0].method).toBe('POST');
 expect(body).toEqual({expectedSchemaVersion:2,queryVersion:'root-query-1',values:{[title]:'差旅报销',[amount]:'9007199254740993.01',[flag]:false}});
 expect((await events(page)).confirmations[0].identity).toMatchObject({kind:'record',recordId:record});
 expect((await events(page)).confirmationGuardStatuses).toEqual([['clean']]);
 expect(await page.evaluate(()=>(window as any).__rootGuardStatus())).toEqual(['clean']);
 await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();
});
test('Root record form edits only changed fields with row and schema CAS',async({page})=>{
 const writes:any[]=[];
 await page.route('**'+base+'/'+record,route=>{const body=route.request().postDataJSON();writes.push({body,method:route.request().method()});return route.fulfill({json:ok(receipt(body.operationId,5))});});
 await open(page,'edit');await page.getByLabel('金额',{exact:true}).fill('17.30');await page.getByRole('button',{name:'保存记录',exact:true}).click();
 await expect.poll(async()=> (await events(page)).confirmations.length).toBe(1);expect(writes).toHaveLength(1);
 const {operationId,...body}=writes[0].body;expect(operationId).toMatch(/^[0-9a-f-]{36}$/i);
 expect(writes[0].method).toBe('PATCH');expect(body).toEqual({expectedSchemaVersion:2,expectedRecordVersion:4,queryVersion:'root-query-1',changes:{[amount]:'17.30'}});
});
test('Root record form unknown save survives unmount and failed lookup using the same key',async({page})=>{
 const writes:any[]=[];const lookups:string[]=[];
 await page.route('**'+base,route=>{writes.push(route.request().postDataJSON());return route.abort('failed');});
 await page.route('**/api/v1/application-operations/*',route=>{const operationId=route.request().url().split('/').pop()!;lookups.push(operationId);return route.fulfill(lookups.length===1?{status:404,json:{code:'APPLICATION_NOT_FOUND',data:null}}:{json:ok({operationId,status:'confirmed',httpStatus:201,location:base+'/'+record,result:receipt(operationId)})});});
 await open(page);await page.getByLabel('事由',{exact:true}).fill('原始待确认值');await page.getByRole('button',{name:'保存记录',exact:true}).click();
 await expect(page.getByRole('status')).toHaveText('保存结果待确认');await page.getByRole('button',{name:'切换挂载'}).click();await page.getByRole('button',{name:'切换挂载'}).click();
 await expect(page.getByLabel('事由',{exact:true})).toHaveValue('原始待确认值');await expect(page.getByLabel('事由',{exact:true})).not.toBeEditable();
 await page.getByRole('button',{name:'恢复保存结果',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'恢复保存结果',exact:true}).click();await expect.poll(async()=> (await events(page)).confirmations.length).toBe(1);
 expect(writes).toHaveLength(1);expect(lookups).toEqual([writes[0].operationId,writes[0].operationId]);
});
test('Root another actor cannot inherit an unknown record editor packet or input',async({page})=>{
 let writes=0;await page.route('**'+base,route=>{writes++;return route.abort('failed');});await open(page);
 await page.getByLabel('事由',{exact:true}).fill('A 的私密输入');await page.getByRole('button',{name:'保存记录',exact:true}).click();await expect(page.getByRole('status')).toHaveText('保存结果待确认');
 await page.getByRole('button',{name:'切换身份'}).click();await expect(page.getByLabel('事由',{exact:true})).toHaveValue('');
 await expect(page.getByRole('button',{name:'恢复保存结果',exact:true})).toHaveCount(0);expect(writes).toBe(1);
});
for(const code of ['APPLICATION_RECORD_CONFLICT','APPLICATION_SCHEMA_CONFLICT','APPLICATION_QUERY_CHANGED','APPLICATION_QUERY_CONTEXT_EXPIRED','APPLICATION_POLICY_CONFLICT']){
 test('Root '+code+' retains input and requires explicit refresh before another save',async({page})=>{
  let writes=0;await page.route('**'+base+'/'+record,route=>{writes++;return route.fulfill({status:409,json:{code,data:null}});});await open(page,'edit');
  await page.getByLabel('事由',{exact:true}).fill('保留修改');await page.getByRole('button',{name:'保存记录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('刷新');await expect(page.getByLabel('事由',{exact:true})).toHaveValue('保留修改');
  await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();await page.getByRole('button',{name:'刷新记录',exact:true}).click();
  expect((await events(page)).refreshes).toBe(1);expect(writes).toBe(1);
 });
}
test('Root record form cancels an unsent preflight through the real leave guard',async({page})=>{
 await open(page);let enter!:()=>void,release!:()=>void,finish!:()=>void;const entered=new Promise<void>(r=>enter=r),gate=new Promise<void>(r=>release=r),settled=new Promise<void>(r=>finish=r);let writes=0;
 await page.route('**/api/v1/sessions/current',async route=>{enter();await gate;try{await route.fulfill({json:ok({id:actor})});}finally{finish();}});
 await page.route('**'+base,route=>{writes++;return route.abort();});
 const wire=page.waitForEvent('requestfinished',{predicate:r=>r.url().endsWith('/sessions/current')});
 await page.getByLabel('事由',{exact:true}).fill('未发送');await page.getByRole('button',{name:'保存记录',exact:true}).click();
 try{await entered;expect(await page.evaluate(()=>(window as any).__rootGuardStatus())).toContain('preflight');expect(await page.evaluate(()=>(window as any).__rootPrepareLeave())).toEqual({ok:true});}
 finally{release();await Promise.allSettled([wire,settled]);}
 await page.evaluate(()=>new Promise<void>(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r()))));expect(writes).toBe(0);
});
test('Root revoked field is hidden while an old pending operation stays recoverable',async({page})=>{
 await page.route('**'+base,route=>route.abort('failed'));await open(page);
 await page.getByLabel('事由',{exact:true}).fill('权限撤销后不可显示');await page.getByRole('button',{name:'保存记录',exact:true}).click();await expect(page.getByRole('status')).toHaveText('保存结果待确认');
 await page.getByRole('button',{name:'撤销事由权限'}).click();await expect(page.getByLabel('事由',{exact:true})).toHaveCount(0);
 await expect(page.getByText('权限撤销后不可显示',{exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'恢复保存结果',exact:true})).toBeEnabled();
});
test('Root description layout renders plain text without executable markup',async({page})=>{
 await open(page);await expect(page.getByText('<img src=x onerror=alert(1)>仅用于说明',{exact:true})).toBeVisible();await expect(page.locator('.record-form img')).toHaveCount(0);
});
