import {test,expect,type Page} from '@playwright/test';
const actor='11111111-1111-4111-8111-111111111111',app='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333';
const title='44444444-4444-4444-8444-444444444444',amount='55555555-5555-4555-8555-555555555555';
const path='/api/v1/applications/'+app+'/forms/'+viewId;
const field=(id:string,name:string,kind:string)=>({id,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:kind==='money'?['eq','neq','gt','gte','lt','lte']:['eq','neq'],sortable:kind==='money',quickSearchable:kind==='text'}});
const runtime=()=>({appId:app,tableId:app,viewId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[field(title,'事由','text'),field(amount,'金额','money')],layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000001',kind:'field',fieldId:title},{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000002',kind:'field',fieldId:amount}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}});
const row=(i:number)=>({id:'77777777-7777-4777-8777-'+String(i).padStart(12,'0'),appId:app,tableId:app,viewId,createdBy:actor,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:00:00Z',recordVersion:1,schemaVersion:1,values:{[title]:'测试记录 '+i,[amount]:'9007199254740993.01'},referenceDisplays:{}});
const pageData=(input:any,token:string)=>({items:Array.from({length:20},(_,i)=>row((input.page-1)*20+i+1)),total:40,page:input.page,pageSize:20,sort:input.sort??null,queryVersion:token,schemaVersion:1,viewVersion:1});
const envelope=(data:unknown)=>({code:'OK',data,meta:null});
const events=(page:Page)=>page.evaluate(()=>(window as any).__rootRecordWorkspace);
const open=(page:Page)=>page.goto('/src/applications/root-record-workspace-fixture.html');

test('Root runtime record list reads ordinary endpoints then opens the exact row context',async({page})=>{
 const requests:{method:string;pathname:string;body:any;actor?:string}[]=[];
 await page.route('**/api/v1/**',route=>{
  const request=route.request(),pathname=new URL(request.url()).pathname,body=request.postData()?request.postDataJSON():null;
  requests.push({method:request.method(),pathname,body,actor:request.headers()['x-expected-actor-id']});
  if(pathname===path+'/runtime')return route.fulfill({status:200,json:envelope(runtime())});
  if(pathname===path+'/records/search')return route.fulfill({status:200,json:envelope(pageData(body,'query-first'))});
  return route.fulfill({status:404,json:{code:'TEST_UNEXPECTED_ENDPOINT',data:null,meta:null}});
 });
 await open(page);
 const table=page.getByRole('table',{name:'记录',exact:true});
 await expect(table).toContainText('测试记录 1');
 expect(requests.map(r=>r.pathname)).toEqual([path+'/runtime',path+'/records/search']);
 expect(requests.every(r=>r.actor===actor)).toBe(true);
 expect(requests[1]).toMatchObject({method:'POST',body:{page:1,pageSize:20,filter:null,sort:null}});
 expect(requests[1].body).not.toHaveProperty('queryVersion');
 await table.getByRole('button',{name:'打开记录：事由 测试记录 1',exact:true}).click();
 await expect.poll(async()=> (await events(page)).opened.length).toBe(1);
 const opened=(await events(page)).opened[0];
 expect(opened.record.id).toBe(row(1).id);expect(opened.view.viewId).toBe(viewId);expect(opened.queryVersion).toBe('query-first');
});

test('Root page jump retains old query and explicit refresh returns to page one without it',async({page})=>{
 const queries:any[]=[];
 await page.route('**'+path+'/runtime',route=>route.fulfill({status:200,json:envelope(runtime())}));
 await page.route('**'+path+'/records/search',route=>{
  const input=route.request().postDataJSON();queries.push(input);
  return route.fulfill({status:200,json:envelope(pageData(input,'query-'+queries.length))});
 });
 await open(page);await expect(page.getByRole('table',{name:'记录'})).toContainText('测试记录 1');
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByRole('button',{name:'跳转到指定页',exact:true}).click();
 await expect(page.getByRole('table',{name:'记录'})).toContainText('测试记录 21');
 expect(queries[1]).toMatchObject({page:2,queryVersion:'query-1'});
 await page.getByRole('button',{name:'刷新',exact:true}).click();
 await expect(page.getByRole('table',{name:'记录'})).toContainText('测试记录 1');
 await expect.poll(()=>queries.length).toBe(3);
 expect(queries[2].page).toBe(1);expect(queries[2]).not.toHaveProperty('queryVersion');
});

test('Root changed query preserves existing rows until the user explicitly refreshes',async({page})=>{
 const queries:any[]=[];
 await page.route('**'+path+'/runtime',route=>route.fulfill({status:200,json:envelope(runtime())}));
 await page.route('**'+path+'/records/search',route=>{
  const input=route.request().postDataJSON();queries.push(input);
  if(queries.length===2)return route.fulfill({status:409,json:{code:'APPLICATION_QUERY_CHANGED',data:null,meta:null}});
  return route.fulfill({status:200,json:envelope(pageData(input,'query-live'))});
 });
 await open(page);await expect(page.getByRole('table',{name:'记录'})).toContainText('测试记录 1');
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByRole('button',{name:'跳转到指定页',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('刷新');
 await expect(page.getByRole('table',{name:'记录'})).toContainText('测试记录 1');
 expect(queries).toHaveLength(2);
 await expect(page.getByRole('button',{name:'新建记录',exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'刷新',exact:true}).click();
 await expect.poll(()=>queries.length).toBe(3);
 expect(queries[2].page).toBe(1);expect(queries[2]).not.toHaveProperty('queryVersion');
 await expect(page.getByRole('alert')).toHaveCount(0);
});

test('Root rejects a foreign-resource row instead of rendering or silently filtering it',async({page})=>{
 await page.route('**'+path+'/runtime',route=>route.fulfill({status:200,json:envelope(runtime())}));
 await page.route('**'+path+'/records/search',route=>{
  const result=pageData(route.request().postDataJSON(),'bad-page');
  result.items[0]={...result.items[0],appId:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',values:{[title]:'外部记录不得出现',[amount]:'1.00'}};
  return route.fulfill({status:200,json:envelope(result)});
 });
 await open(page);await expect(page.getByRole('alert')).toBeVisible();
 await expect(page.getByText('外部记录不得出现',{exact:false})).toHaveCount(0);
 expect((await events(page)).opened).toEqual([]);
});

test('Root create-only access opens the new record action without querying unreadable rows',async({page})=>{
 let searches=0;const view=runtime();view.capabilities.read='none';view.capabilities.search=false;view.fields.forEach(f=>{f.access.read='none';});
 await page.route('**'+path+'/runtime',route=>route.fulfill({status:200,json:envelope(view)}));
 await page.route('**'+path+'/records/search',route=>{searches++;return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null,meta:null}});});
 await open(page);await expect(page.getByRole('button',{name:'新建记录',exact:true})).toBeVisible();await page.getByRole('button',{name:'新建记录',exact:true}).click();
 await expect.poll(async()=> (await events(page)).creates.length).toBe(1);
 expect(searches).toBe(0);expect((await events(page)).creates[0].view.viewId).toBe(viewId);
 expect((await events(page)).creates[0].queryVersion).toBeUndefined();
});

test('Root metadata-only view changes reload runtime without discarding query context',async({page})=>{
 let runtimeCalls=0;const queries:any[]=[];
 await page.route('**'+path+'/runtime',route=>{
  runtimeCalls++;const next=runtime();if(runtimeCalls>1){next.viewVersion=2;next.fields[0].name='事由（新）';}
  return route.fulfill({status:200,json:envelope(next)});
 });
 await page.route('**'+path+'/records/search',route=>{
  const input=route.request().postDataJSON();queries.push(input);
  const result=pageData(input,'metadata-query-'+queries.length);if(queries.length>1)result.viewVersion=2;
  return route.fulfill({status:200,json:envelope(result)});
 });
 await open(page);
 const table=page.getByRole('table',{name:'记录',exact:true});
 await expect(table).toContainText('测试记录 1');
 await table.getByRole('button',{name:'排序 '+amount,exact:true}).click();
 await page.getByRole('menuitem',{name:'升序',exact:true}).click();
 await expect(table.locator('thead')).toContainText('事由（新）');
 expect(runtimeCalls).toBe(2);
 expect(queries[1]).toMatchObject({page:1,queryVersion:'metadata-query-1',sort:{fieldId:amount,direction:'asc'}});
 expect(queries.slice(1).every(input=>typeof input.queryVersion==='string'&&input.queryVersion.length>0)).toBe(true);
 await expect(page.getByRole('alert')).toHaveCount(0);
 await expect(table).toContainText('测试记录 1');
});
