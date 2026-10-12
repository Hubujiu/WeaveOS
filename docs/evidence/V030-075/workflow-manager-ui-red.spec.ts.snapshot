import {test,expect,type Page} from '@playwright/test';
// V075 P15 closed manager discovery, V020 authority; Figma483:3310. Real Shell, external HTTP only mocked.
const id=(n:number)=>'d1d1d1d1-d1d1-4d1d-8d1d-'+String(n).padStart(12,'0'),actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),fieldId=id(4),tableId=id(5),base=`applications/${appId}/forms/${viewId}`,managerPath=base+'/workflows/search',route='/app/'+base+'/workflows';
type Mode='normal'|'changed'|'denied'|'leak'|'late'|'member'|'offline';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[];let changed=false,release:(()=>void)|undefined;
 const app={id:appId,name:'流程管理测试',ownerUserId:mode==='member'?id(99):actor.id,policyRevision:1};
 const runtime={appId,viewId,tableId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(6),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime')data=runtime;else if(path===base+'/records/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'records',schemaVersion:1,viewVersion:1};
 else if(path===managerPath){
  if(mode==='late')await new Promise<void>(resolve=>{release=resolve;});
  if(mode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  if(mode==='offline')return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  if(mode==='changed'&&body.page===2){changed=true;return r.fulfill({status:409,json:{code:'COMMON_QUERY_CHANGED',data:null}});}
  const row={id:id(body.page===2?12:changed?13:11),name:body.page===2?'第二页流程':changed?'刷新后的流程':'部门审批',revision:3,currentVersion:1,candidateVersion:2,state:'closing',triggerEvents:['manual'],inFlightCount:2,updatedAt:'2026-10-12T09:00:00Z',...(mode==='leak'?{graph:{privateValue:'hidden-approval-secret'}}:{})};data={items:[row],total:21,page:body.page,pageSize:body.pageSize,queryVersion:changed?'manager-new':'manager-original'};
 }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return {calls,release:()=>release?.()};
}
const queries=(api:Awaited<ReturnType<typeof setup>>)=>api.calls.filter(c=>c.path===managerPath);
test('Root workflow manager entry preserves Shell and displays real server summaries and page identity',async({page})=>{
 const api=await setup(page);await page.setViewportSize({width:1440,height:1000});await page.goto('/app/'+base);await expect(page.getByRole('button',{name:'新建记录',exact:true})).toBeVisible();const shell=await page.getByTestId('workspace-shell').elementHandle();await page.getByRole('button',{name:'审批流程',exact:true}).click();await expect(page).toHaveURL(route);await expect(page.getByRole('heading',{name:'审批流程',exact:true})).toBeVisible();await expect(page.getByText('部门审批',{exact:true})).toBeVisible();await expect(page.getByText('关闭中',{exact:true})).toBeVisible();expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);expect(queries(api)[0]).toEqual({path:managerPath,method:'POST',body:{page:1,pageSize:20}});await page.getByRole('button',{name:'下一页',exact:true}).click();await expect(page.getByText('第二页流程',{exact:true})).toBeVisible();await expect(page.getByText('部门审批',{exact:true})).toHaveCount(0);expect(queries(api)[1].body).toEqual({page:2,pageSize:20,queryVersion:'manager-original'});await test.info().attach('workflow-manager-desktop',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});
test('Root workflow manager changed query clears rows and requires explicit first-page refresh',async({page})=>{
 const api=await setup(page,'changed');await page.goto(route);await expect(page.getByText('部门审批',{exact:true})).toBeVisible();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect(page.getByRole('alert')).toContainText('变化');await expect(page.getByText('部门审批',{exact:true})).toHaveCount(0);expect(queries(api)).toHaveLength(2);await page.getByRole('button',{name:'刷新流程',exact:true}).click();await expect(page.getByText('刷新后的流程',{exact:true})).toBeVisible();expect(queries(api)[2].body).toEqual({page:1,pageSize:20});
});
for(const mode of ['denied','leak','offline'] as const)test('Root workflow manager '+mode+' fails closed without pretending empty success or unknown write',async({page})=>{
 const api=await setup(page,mode);await page.goto(route);await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText('部门审批',{exact:true})).toHaveCount(0);await expect(page.getByText('hidden-approval-secret')).toHaveCount(0);await expect(page.getByText('暂无流程',{exact:true})).toHaveCount(0);await expect(page.getByText('操作结果尚未确认',{exact:false})).toHaveCount(0);expect(api.calls.filter(c=>['PUT','PATCH','DELETE'].includes(c.method)||c.method==='POST'&&!c.path.endsWith('/search'))).toEqual([]);
});
test('Root ordinary form reader neither receives manager entry nor sends manager query from direct URL',async({page})=>{
 const api=await setup(page,'member');await page.goto('/app/'+base);await expect(page.getByRole('button',{name:'新建记录',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'审批流程',exact:true})).toHaveCount(0);await page.goto(route);await expect(page.getByRole('alert')).toContainText('流程管理权限');expect(queries(api)).toEqual([]);
});
test('Root manager navigation asks before dropping a dirty record and continue preserves its input',async({page})=>{
 const api=await setup(page);await page.goto('/app/'+base);await page.getByRole('button',{name:'新建记录',exact:true}).click();await page.getByRole('textbox',{name:'说明',exact:true}).fill('尚未保存');await page.getByRole('button',{name:'审批流程',exact:true}).click();let guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await guard.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByRole('textbox',{name:'说明',exact:true})).toHaveValue('尚未保存');expect(queries(api)).toEqual([]);await page.getByRole('button',{name:'审批流程',exact:true}).click();guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByText('部门审批',{exact:true})).toBeVisible();expect(api.calls.filter(c=>['PATCH','PUT','DELETE'].includes(c.method)||c.method==='POST'&&!c.path.endsWith('/search'))).toEqual([]);
});
test('Root leaving pending workflow manager cancels late results and returns to the original form',async({page})=>{
 const api=await setup(page,'late');await page.goto(route);await expect.poll(()=>queries(api).length).toBe(1);await page.getByRole('button',{name:'返回记录',exact:true}).click();await expect(page.getByRole('button',{name:'新建记录',exact:true})).toBeVisible();api.release();await expect(page.getByText('部门审批',{exact:true})).toHaveCount(0);await expect(page).toHaveURL('/app/'+base);
});
test('Root narrow manager keeps refresh and return controls reachable without page overflow',async({page})=>{
 await page.setViewportSize({width:390,height:844});await setup(page);await page.goto(route);await expect(page.getByText('部门审批',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'刷新流程',exact:true})).toBeInViewport();await expect(page.getByRole('button',{name:'返回记录',exact:true})).toBeInViewport();expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);await test.info().attach('workflow-manager-narrow',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});
