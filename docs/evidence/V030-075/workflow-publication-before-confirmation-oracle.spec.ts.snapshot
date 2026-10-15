import {test,expect,type Page} from '@playwright/test';
// V027 five-state publication, V075 P17 and Figma483:3500. Never equate 202 with publication.
const id=(n:number)=>'d3d3d3d3-d3d3-4d3d-8d3d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),base=`applications/${appId}/forms/${viewId}/workflows/${flowId}`,route='/app/'+base+'/design';
type Mode='pending'|'unknown'|'blocked'|'superseded'|'bad-location'|'wrong-version';
async function setup(page:Page,mode:Mode='pending'){
 const calls:{path:string;method:string;body:any}[]=[],writes:any[]=[];let published=false;
 const app={id:appId,name:'发布测试',ownerUserId:actor.id,policyRevision:1};
 const result=(operationId:string,status:string,reason:string|null=null)=>({operationId,flowId,version:2,status,reason});
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/definition')data={id:flowId,name:'采购审批',revision:3,currentVersion:published?2:1,candidateVersion:2,state:'disabled',schemaVersion:1,allowWithdraw:true,graph:{version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'end'}],edges:[{from:id(10),to:id(11)}]},triggers:[{event:'manual',condition:null}]};
 else if(path===base+'/publish'){
  writes.push(body);const terminal=mode==='blocked'||mode==='superseded';const status=terminal?mode:mode==='unknown'?'unknown':'pending';
  data={...result(body.operationId,status,mode==='blocked'?'SCHEMA_INCOMPATIBLE':mode==='superseded'?'CANDIDATE_SUPERSEDED':mode==='unknown'?'DEPENDENCY_UNAVAILABLE':null),...(mode==='wrong-version'?{version:99}:{})};
  return r.fulfill({status:terminal?200:202,headers:terminal?{}:{Location:mode==='bad-location'?'https://other.invalid/publish':'/api/v1/'+base+'/publications/'+body.operationId},json:{code:'OK',data,meta:null}});
 }else if(path.startsWith(base+'/publications/')){published=true;data=result(path.split('/').at(-1)!,'confirmed');}
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes};
}
const publish=(page:Page)=>page.getByRole('button',{name:'发布',exact:true});
test('Root publication 202 retains original candidate until dedicated query confirms without enabling',async({page})=>{
 const f=await setup(page);await page.goto(route);await publish(page).click();await expect(page.getByText('发布处理中',{exact:true})).toBeVisible();await expect(page.getByText('发布已确认',{exact:true})).toHaveCount(0);await expect(page.getByText('已发布版本 1',{exact:true})).toBeVisible();await expect(publish(page)).toBeDisabled();
 expect(f.writes).toHaveLength(1);expect(f.writes[0]).toEqual({operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),expectedRevision:3,expectedSchemaVersion:1});
 await page.getByRole('button',{name:'核查发布结果',exact:true}).click();await expect(page.getByText('发布已确认',{exact:true})).toBeVisible();await expect(page.getByText('已发布版本 2',{exact:true})).toBeVisible();expect(f.calls.filter(c=>c.path.startsWith(base+'/publications/')).map(c=>c.path)).toEqual([base+'/publications/'+f.writes[0].operationId]);expect(f.calls.filter(c=>c.path.endsWith('/enable')||c.path.startsWith('application-operations/'))).toEqual([]);expect(f.writes).toHaveLength(1);
});
test('Root unknown publication retains same CAS and operation across explicit retry',async({page})=>{
 const f=await setup(page,'unknown');await page.goto(route);await publish(page).click();await expect(page.getByText('发布结果待确认',{exact:true})).toBeVisible();await expect(publish(page)).toBeDisabled();await page.getByRole('button',{name:'重试原发布',exact:true}).click();await expect.poll(()=>f.writes.length).toBe(2);expect(f.writes[1]).toEqual(f.writes[0]);await expect(page.getByText('发布已确认',{exact:true})).toHaveCount(0);
});
for(const [mode,message] of [['blocked','发布未生效'],['superseded','候选版本已被替代']] as const)test('Root publication '+mode+' is never presented as published',async({page})=>{
 const f=await setup(page,mode);await page.goto(route);await publish(page).click();await expect(page.getByText(message,{exact:true})).toBeVisible();await expect(page.getByText('发布已确认',{exact:true})).toHaveCount(0);await expect(page.getByText('已发布版本 1',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);expect(f.calls.filter(c=>c.path.endsWith('/enable'))).toEqual([]);
});
for(const mode of ['bad-location','wrong-version'] as const)test('Root publication '+mode+' cannot change the published version',async({page})=>{
 const f=await setup(page,mode);await page.goto(route);await publish(page).click();await expect(page.getByRole('alert')).toContainText('尚未确认');await expect(page.getByText('发布已确认',{exact:true})).toHaveCount(0);await expect(page.getByText('已发布版本 1',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
});
test('Root publication refuses dirty settings rather than automatically saving them',async({page})=>{
 const f=await setup(page);await page.goto(route);await page.getByRole('tab',{name:'触发与设置',exact:true}).click();await page.getByLabel('允许发起人撤回',{exact:true}).uncheck();await expect(publish(page)).toBeDisabled();expect(f.writes).toEqual([]);expect(f.calls.filter(c=>c.method==='PUT')).toEqual([]);
});
