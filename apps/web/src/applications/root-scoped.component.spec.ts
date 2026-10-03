import {test,expect,type Page} from '@playwright/test';
const actorId='11111111-1111-4111-8111-111111111111';
const appId='22222222-2222-4222-8222-222222222222';
const viewId='33333333-3333-4333-8333-333333333333';
const recordId='77777777-7777-4777-8777-777777777777';
const draftId='88888888-8888-4888-8888-888888888888';
const base='/api/v1/applications/'+appId+'/forms/'+viewId;
const envelope=(data:unknown)=>({code:'OK',data,meta:null});
const receipt=(operationId:string)=>({operationId,id:recordId,recordVersion:1,schemaVersion:1,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z'});
const events=(page:Page)=>page.evaluate(()=>(window as any).__rootScopedOperation);
async function open(page:Page,mode='create'){
 await page.route('**/api/v1/sessions/current',route=>route.fulfill({status:200,json:envelope({id:actorId,account:'root-fixture'})}));
 await page.goto('/src/applications/root-scoped-fixture.html?mode='+mode);
}

test('Root scoped create confirms exact receipt and canonical Location once',async({page})=>{
 let writes=0;let submitted:any;
 await page.route('**'+base+'/records',route=>{writes++;submitted=route.request().postDataJSON();expect(route.request().headers()['x-expected-actor-id']).toBe(actorId);
 return route.fulfill({status:201,headers:{Location:base+'/records/'+recordId},json:envelope(receipt(submitted.operationId))});});
 await open(page);await page.getByRole('button',{name:'保存',exact:true}).click();
 await expect.poll(async()=> (await events(page)).confirmed.length).toBe(1);
 expect(writes).toBe(1);expect(submitted.values).toEqual({amount:'1.20'});
 expect(submitted.operationId).toMatch(/^[0-9a-f-]{36}$/i);
 await expect(page.getByLabel('操作阶段')).toHaveText('idle');
});

for(const malformed of ['extra-values','wrong-operation','wrong-record','wrong-location']){
 test('Root scoped create keeps original operation for '+malformed,async({page})=>{
  let writes=0,operationId='';
  await page.route('**'+base+'/records',route=>{
   writes++;operationId=route.request().postDataJSON().operationId;
   const data:any=receipt(operationId);
   if(malformed==='extra-values')data.values={secret:'must-not-confirm'};
   if(malformed==='wrong-operation')data.operationId='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
   if(malformed==='wrong-record')data.id=draftId;
   return route.fulfill({status:201,headers:{Location:malformed==='wrong-location'?base+'/records/'+draftId:base+'/records/'+recordId},json:envelope(data)});
  });
  await page.route('**/api/v1/application-operations/*',route=>{
   expect(route.request().url().split('/').at(-1)).toBe(operationId);
   expect(route.request().headers()['x-expected-actor-id']).toBe(actorId);
   return route.fulfill({status:200,json:envelope({operationId,status:'confirmed',httpStatus:201,location:base+'/records/'+recordId,result:receipt(operationId)})});
  });
  await open(page);await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByLabel('操作阶段')).toHaveText('unconfirmed');
  expect((await events(page)).confirmed).toEqual([]);
  await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();
  await expect(page.getByLabel('当前操作')).toHaveText(operationId);
  await page.getByRole('button',{name:'核查原操作',exact:true}).click();
  await expect.poll(async()=> (await events(page)).confirmed.length).toBe(1);
  expect(writes).toBe(1);
  expect((await events(page)).confirmed[0].result.operationId).toBe(operationId);
 });
}

test('Root scoped PATCH sends one operation and correct record CAS',async({page})=>{
 let body:any;
 await page.route('**'+base+'/records/'+recordId,route=>{
  expect(route.request().method()).toBe('PATCH');body=route.request().postDataJSON();
  return route.fulfill({status:200,json:envelope({...receipt(body.operationId),recordVersion:2})});
 });
 await open(page,'edit');await page.getByRole('button',{name:'保存',exact:true}).click();
 await expect.poll(async()=> (await events(page)).confirmed.length).toBe(1);
 expect(body).toEqual({operationId:expect.any(String),expectedSchemaVersion:1,expectedRecordVersion:1,changes:{amount:'1.20'}});
});

test('Root draft DELETE empty 204 confirms without invented response body',async({page})=>{
 let writes=0;
 await page.route('**'+base+'/drafts/'+draftId+'?*',route=>{
  writes++;const request=route.request(),url=new URL(request.url());
  expect(request.method()).toBe('DELETE');expect(request.postData()).toBeNull();
  expect(url.searchParams.get('operationId')).toMatch(/^[0-9a-f-]{36}$/i);
  expect(url.searchParams.get('expectedDraftVersion')).toBe('2');
  return route.fulfill({status:204,body:''});
 });
 await open(page,'delete');await page.getByRole('button',{name:'保存',exact:true}).click();
 await expect.poll(async()=> (await events(page)).confirmed.length).toBe(1);
 expect((await events(page)).confirmed[0].empty).toBe(true);expect(writes).toBe(1);
});

test('Root unknown DELETE recovers minimum receipt without a second delete',async({page})=>{
 let writes=0,operationId='';
 await page.route('**'+base+'/drafts/'+draftId+'?*',route=>{
  writes++;operationId=new URL(route.request().url()).searchParams.get('operationId')!;
  return route.abort('failed');
 });
 await page.route('**/api/v1/application-operations/*',route=>{
  expect(route.request().url().split('/').at(-1)).toBe(operationId);
  return route.fulfill({status:200,json:envelope({operationId,status:'confirmed',httpStatus:204,location:null,result:{operationId,id:draftId,draftVersion:2}})});
 });
 await open(page,'delete');await page.getByRole('button',{name:'保存',exact:true}).click();
 await expect(page.getByLabel('操作阶段')).toHaveText('unconfirmed');
 await page.getByRole('button',{name:'核查原操作',exact:true}).click();
 await expect.poll(async()=> (await events(page)).confirmed.length).toBe(1);
 expect(writes).toBe(1);expect((await events(page)).confirmed[0].result).toEqual({operationId,id:draftId,draftVersion:2});
});
