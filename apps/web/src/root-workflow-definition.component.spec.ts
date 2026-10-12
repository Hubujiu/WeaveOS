import {test,expect,type Page} from '@playwright/test';
// V075 P17 / V019 / V020 / Figma483:3500. Real Shell; HTTP alone is isolated.
const id=(n:number)=>'d1d1d1d1-d1d1-4d1d-8d1d-'+String(n).padStart(12,'0'),actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),fieldId=id(4),tableId=id(5),base=`applications/${appId}/forms/${viewId}`,managerPath=base+'/workflows/search',route='/app/'+base+'/workflows';
const flowId=id(11),definitionPath=base+'/workflows/'+flowId+'/definition',designRoute=route+'/'+flowId+'/design';
type Mode='normal'|'changed'|'denied'|'leak'|'late'|'member'|'offline';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[];let changed=false,release:(()=>void)|undefined;
 const app={id:appId,name:'流程管理测试',ownerUserId:mode==='member'?id(99):actor.id,policyRevision:1};
 const runtime={appId,viewId,tableId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(6),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime')data=runtime;else if(path===base+'/records/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'records',schemaVersion:1,viewVersion:1};
 else if(path===definitionPath){
 if(mode==='late')await new Promise<void>(resolve=>{release=resolve;});
 if(mode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
 if(mode==='offline')return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
 data={id:flowId,name:'部门审批',revision:3,currentVersion:1,candidateVersion:2,state:'enabled',schemaVersion:1,allowWithdraw:true,graph:{version:1,nodes:[{id:id(21),kind:'end'},{id:id(20),kind:'approval',approval:{mode:'all',assigneeIds:[id(31),id(32)],editableFieldIds:[fieldId]}},{id:id(19),kind:'start'}],edges:[{from:id(19),to:id(20)},{from:id(20),to:id(21)}]},triggers:[{event:'manual',condition:null}],...(mode==='leak'?{bpmn:'secret-engine-data'}:{})};
 }else if(path===managerPath){
  if(mode==='late')await new Promise<void>(resolve=>{release=resolve;});
  if(mode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  if(mode==='offline')return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  if(mode==='changed'&&body.page===2){changed=true;return r.fulfill({status:409,json:{code:'COMMON_QUERY_CHANGED',data:null}});}
  const row={id:id(body.page===2?12:changed?13:11),name:body.page===2?'第二页流程':changed?'刷新后的流程':'部门审批',revision:3,currentVersion:1,candidateVersion:2,state:'closing',triggerEvents:['manual'],inFlightCount:2,updatedAt:'2026-10-12T09:00:00Z',...(mode==='leak'?{graph:{privateValue:'hidden-approval-secret'}}:{})};data={items:[row],total:21,page:body.page,pageSize:body.pageSize,queryVersion:changed?'manager-new':'manager-original'};
 }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return {calls,release:()=>release?.()};
}
const writes=(api:Awaited<ReturnType<typeof setup>>)=>api.calls.filter(c=>['PUT','PATCH','DELETE'].includes(c.method)||c.method==='POST'&&!c.path.endsWith('/search'));
test('Root workflow definition enters from its real summary without remounting Shell or pretending publication',async({page})=>{
 const api=await setup(page);await page.goto(route);await expect(page.getByText('部门审批',{exact:true})).toBeVisible();const shell=await page.getByTestId('workspace-shell').elementHandle();
 await page.getByRole('button',{name:'编辑部门审批',exact:true}).click();await expect(page).toHaveURL(designRoute);await expect(page.getByRole('heading',{name:'部门审批',exact:true})).toBeVisible();
 await expect(page.getByText('候选版本 2',{exact:true})).toBeVisible();await expect(page.getByText('已发布版本 1',{exact:true})).toBeVisible();expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);
 expect(api.calls.filter(c=>c.path===definitionPath).map(c=>c.method)).toEqual(['GET']);expect(writes(api)).toEqual([]);
 await page.getByRole('button',{name:'返回流程列表',exact:true}).click();await expect(page).toHaveURL(route);await expect(page.getByText('部门审批',{exact:true})).toBeVisible();
});
test('Root workflow definition orders actual DAG nodes and shows only real approval configuration',async({page})=>{
 const api=await setup(page);await page.goto(designRoute);const canvas=page.getByRole('region',{name:'流程图',exact:true});await expect(canvas).toBeVisible();
 const nodeButtons=canvas.getByRole('button');await expect(nodeButtons).toHaveText(['开始','审批节点','结束']);
 await canvas.getByRole('button',{name:'审批节点',exact:true}).click();const settings=page.getByRole('region',{name:'节点设置',exact:true});
 await expect(settings).toContainText(id(20));await expect(settings).toContainText('全部同意');await expect(settings).toContainText(id(31));await expect(settings).toContainText(id(32));await expect(settings).toContainText(fieldId);
 await expect(page.getByText('林晓',{exact:true})).toHaveCount(0);await expect(page.getByText('财务审批',{exact:true})).toHaveCount(0);expect(writes(api)).toEqual([]);
 await test.info().attach('workflow-definition-desktop',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});
test('Root workflow settings reads current trigger and withdraw configuration without mutating it',async({page})=>{
 const api=await setup(page);await page.goto(designRoute);await page.getByRole('tab',{name:'触发与设置',exact:true}).click();const settings=page.getByRole('region',{name:'触发与设置',exact:true});await expect(settings).toContainText('手动');await expect(settings).toContainText('允许撤回');expect(writes(api)).toEqual([]);
});
for(const mode of ['denied','leak','offline'] as const)test('Root definition '+mode+' clears graph and never presents success',async({page})=>{
 const api=await setup(page,mode);await page.goto(designRoute);await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('region',{name:'流程图',exact:true})).toHaveCount(0);await expect(page.getByText('secret-engine-data')).toHaveCount(0);expect(writes(api)).toEqual([]);
});
test('Root ordinary reader cannot query definitions through a direct designer URL',async({page})=>{
 const api=await setup(page,'member');await page.goto(designRoute);await expect(page.getByRole('alert')).toContainText('流程管理权限');expect(api.calls.filter(c=>c.path===definitionPath)).toEqual([]);
});
test('Root leaving a pending definition ignores its late response and preserves the list',async({page})=>{
 const api=await setup(page,'late');await page.goto(designRoute);await expect.poll(()=>api.calls.filter(c=>c.path===definitionPath).length).toBe(1);await page.getByRole('button',{name:'返回流程列表',exact:true}).click();await expect(page).toHaveURL(route);api.release();await expect(page.getByRole('region',{name:'流程图',exact:true})).toHaveCount(0);expect(writes(api)).toEqual([]);
});
test('Root narrow workflow definition contains its canvas and lets node settings remain reachable',async({page})=>{
 await page.setViewportSize({width:390,height:844});await setup(page);await page.goto(designRoute);const canvas=page.getByRole('region',{name:'流程图',exact:true});await canvas.getByRole('button',{name:'审批节点',exact:true}).click();const settings=page.getByRole('region',{name:'节点设置',exact:true});await settings.scrollIntoViewIfNeeded();await expect(settings).toBeInViewport();expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);await test.info().attach('workflow-definition-narrow',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});
