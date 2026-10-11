import {test,expect,type Page} from '@playwright/test';
// V046 + V075 P7: real Shell and record-opening UI, mocked HTTP boundary only.
const id=(n:number)=>'bbbbbbbb-bbbb-4bbb-8bbb-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reader'},appId=id(2),viewId=id(3),fieldId=id(4),recordId=id(5);
const app={id:appId,name:'报销管理',ownerUserId:id(99),policyRevision:1};
const form=`applications/${appId}/forms/${viewId}`,record=form+'/records/'+recordId,search=record+'/workflow-instances/search';
const runtime={appId,viewId,tableId:id(6),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:false,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq','gt','gte','lt','lte'],sortable:true,quickSearchable:false}}],layout:[{id:id(7),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const item={id:recordId,appId,viewId,tableId:id(6),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:1,schemaVersion:1,values:{[fieldId]:'10.00'},referenceDisplays:{}};
const flow=(page=1)=>({id:id(10+page),flowId:id(20),name:'关联审批 '+page,initiatorId:actor.id,state:'active',createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',definitionVersion:2,sequence:1});
async function setup(page:Page,options:{changed?:boolean;denied?:boolean;malformed?:boolean;empty?:boolean;failure?:boolean}={}){
 const calls:{path:string;method:string;body:any;actor?:string}[]=[];let n=0;
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),body=req.postData()?req.postDataJSON():null;calls.push({path,method:req.method(),body,actor:req.headers()['x-expected-actor-id']});let data:unknown;
  if(path==='sessions/current')data=actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[app]};
  else if(path==='applications/'+appId)data=app;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===form+'/runtime')data=runtime;
  else if(path===form+'/records/search')data={items:[item],total:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'records',schemaVersion:1,viewVersion:1};
  else if(path===record)data=item;
  else if(path===search){
   n++;if(options.denied||options.changed&&body.page===2||options.failure&&n===1)return route.fulfill({status:options.denied?403:options.changed?409:503,json:{code:options.denied?'APPLICATION_FORBIDDEN':options.changed?'APPLICATION_QUERY_CHANGED':'COMMON_SERVICE_UNAVAILABLE',data:null,meta:null}});
   data={items:options.empty?[]:[{...flow(body.page),...(options.malformed?{evidenceHash:'must-not-disclose'}:{})}],total:options.empty?0:21,page:body.page,pageSize:body.pageSize,queryVersion:n>1&&!body.queryVersion?'refreshed':'flows-v1'};
  }else return route.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return route.fulfill({json:{code:'OK',data,meta:null}});
 });return calls;
}
async function open(page:Page){await page.goto('/app/'+form);await page.getByRole('button',{name:'打开记录：金额 10.00',exact:true}).click();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');}
test('Root record detail reads only its scoped workflow page and preserves server pagination identity',async({page})=>{
 const calls=await setup(page);await open(page);const panel=page.getByRole('region',{name:'关联流程',exact:true});await expect(panel).toContainText('关联审批 1');await expect(panel).toContainText('审批中');await panel.getByRole('button',{name:'下一页',exact:true}).click();await expect(panel).toContainText('关联审批 2');
 const reads=calls.filter(c=>c.path===search);expect(reads.map(c=>c.body)).toEqual([{page:1,pageSize:20},{page:2,pageSize:20,queryVersion:'flows-v1'}]);expect(reads.every(c=>c.method==='POST'&&c.actor===actor.id)).toBe(true);expect(calls.filter(c=>c.method!=='GET'&&!c.path.endsWith('/search'))).toEqual([]);
});
test('Root record workflows changed query clears old summaries and requires explicit first-page refresh',async({page})=>{
 const calls=await setup(page,{changed:true});await open(page);const panel=page.getByRole('region',{name:'关联流程',exact:true});await expect(panel).toContainText('关联审批 1');await panel.getByRole('button',{name:'下一页',exact:true}).click();await expect(panel).toContainText('已变化');await expect(panel).not.toContainText('关联审批 1');await panel.getByRole('button',{name:'刷新流程',exact:true}).click();await expect(panel).toContainText('关联审批 1');expect(calls.filter(c=>c.path===search).at(-1)!.body).toEqual({page:1,pageSize:20});await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');
});
for(const failure of ['denied','malformed'] as const)test('Root record workflow '+failure+' is not a successful empty list',async({page})=>{
 await setup(page,{[failure]:true});await open(page);const panel=page.getByRole('region',{name:'关联流程',exact:true});await expect(panel).toBeVisible();await expect(panel.getByRole('alert')).toBeVisible();await expect(panel).not.toContainText('暂无关联流程');await expect(panel).not.toContainText('关联审批 1');await expect(panel).not.toContainText('must-not-disclose');
});
test('Root record workflows successful empty page is explicit and never invents action eligibility',async({page})=>{
 const calls=await setup(page,{empty:true});await open(page);const panel=page.getByRole('region',{name:'关联流程',exact:true});await expect(panel).toContainText('暂无关联流程');expect(calls.some(c=>c.path.endsWith('/lifecycle')||c.path.endsWith('/actions'))).toBe(false);
});
test('Root record workflow read failure retries the read without creating an operation',async({page})=>{
 const calls=await setup(page,{failure:true});await open(page);const panel=page.getByRole('region',{name:'关联流程',exact:true});await expect(panel.getByRole('alert')).toBeVisible();await expect(panel).not.toContainText('尚未确认');await panel.getByRole('button',{name:'重试流程读取',exact:true}).click();await expect(panel).toContainText('关联审批 1');expect(calls.filter(c=>c.path===search).map(c=>c.body)).toEqual([{page:1,pageSize:20},{page:1,pageSize:20}]);
});
test('Root record workflow refresh preserves unsaved record editing without writing',async({page})=>{
 const calls=await setup(page);await open(page);const panel=page.getByRole('region',{name:'关联流程',exact:true});await expect(panel).toContainText('关联审批 1');await page.getByRole('button',{name:'编辑记录',exact:true}).click();await page.getByLabel('金额',{exact:true}).fill('25.00');await panel.getByRole('button',{name:'刷新流程',exact:true}).click();await expect(panel).toContainText('关联审批 1');await expect(page.getByLabel('金额',{exact:true})).toHaveValue('25.00');expect(calls.filter(c=>c.method!=='GET'&&!c.path.endsWith('/search'))).toEqual([]);
});
test('Root record workflow closing cancels late unauthorized read without leaving the form workspace',async({page})=>{
 await setup(page);let release:(()=>void)|undefined,started=false;
 await page.route('**/api/v1/'+search,async r=>{started=true;await new Promise<void>(resolve=>{release=resolve;});await r.fulfill({status:401,json:{code:'AUTH_UNAUTHENTICATED',data:null,meta:null}}).catch(()=>{});});
 await open(page);await expect.poll(()=>started).toBe(true);await page.getByRole('button',{name:'关闭',exact:true}).click();release!();await expect(page.getByRole('button',{name:'打开记录：金额 10.00',exact:true})).toBeVisible();await expect(page.getByRole('region',{name:'关联流程',exact:true})).toHaveCount(0);await expect(page).toHaveURL('/app/'+form);
});
