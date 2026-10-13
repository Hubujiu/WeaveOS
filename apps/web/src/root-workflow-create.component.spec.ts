import {test,expect,type Page} from '@playwright/test';
// V020 first save expectedRevision=0/201; V075 explicit new route and original recovery.
// Figma483:3310 new-flow entry reuses the existing definition editor and approved controls.
const id=(n:number)=>'e1e1e1e1-e1e1-4e1e-8e1e-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),fieldId=id(4),base=`applications/${appId}/forms/${viewId}`,managerRoute='/app/'+base+'/workflows';
type Mode='normal'|'unknown'|'runtime-denied'|'member';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[],writes:{path:string;body:any}[]=[];let saved:{path:string;body:any}|null=null;
 const app={id:appId,name:'流程创建测试',ownerUserId:mode==='member'?id(99):actor.id,policyRevision:1};
 const receipt=(write:{path:string;body:any})=>({operationId:write.body.operationId,id:write.path.split('/').at(-2),revision:1,currentVersion:0,candidateVersion:1,state:'disabled'});
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
  if(path==='sessions/current')data=actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===base+'/runtime'){
   if(mode==='runtime-denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   data={appId,viewId,tableId:id(8),schemaVersion:2,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(9),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
  }else if(path===base+'/workflows/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'create-list'};
  else if(path.startsWith(base+'/workflows/')&&path.endsWith('/definition')){
   if(method==='PUT'){
    const write={path,body};writes.push(write);if(mode==='unknown'&&writes.length===1)return r.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:null}});
    saved=write;return r.fulfill({status:201,json:{code:'OK',data:receipt(write),meta:null}});
   }
   if(!saved||saved.path!==path)return r.fulfill({status:404,json:{code:'APPLICATION_NOT_FOUND',data:null}});
   data={id:path.split('/').at(-2),name:saved.body.name,revision:1,currentVersion:0,candidateVersion:1,state:'disabled',schemaVersion:2,allowWithdraw:saved.body.allowWithdraw,graph:saved.body.graph,triggers:saved.body.triggers};
  }else if(path.startsWith('application-operations/'))return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes};
}
async function enter(page:Page){
 await page.goto(managerRoute);await page.getByRole('button',{name:'新建流程',exact:true}).click();await expect(page).toHaveURL(new RegExp('/workflows/[0-9a-f-]{36}/new$'));await page.getByRole('tab',{name:'触发与设置',exact:true}).click();
}
test('Root creating a workflow is local until explicit first save and remains unpublished and disabled',async({page})=>{
 const f=await setup(page);await page.goto(managerRoute);const shell=await page.getByTestId('workspace-shell').elementHandle();await page.getByRole('button',{name:'新建流程',exact:true}).click();await expect(page).toHaveURL(new RegExp('/workflows/[0-9a-f-]{36}/new$'));const flow=page.url().split('/').at(-2)!;expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);
 await expect(page.getByText('尚未保存',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'发布',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);expect(f.calls.filter(c=>c.path.endsWith('/definition'))).toEqual([]);
 await page.getByRole('tab',{name:'触发与设置',exact:true}).click();await page.getByRole('textbox',{name:'流程名称',exact:true}).fill('新采购审批');await page.getByRole('checkbox',{name:'手动发起',exact:true}).check();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page).toHaveURL('/app/'+base+'/workflows/'+flow+'/design');await expect(page.getByRole('heading',{name:'新采购审批',exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
 const write=f.writes[0];expect(write.path).toBe(base+'/workflows/'+flow+'/definition');expect(write.body).toEqual({operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),name:'新采购审批',expectedRevision:0,expectedSchemaVersion:2,allowWithdraw:false,triggers:[{event:'manual',condition:null}],graph:{version:1,nodes:[{id:expect.stringMatching(/^[0-9a-f-]{36}$/),kind:'start'},{id:expect.stringMatching(/^[0-9a-f-]{36}$/),kind:'end'}],edges:[{from:expect.any(String),to:expect.any(String)}]}});expect(write.body.graph.nodes[0].id).not.toBe(write.body.graph.nodes[1].id);expect(write.body.graph.edges).toEqual([{from:write.body.graph.nodes[0].id,to:write.body.graph.nodes[1].id}]);expect(f.calls.filter(c=>c.path.endsWith('/publish')||c.path.endsWith('/enable'))).toEqual([]);await expect(page.getByText('尚未发布',{exact:true})).toBeVisible();
});
test('Root cancelling a named new workflow uses the shared dirty guard and performs zero writes',async({page})=>{
 const f=await setup(page);await enter(page);await page.getByRole('textbox',{name:'流程名称',exact:true}).fill('未提交流程');await page.getByRole('button',{name:'返回流程列表',exact:true}).click();let guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await guard.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByRole('textbox',{name:'流程名称',exact:true})).toHaveValue('未提交流程');await page.getByRole('button',{name:'返回流程列表',exact:true}).click();guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(managerRoute);expect(f.writes).toEqual([]);
});
test('Root unknown first workflow save survives actual Shell unmount and retries its exact original flow and zero CAS',async({page})=>{
 const f=await setup(page,'unknown');await enter(page);await page.getByRole('textbox',{name:'流程名称',exact:true}).fill('未知创建');const originalRoute=page.url();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByRole('alert')).toContainText('尚未确认');const original=structuredClone(f.writes[0]);await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();await page.getByRole('button',{name:'返回主页',exact:true}).click();await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();await page.goBack();await expect(page).toHaveURL(originalRoute);await expect(page.getByRole('button',{name:'重试原保存',exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);await page.getByRole('button',{name:'重试原保存',exact:true}).click();await expect(page.getByRole('heading',{name:'未知创建',exact:true})).toBeVisible();expect(f.writes).toHaveLength(2);expect(f.writes[1]).toEqual(original);expect(original.body.expectedRevision).toBe(0);
});
test('Root failed current runtime prevents a new definition write',async({page})=>{
 const f=await setup(page,'runtime-denied');await page.goto(managerRoute);await page.getByRole('button',{name:'新建流程',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
test('Root ordinary member cannot open a new workflow from its direct address',async({page})=>{
 const f=await setup(page,'member');await page.goto(managerRoute+'/'+id(20)+'/new');await expect(page.getByRole('alert')).toContainText('流程管理权限');await expect(page.getByRole('button',{name:'保存',exact:true})).toHaveCount(0);expect(f.calls.filter(c=>c.path.endsWith('/definition')||c.path===base+'/runtime')).toEqual([]);
});
test('Root missing saved workflow is not silently converted into a create operation',async({page})=>{
 const f=await setup(page);await page.goto(managerRoute+'/'+id(20)+'/design');await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('button',{name:'保存',exact:true})).toHaveCount(0);await expect(page.getByText('尚未保存',{exact:true})).toHaveCount(0);expect(f.writes).toEqual([]);
});
