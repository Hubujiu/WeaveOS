import {test,expect,type Page} from '@playwright/test';
// V068 current persistent deletion, P20; source close-confirmation component composition only.
const id=(n:number)=>'e5e5e5e5-e5e5-4e5e-8e5e-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),base=`applications/${appId}/forms/${viewId}/workflows/${flowId}`,route='/app/'+base+'/design';
type Mode='normal'|'unknown'|'cleared'|'bad-receipt';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[],writes:{path:string;body:any}[]=[];let identityChanged=false;
 const app={id:appId,name:'流程删除测试',ownerUserId:actor.id,policyRevision:1};
 const pending=(operationId:string)=>({operationId,flowId,status:'pending',reason:null,deletedVersions:null,deletedAt:null,completedAt:null});
 const done=(operationId:string)=>({...pending(operationId),status:'deleted',deletedVersions:2,deletedAt:'2026-10-13T08:00:00Z',completedAt:'2026-10-13T08:00:01Z'});
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=identityChanged?{id:id(99),account:'other'}:actor;
 else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
 else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===`applications/${appId}/forms/${viewId}/runtime`)data={appId,viewId,tableId:id(8),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:id(5),name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(9),kind:'field',fieldId:id(5)}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 else if(path===base+'/definition'){
  if(mode==='cleared'&&writes.length)return r.fulfill({status:404,json:{code:'APPLICATION_NOT_FOUND',data:null}});
  data={id:flowId,name:'采购审批',revision:3,currentVersion:1,candidateVersion:1,state:'enabled',schemaVersion:1,allowWithdraw:true,graph:{version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'end'}],edges:[{from:id(10),to:id(11)}]},triggers:[{event:'manual',condition:null}]};
 }else if(path===base+'/delete'){
  writes.push({path,body});if(mode==='unknown'&&writes.length===1)return r.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:null}});
  if(mode==='unknown')return r.fulfill({status:200,json:{code:'OK',data:done(body.operationId),meta:null}});
  if(mode==='bad-receipt')return r.fulfill({status:200,json:{code:'OK',data:{...done(body.operationId),completedAt:null},meta:null}});
  return r.fulfill({status:202,headers:{Location:'/api/v1/'+base+'/deletions/'+body.operationId},json:{code:'OK',data:pending(body.operationId),meta:null}});
 }else if(path.startsWith(base+'/deletions/'))data=done(path.split('/').at(-1)!);
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes,changeIdentity:()=>{identityChanged=true;}};
}
async function settings(page:Page){await page.goto(route);await page.getByRole('tab',{name:'触发与设置',exact:true}).click();}
async function confirm(page:Page){await page.getByRole('button',{name:'删除流程',exact:true}).click();await page.getByRole('dialog',{name:'删除流程',exact:true}).getByRole('button',{name:'确认删除',exact:true}).click();}
test('Root workflow deletion confirmation discloses irreversible configuration and retained logs while cancel writes nothing',async({page},testInfo)=>{
 const f=await setup(page);await settings(page);await page.getByRole('button',{name:'删除流程',exact:true}).click();const dialog=page.getByRole('dialog',{name:'删除流程',exact:true});await expect(dialog).toContainText('不可恢复');await expect(dialog).toContainText('日志');await expect(dialog).toContainText('等待');await expect.poll(async()=>{const r=await dialog.boundingBox();return r?{width:Math.round(r.width),height:Math.round(r.height)}:null;}).toEqual({width:620,height:390});await testInfo.attach('workflow-deletion-confirmation',{body:await page.screenshot(),contentType:'image/png'});expect(f.writes).toEqual([]);await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(dialog).toHaveCount(0);await expect(page.getByRole('button',{name:'删除流程',exact:true})).toBeFocused();expect(f.writes).toEqual([]);
});
test('Root accepted workflow deletion stops new edits but is complete only after dedicated deleted receipt',async({page})=>{
 const f=await setup(page);await settings(page);await confirm(page);await expect(page.getByText('流程删除处理中',{exact:true})).toBeVisible();await expect(page.getByText('流程已删除',{exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'发布',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'删除流程',exact:true})).toBeDisabled();expect(f.writes).toEqual([{path:base+'/delete',body:{operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),expectedRevision:3}}]);await page.getByRole('button',{name:'核查删除结果',exact:true}).click();await expect(page.getByText('流程已删除',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'保存',exact:true})).toHaveCount(0);expect(f.calls.filter(c=>c.path.startsWith('application-operations/'))).toEqual([]);expect(f.calls.filter(c=>c.path.startsWith(base+'/deletions/'))[0].path).toBe(base+'/deletions/'+f.writes[0].body.operationId);expect(f.writes).toHaveLength(1);
});
test('Root unknown workflow deletion retries the same original revision and key without inventing another delete',async({page})=>{
 const f=await setup(page,'unknown');await settings(page);await confirm(page);await expect(page.getByRole('alert')).toContainText('尚未确认');await page.getByRole('button',{name:'重试原删除',exact:true}).click();await expect(page.getByText('流程已删除',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(2);expect(f.writes[1]).toEqual(f.writes[0]);
});
test('Root deletion recovery survives actual Shell unmount even after the flow definition disappears',async({page})=>{
 const f=await setup(page,'cleared');await settings(page);await confirm(page);await expect(page.getByText('流程删除处理中',{exact:true})).toBeVisible();await page.getByRole('button',{name:'返回主页',exact:true}).click();await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();await page.goBack();await expect(page.getByRole('button',{name:'核查删除结果',exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);await page.getByRole('button',{name:'核查删除结果',exact:true}).click();await expect(page.getByText('流程已删除',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
});
test('Root dirty workflow cannot delete or silently save local settings',async({page})=>{
 const f=await setup(page);await settings(page);await page.getByRole('checkbox',{name:'允许发起人撤回',exact:true}).uncheck();await expect(page.getByRole('button',{name:'删除流程',exact:true})).toBeDisabled();expect(f.calls.filter(c=>['PUT','PATCH','DELETE'].includes(c.method)||c.path.endsWith('/delete'))).toEqual([]);
});
test('Root malformed deletion completion remains unknown without claiming successful removal',async({page})=>{
 const f=await setup(page,'bad-receipt');await settings(page);await confirm(page);await expect(page.getByRole('alert')).toContainText('尚未确认');await expect(page.getByText('流程已删除',{exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'重试原删除',exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
});
test('Root changed identity cannot send workflow deletion after its confirmation is opened',async({page})=>{
 const f=await setup(page);await settings(page);await page.getByRole('button',{name:'删除流程',exact:true}).click();f.changeIdentity();await page.getByRole('dialog',{name:'删除流程',exact:true}).getByRole('button',{name:'确认删除',exact:true}).click();await expect(page.getByRole('alert')).toContainText('登录身份已变化');expect(f.writes).toEqual([]);
});
