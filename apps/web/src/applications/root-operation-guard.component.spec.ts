import {test,expect,type Page} from '@playwright/test';
const actorId='11111111-1111-4111-8111-111111111111',appId='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const recordId='77777777-7777-4777-8777-777777777777';
const path='/api/v1/applications/'+appId+'/forms/'+viewId+'/records';
const envelope=(data:unknown)=>({code:'OK',data,meta:null});
const receipt=(operationId:string)=>({operationId,id:recordId,recordVersion:1,schemaVersion:1,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z'});
const events=(page:Page)=>page.evaluate(()=>(window as any).__rootScopedOperation);
const pending=(page:Page)=>page.evaluate(()=>(window as any).__rootPendingStatus());
async function open(page:Page,mode='create'){
 await page.route('**/api/v1/sessions/current',route=>route.fulfill({status:200,json:envelope({id:actorId,account:'root'})}));
 await page.goto('/src/applications/root-scoped-fixture.html?mode='+mode);
}

test('Root leave guard sees preflight immediately and cancellation prevents the write',async({page})=>{
 await open(page);let release!:()=>void,started!:()=>void,settled!:()=>void;let writes=0;
 const gate=new Promise<void>(resolve=>{release=resolve;});
 const entered=new Promise<void>(resolve=>{started=resolve;});
 const done=new Promise<void>(resolve=>{settled=resolve;});
 await page.route('**/api/v1/sessions/current',async route=>{started();await gate;try{await route.fulfill({status:200,json:envelope({id:actorId,account:'root'})});}finally{settled();}});
 await page.route('**'+path,route=>{writes++;const op=route.request().postDataJSON().operationId;return route.fulfill({status:201,headers:{Location:path+'/'+recordId},json:envelope(receipt(op))});});
 const wire=page.waitForEvent('requestfinished',{predicate:r=>r.url().endsWith('/sessions/current')});
 try{
  await page.getByRole('button',{name:'保存',exact:true}).click();await entered;
  expect((await events(page)).afterStartPending).toEqual(['preflight']);
  expect(await pending(page)).toBe('preflight');
  expect(await page.evaluate(()=>(window as any).__rootCancelPreflight())).toBeNull();
 }finally{release();await Promise.allSettled([wire,done]);}
 await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))));
 expect(writes).toBe(0);await expect(page.getByLabel('操作阶段')).toHaveText('idle');
});

test('Root leave guard distinguishes a sent write and clears only on confirmation',async({page})=>{
 let release!:()=>void,started!:()=>void;
 const gate=new Promise<void>(resolve=>{release=resolve;});const entered=new Promise<void>(resolve=>{started=resolve;});
 await page.route('**'+path,async route=>{const op=route.request().postDataJSON().operationId;started();await gate;await route.fulfill({status:201,headers:{Location:path+'/'+recordId},json:envelope(receipt(op))});});
 await open(page);
 try{await page.getByRole('button',{name:'保存',exact:true}).click();await entered;expect(await pending(page)).toBe('write_in_flight');}
 finally{release();}
 await expect.poll(async()=> (await events(page)).confirmed.length).toBe(1);
 expect(await pending(page)).toBeNull();
 expect((await events(page)).confirmedPending).toEqual([null]);
});

for(const status of [408,500,504]){
 test('Root ambiguous HTTP '+status+' keeps the original write pending',async({page})=>{
  await page.route('**'+path,route=>route.fulfill({status,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null,meta:null}}));
  await open(page);await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByLabel('操作阶段')).toHaveText('unconfirmed');
  expect(await pending(page)).toBe('unknown');
  await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();
  expect((await events(page)).confirmed).toEqual([]);
 });
}
test('Root definitive record conflict exposes a safe error code for refresh handling',async({page})=>{
 await page.route('**'+path+'/'+recordId,route=>route.fulfill({status:409,json:{code:'APPLICATION_RECORD_CONFLICT',data:{currentRecordVersion:2},meta:null}}));
 await open(page,'edit');await page.getByRole('button',{name:'保存',exact:true}).click();
 await expect(page.getByLabel('操作阶段')).toHaveText('error');
 await expect(page.getByLabel('错误代码')).toHaveText('APPLICATION_RECORD_CONFLICT');
 expect(await pending(page)).toBeNull();expect((await events(page)).confirmed).toEqual([]);
});
