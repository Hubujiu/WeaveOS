import {test,expect,type Page} from '@playwright/test';
// Independent V019/V020/V075 P17, Figma483:3500/484:3052. Only HTTP is isolated.
const id=(n:number)=>'d2d2d2d2-d2d2-4d2d-8d2d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),fieldId=id(5),base=`applications/${appId}/forms/${viewId}`,definitionPath=base+'/workflows/'+flowId+'/definition',route='/app/'+base+'/workflows/'+flowId+'/design';
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'approval',approval:{mode:'all',assigneeIds:[id(20),id(21)],editableFieldIds:[fieldId]}},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12)}]};
const triggers=[{event:'manual',condition:null},{event:'record.created',condition:{operator:'and',children:[{fieldId,operator:'eq',value:'原触发条件'}]}}];
type Options={currentSchema?:number;runtimeDenied?:boolean;write?:(body:any,n:number)=>{status:number;code:string;data?:unknown};lookupFails?:boolean;readAfterFails?:boolean};
async function setup(page:Page,options:Options={}){
 const calls:{path:string;method:string;body:any}[]=[],writes:any[]=[];let saved:any=null,identityChanged=false;
 const receipt=(operationId:string)=>({operationId,id:flowId,revision:4,currentVersion:1,candidateVersion:3,state:'enabled'});
 const app={id:appId,name:'流程保存测试',ownerUserId:actor.id,policyRevision:1};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=identityChanged?{id:id(99),account:'other'}:actor;
 else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
 else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime'){
  if(options.runtimeDenied)return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  data={appId,viewId,tableId:id(8),schemaVersion:options.currentSchema??1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(9),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 }else if(path===definitionPath&&method==='GET'){
  if(saved&&options.readAfterFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  data={id:flowId,name:'部门审批',revision:saved?4:3,currentVersion:1,candidateVersion:saved?3:2,state:'enabled',schemaVersion:saved?saved.expectedSchemaVersion:1,allowWithdraw:saved?saved.allowWithdraw:true,graph:saved?.graph??graph,triggers:saved?.triggers??triggers};
 }else if(path===definitionPath&&method==='PUT'){
  writes.push({body,method,path,headers:req.headers()});const reply=options.write?.(body,writes.length)??{status:200,code:'OK',data:receipt(body.operationId)};
  if(reply.status===200&&reply.code==='OK')saved=body;
  return r.fulfill({status:reply.status,json:{code:reply.code,data:reply.data??null,meta:null}});
 }else if(path.startsWith('application-operations/')){
  if(options.lookupFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  saved=writes[0].body;data={operationId:path.split('/').at(-1),status:'confirmed',httpStatus:200,location:null,result:receipt(path.split('/').at(-1)!)};
 }else if(path===base+'/workflows/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'manager'};
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes,receipt,changeIdentity:()=>{identityChanged=true;}};
}
async function edit(page:Page){await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();await page.getByLabel('通过规则',{exact:true}).selectOption('any');}
const save=(page:Page)=>page.getByRole('button',{name:'保存',exact:true});
test('Root definition Save persists edited rule and withdraw flag with original graph CAS and all triggers but does not publish',async({page})=>{
 const f=await setup(page);await edit(page);await page.getByRole('tab',{name:'触发与设置',exact:true}).click();await page.getByLabel('允许发起人撤回',{exact:true}).uncheck();expect(f.writes).toHaveLength(0);
 await save(page).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();await expect(page.getByText('候选版本 3',{exact:true})).toBeVisible();await expect(page.getByText('已发布版本 1',{exact:true})).toBeVisible();
 const expected=structuredClone(graph);expected.nodes[1].approval!.mode='any';expect(f.writes).toHaveLength(1);expect(f.writes[0].body).toEqual({operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),name:'部门审批',expectedRevision:3,expectedSchemaVersion:1,allowWithdraw:false,graph:expected,triggers});expect(f.writes[0].headers['x-expected-actor-id']).toBe(actor.id);expect(f.calls.filter(c=>c.path.endsWith('/publish')||c.path.endsWith('/enable'))).toEqual([]);
});
test('Root definition dirty leave and browser refresh protect edits without implicit save',async({page})=>{
 const f=await setup(page);await edit(page);expect(await page.evaluate(()=>{const e=new Event('beforeunload',{cancelable:true});window.dispatchEvent(e);return e.defaultPrevented;})).toBe(true);
 await page.getByRole('button',{name:'返回流程列表',exact:true}).click();await expect(page.getByRole('dialog',{name:'有未保存的修改'})).toBeVisible();await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('通过规则',{exact:true})).toHaveValue('any');
 await page.getByRole('button',{name:'返回流程列表',exact:true}).click();await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL('/app/'+base+'/workflows');expect(f.writes).toEqual([]);
});
test('Root definition version conflict keeps edits and never replaces CAS or silently retries',async({page})=>{
 const f=await setup(page,{write:()=>({status:409,code:'WORKFLOW_CONFLICT'})});await edit(page);await save(page).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByLabel('通过规则',{exact:true})).toHaveValue('any');expect(f.writes).toHaveLength(1);expect(f.calls.filter(c=>c.path===definitionPath&&c.method==='GET')).toHaveLength(1);
});
test('Root definition unknown and failed lookup retry only the same original complete request',async({page})=>{
 let f:Awaited<ReturnType<typeof setup>>;f=await setup(page,{lookupFails:true,write:(body,n)=>n===1?{status:503,code:'APPLICATION_OPERATION_UNCONFIRMED'}:{status:200,code:'OK',data:f.receipt(body.operationId)}});await edit(page);await save(page).click();await expect(page.getByRole('alert')).toContainText('尚未确认');await expect(save(page)).toBeDisabled();
 await page.getByRole('button',{name:'核查保存结果',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');expect(f.writes).toHaveLength(1);await page.getByRole('button',{name:'重试原保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(2);expect(f.writes[1].body).toEqual(f.writes[0].body);
});
test('Root definition malformed receipt remains unknown and never announces saved',async({page})=>{
 const f=await setup(page,{write:body=>({status:200,code:'OK',data:{operationId:body.operationId,id:id(99),revision:4,currentVersion:1,candidateVersion:3,state:'enabled'}})});await edit(page);await save(page).click();await expect(page.getByRole('alert')).toContainText('尚未确认');await expect(page.getByText('定义已保存',{exact:true})).toHaveCount(0);expect(f.calls.filter(c=>c.path===definitionPath&&c.method==='GET')).toHaveLength(1);
});
test('Root definition confirmed Save stays confirmed when definition reread fails',async({page})=>{
 const f=await setup(page,{readAfterFails:true});await edit(page);await save(page).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();await expect(page.getByRole('alert')).toBeVisible();await expect(save(page)).toHaveCount(0);expect(f.writes).toHaveLength(1);
});
test('Root definition synchronous double Save transmits one operation',async({page})=>{
 const f=await setup(page);await edit(page);await save(page).evaluate(el=>{(el as HTMLButtonElement).click();(el as HTMLButtonElement).click();});await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
});
test('Root definition changed identity cannot transmit Save',async({page})=>{
 const f=await setup(page);await edit(page);f.changeIdentity();await save(page).click();await expect(page.getByRole('alert')).toContainText('登录身份已变化');expect(f.writes).toEqual([]);
});
test('Root definition unknown Save survives actual Shell unmount and restores only its original operation',async({page})=>{
 let f:Awaited<ReturnType<typeof setup>>;f=await setup(page,{lookupFails:true,write:(body,n)=>n===1?{status:503,code:'APPLICATION_OPERATION_UNCONFIRMED'}:{status:200,code:'OK',data:f.receipt(body.operationId)}});await edit(page);await save(page).click();await expect(page.getByRole('alert')).toContainText('尚未确认');const first=structuredClone(f.writes[0].body);
 await page.getByRole('button',{name:'返回主页',exact:true}).click();await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();await page.goBack();await expect(page.getByRole('button',{name:'重试原保存',exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
 await page.getByRole('button',{name:'重试原保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(2);expect(f.writes[1].body).toEqual(first);
});

test('Root workflow older candidate saves only after reading current schema while preserving flow revision',async({page})=>{
 const f=await setup(page,{currentSchema:2});await edit(page);await save(page).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);expect(f.writes[0].body.expectedRevision).toBe(3);expect(f.writes[0].body.expectedSchemaVersion).toBe(2);expect(f.calls.some(c=>c.path===base+'/runtime'&&c.method==='GET')).toBe(true);
});
test('Root workflow inaccessible current schema cannot authorize a new definition Save',async({page})=>{
 const f=await setup(page,{runtimeDenied:true});await page.goto(route);await expect(page.getByRole('alert')).toBeVisible();await expect(save(page)).toBeDisabled();expect(f.writes).toEqual([]);expect(f.calls.some(c=>c.path===base+'/runtime'&&c.method==='GET')).toBe(true);
});

test('Root workflow editable-field choice uses current label but saves only exact IDs without changing the approval rule',async({page})=>{
 const f=await setup(page);await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();
 const field=page.getByRole('region',{name:'节点设置',exact:true}).getByRole('checkbox',{name:'说明',exact:true});await expect(field).toBeChecked();await field.uncheck();expect(f.writes).toEqual([]);await save(page).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();
 const expected=structuredClone(graph);expected.nodes[1].approval!.editableFieldIds=[];expect(f.writes[0].body.graph).toEqual(expected);expect(f.writes[0].body.triggers).toEqual(triggers);
});
