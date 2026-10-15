import {test,expect,type Page} from '@playwright/test';
// V020 FLOW06, V075 P16/P17 and Figma488:6874. Only real HTTP boundary is isolated.
const id=(n:number)=>'d4d4d4d4-d4d4-4d4d-8d4d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),fieldId=id(5),base=`applications/${appId}/forms/${viewId}`,definitionPath=base+'/workflows/'+flowId+'/definition',route='/app/'+base+'/workflows/'+flowId+'/design';
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'approval',approval:{mode:'all',assigneeIds:[id(20),id(21)],editableFieldIds:[fieldId]}},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12)}]};
const triggers=[{event:'manual',condition:null},{event:'record.created',condition:{operator:'and',children:[{fieldId,operator:'eq',value:'原触发条件'}]}}];
type Options={candidateMode?:'normal'|'changed'|'denied'|'skew'|'late';write?:(body:any,n:number)=>{status:number;code:string;data?:unknown};lookupFails?:boolean;readAfterFails?:boolean};
async function setup(page:Page,options:Options={}){
 const calls:{path:string;method:string;body:any}[]=[],writes:any[]=[];let saved:any=null,identityChanged=false,refreshed=false,release:(()=>void)|undefined;
 const receipt=(operationId:string)=>({operationId,id:flowId,revision:4,currentVersion:1,candidateVersion:3,state:'enabled'});
 const app={id:appId,name:'流程保存测试',ownerUserId:actor.id,policyRevision:1};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=identityChanged?{id:id(99),account:'other'}:actor;
 else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
 else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===definitionPath&&method==='GET'){
  if(saved&&options.readAfterFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  data={id:flowId,name:'部门审批',revision:saved?4:3,currentVersion:1,candidateVersion:saved?3:2,state:'enabled',schemaVersion:1,allowWithdraw:saved?saved.allowWithdraw:true,graph:saved?.graph??graph,triggers:saved?.triggers??triggers};
 }else if(path===base+'/workflows/approvers/search'){
  if(options.candidateMode==='late')await new Promise<void>(resolve=>{release=resolve;});
  if(options.candidateMode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  if(options.candidateMode==='changed'&&body.page===2){refreshed=true;return r.fulfill({status:409,json:{code:'COMMON_QUERY_CHANGED',data:null}});}
  data={items:body.page===2?[{id:id(22),label:'次页审批人',status:'active'}]:[{id:id(20),label:refreshed?'刷新后审批人':'原审批人甲',status:'active'},{id:id(21),label:'原审批人乙',status:'active'}],total:21,page:body.page,pageSize:body.pageSize,queryVersion:refreshed?'new-token':'original-token',schemaVersion:options.candidateMode==='skew'?2:1,policyRevision:1};
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
 });return {calls,writes,receipt,release:()=>release?.(),changeIdentity:()=>{identityChanged=true;}};
}
const candidatePath=base+'/workflows/approvers/search';
async function open(page:Page){await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();await page.getByRole('button',{name:'选择审批人',exact:true}).click();return page.getByRole('dialog',{name:'选择审批人',exact:true});}
test('Root approver picker explicit open and confirmed cross-page selection only affect unsaved node IDs',async({page})=>{
 const f=await setup(page);await page.goto(route);await expect(page.getByRole('region',{name:'流程图',exact:true})).toBeVisible();expect(f.calls.filter(c=>c.path===candidatePath)).toEqual([]);
 await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();await page.getByRole('button',{name:'选择审批人',exact:true}).click();const dialog=page.getByRole('dialog',{name:'选择审批人',exact:true});
 await expect(dialog.getByRole('checkbox',{name:'原审批人甲',exact:true})).toBeChecked();await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await dialog.getByRole('button',{name:'下一页',exact:true}).click();await dialog.getByRole('checkbox',{name:'次页审批人',exact:true}).check();expect(f.writes).toEqual([]);await dialog.getByRole('button',{name:'确认',exact:true}).click();await expect(dialog).toHaveCount(0);expect(f.writes).toEqual([]);
 await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const g=structuredClone(graph);g.nodes[1].approval!.assigneeIds=[id(20),id(22)];expect(f.writes[0].body.graph).toEqual(g);expect(f.calls.filter(c=>c.path===candidatePath).map(c=>c.body)).toEqual([{page:1,pageSize:20},{page:2,pageSize:20,queryVersion:'original-token'}]);
});
test('Root approver picker cancel preserves original node and returns keyboard focus',async({page})=>{
 const f=await setup(page);const dialog=await open(page);await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(dialog).toHaveCount(0);await expect(page.getByRole('button',{name:'选择审批人',exact:true})).toBeFocused();expect(f.writes).toEqual([]);await page.getByRole('button',{name:'选择审批人',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'原审批人乙',exact:true})).toBeChecked();
});
test('Root approver picker does not permit an empty approval configuration',async({page})=>{
 const f=await setup(page);const dialog=await open(page);await dialog.getByRole('checkbox',{name:'原审批人甲',exact:true}).uncheck();await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await expect(dialog.getByRole('button',{name:'确认',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
test('Root changed approver query clears candidates and explicitly refreshes first page without old token',async({page})=>{
 const f=await setup(page,{candidateMode:'changed'});const dialog=await open(page);await dialog.getByRole('button',{name:'下一页',exact:true}).click();await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('checkbox',{name:'原审批人甲',exact:true})).toHaveCount(0);await dialog.getByRole('button',{name:'刷新',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'刷新后审批人',exact:true})).toBeVisible();expect(f.calls.filter(c=>c.path===candidatePath).at(-1)!.body).toEqual({page:1,pageSize:20});expect(f.writes).toEqual([]);
});
for(const candidateMode of ['denied','skew'] as const)test('Root approver '+candidateMode+' never leaves selectable stale candidates',async({page})=>{
 const f=await setup(page,{candidateMode});const dialog=await open(page);await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('checkbox')).toHaveCount(0);await expect(dialog.getByRole('button',{name:'确认',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
test('Root approver picker late response cannot reopen cancelled dialog',async({page})=>{
 const f=await setup(page,{candidateMode:'late'});const dialog=await open(page);await expect.poll(()=>f.calls.filter(c=>c.path===candidatePath).length).toBe(1);await dialog.getByRole('button',{name:'取消',exact:true}).click();f.release();await expect(dialog).toHaveCount(0);expect(f.writes).toEqual([]);
});
