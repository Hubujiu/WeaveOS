import {test,expect,type Page} from '@playwright/test';
// V018/V020/P18 + actual Figma484:3052 and 488:6807. Closing is not closed.
const id=(n:number)=>'d5d5d5d5-d5d5-4d5d-8d5d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),base=`applications/${appId}/forms/${viewId}/workflows/${flowId}`,route='/app/'+base+'/design';
type Mode='normal'|'disabled-result'|'unknown'|'enable'|'unpublished'|'closing'|'unpublished-candidate';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[],writes:any[]=[];let state=mode==='closing'?'closing':mode==='enable'||mode==='unpublished'||mode==='unpublished-candidate'?'disabled':'enabled',revision=3;
 const app={id:appId,name:'管理动作测试',ownerUserId:actor.id,policyRevision:1};
 const receipt=(operationId:string,action:string)=>({operationId,id:flowId,revision:4,currentVersion:1,candidateVersion:1,state:action==='enable'?'enabled':mode==='disabled-result'?'disabled':'closing'});
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===`applications/${appId}/forms/${viewId}/runtime`){
  data={appId,viewId,tableId:id(8),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:id(5),name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(9),kind:'field',fieldId:id(5)}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 }else if(path===base+'/definition')data={id:flowId,name:'采购审批',revision,currentVersion:mode==='unpublished'?0:1,candidateVersion:mode==='unpublished-candidate'?2:1,state,schemaVersion:1,allowWithdraw:true,graph:{version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'end'}],edges:[{from:id(10),to:id(11)}]},triggers:[{event:'manual',condition:null}]};
 else if(path===base+'/close'||path===base+'/enable'){
  writes.push({path,body});if(mode==='unknown'&&writes.length===1)return r.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:null}});
  const result=receipt(body.operationId,path.split('/').at(-1)!);state=result.state;revision=result.revision;data=result;
 }else if(path.startsWith('application-operations/'))return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes};
}
async function settings(page:Page){await page.goto(route);await page.getByRole('tab',{name:'触发与设置',exact:true}).click();}
async function close(page:Page){await page.getByRole('button',{name:'关闭流程',exact:true}).click();const dialog=page.getByRole('dialog',{name:'关闭流程',exact:true});await expect(dialog).toContainText('停止新的发起');await dialog.getByRole('button',{name:'开始关闭',exact:true}).click();}
test('Root workflow close confirmation cancel sends nothing and restores focus',async({page},testInfo)=>{
 const f=await setup(page);await settings(page);await page.getByRole('button',{name:'关闭流程',exact:true}).click();const d=page.getByRole('dialog',{name:'关闭流程',exact:true});await expect(d).toContainText('等待现有审批结束');await expect.poll(async()=>{const box=await d.boundingBox();return box?{width:Math.round(box.width),height:Math.round(box.height)}:null;}).toEqual({width:620,height:390});await testInfo.attach('workflow-close-confirmation',{body:await page.screenshot(),contentType:'image/png'});expect(f.writes).toEqual([]);await d.getByRole('button',{name:'取消',exact:true}).click();await expect(d).toHaveCount(0);await expect(page.getByRole('button',{name:'关闭流程',exact:true})).toBeFocused();expect(f.writes).toEqual([]);
});
test('Root workflow closing receipt is not falsely presented as closed',async({page})=>{
 const f=await setup(page);await settings(page);await close(page);await expect(page.getByText('关闭中',{exact:true})).toBeVisible();await expect(page.getByText('已关闭',{exact:true})).toHaveCount(0);expect(f.writes).toEqual([{path:base+'/close',body:{operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),expectedRevision:3}}]);expect(f.calls.filter(c=>c.method==='DELETE'||c.path.endsWith('/enable'))).toEqual([]);
});
test('Root workflow disabled receipt is shown as closed without deleting history',async({page})=>{
 const f=await setup(page,'disabled-result');await settings(page);await close(page);await expect(page.getByText('已关闭',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);expect(f.calls.filter(c=>c.method==='DELETE')).toEqual([]);
});
test('Root workflow unknown close only retries original action revision and key',async({page})=>{
 const f=await setup(page,'unknown');await settings(page);await close(page);await expect(page.getByRole('alert')).toContainText('尚未确认');await page.getByRole('button',{name:'核查管理结果',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');await page.getByRole('button',{name:'重试原管理操作',exact:true}).click();await expect(page.getByText('关闭中',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(2);expect(f.writes[1]).toEqual(f.writes[0]);
});
test('Root workflow enable uses only current persisted revision and never publishes implicitly',async({page})=>{
 const f=await setup(page,'enable');await settings(page);await page.getByRole('button',{name:'启用流程',exact:true}).click();await expect(page.getByText('已启用',{exact:true})).toBeVisible();expect(f.writes).toEqual([{path:base+'/enable',body:{operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),expectedRevision:3}}]);expect(f.calls.filter(c=>c.path.endsWith('/publish')||c.method==='PUT')).toEqual([]);
});
test('Root unpublished workflow cannot be enabled through settings',async({page})=>{
 const f=await setup(page,'unpublished');await settings(page);await expect(page.getByRole('button',{name:'启用流程',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
test('Root dirty workflow cannot close or implicitly persist settings',async({page})=>{
 const f=await setup(page);await settings(page);await page.getByLabel('允许发起人撤回',{exact:true}).uncheck();await expect(page.getByRole('button',{name:'关闭流程',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);expect(f.calls.filter(c=>c.method==='PUT')).toEqual([]);
});

test('Root closing workflow can explicitly reopen the confirmed current candidate without saving or publishing',async({page})=>{
 const f=await setup(page,'closing');await settings(page);await page.getByRole('button',{name:'启用流程',exact:true}).click();await expect(page.getByText('已启用',{exact:true})).toBeVisible();expect(f.writes).toEqual([{path:base+'/enable',body:{operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),expectedRevision:3}}]);expect(f.calls.filter(c=>c.path.endsWith('/publish')||c.method==='PUT')).toEqual([]);
});
test('Root workflow with a newer unconfirmed candidate must publish before enabling',async({page})=>{
 const f=await setup(page,'unpublished-candidate');await settings(page);await expect(page.getByRole('button',{name:'启用流程',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
