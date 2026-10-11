import {test,expect,type Page} from '@playwright/test';
// V045 + V075 P8 and Figma 487:5726. Tests precede all withdrawal UI/hook integration.
const id=(n:number)=>'dddddddd-dddd-4ddd-8ddd-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'initiator'},appId=id(2),viewId=id(3),fieldId=id(4),recordId=id(5),instanceId=id(8);
const form=`applications/${appId}/forms/${viewId}`,record=form+'/records/'+recordId,instance=record+'/workflow-instances/'+instanceId,search=record+'/workflow-instances/search';
const app={id:appId,name:'报销管理',ownerUserId:id(99),policyRevision:1};
const field={id:fieldId,name:'金额',kind:'money',required:false,default:null,config:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const runtime={appId,viewId,tableId:id(6),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:false,presentation:field.presentation,input:{decimal:field.config},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq','gt','gte','lt','lte'],sortable:true,quickSearchable:false}}],layout:[{id:id(7),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const item={id:recordId,appId,viewId,tableId:id(6),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:2,schemaVersion:1,values:{[fieldId]:'10.00'},referenceDisplays:{}};
async function setup(page:Page,options:{denied?:boolean;notEligible?:boolean;unknown?:boolean;noEffect?:boolean;stale?:boolean;wrongResult?:boolean}={}){
 const calls:{path:string;method:string;body:any}[]=[];let operationId='',posts=0;
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),body=req.postData()?req.postDataJSON():null;calls.push({path,method:req.method(),body});let data:unknown;let status=200;
  if(path==='sessions/current')data=actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[app]};
  else if(path==='applications/'+appId)data=app;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===form+'/runtime')data=runtime;
  else if(path===form+'/records/search')data={items:[item],total:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'records',schemaVersion:1,viewVersion:1};
  else if(path===record)data=item;
  else if(path===search)data={items:[{id:instanceId,flowId:id(9),name:'财务审批',initiatorId:actor.id,state:'active',createdAt:item.createdAt,updatedAt:item.updatedAt,definitionVersion:1,sequence:2}],total:1,page:body.page,pageSize:body.pageSize,queryVersion:'flows'};
  else if(path===instance+'/lifecycle'){
   if(options.denied)return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   data={basisToken:'life-basis',instance:{id:instanceId,sequence:2,state:'active'},record:item,fields:[field],canWithdraw:!options.notEligible,returnTargets:options.notEligible?[{nodeId:id(10),taskId:id(11),activationEpoch:1}]:[]};
  }else if(path===instance+'/actions'){
   operationId=body.operationId;posts++;
   if(options.stale)return r.fulfill({status:409,json:{code:'WORKFLOW_BASIS_CHANGED',data:null}});
   if(options.unknown&&posts===1)return r.abort('failed');
   status=202;data={operationId,instanceId,commandId:id(12),status:'pending'};
   return r.fulfill({status,headers:{Location:'/api/v1/application-workflow-operations/'+operationId},json:{code:'OK',data,meta:null}});
  }else if(path==='application-workflow-operations/'+operationId)data={operationId,instanceId:options.wrongResult?id(99):instanceId,commandId:id(12),status:options.noEffect?'no_effect':'success',instanceState:options.noEffect?'unchanged':'withdrawn',sequence:options.noEffect?2:3,...(options.noEffect?{reason:'terminal_instance'}:{})};
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({status,json:{code:'OK',data,meta:null}});
 });return calls;
}
async function open(page:Page){await page.goto('/app/'+form);await page.getByRole('button',{name:'打开记录：金额 10.00',exact:true}).click();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');}
async function preview(page:Page){await page.getByRole('button',{name:'查看流程操作：财务审批',exact:true}).click();}
async function confirm(page:Page){await preview(page);await page.getByRole('button',{name:'撤回审批',exact:true}).click();const dialog=page.getByRole('dialog',{name:'撤回财务审批',exact:true});await expect(dialog).toContainText('记录及已保存修改将保留');await dialog.getByRole('button',{name:'确认撤回',exact:true}).click();}
test('Root withdrawal qualification is loaded on demand and cancel never submits',async({page})=>{
 const calls=await setup(page);await open(page);expect(calls.some(c=>c.path.endsWith('/lifecycle'))).toBe(false);await preview(page);await page.getByRole('button',{name:'撤回审批',exact:true}).click();await page.getByRole('dialog',{name:'撤回财务审批',exact:true}).getByRole('button',{name:'取消',exact:true}).click();expect(calls.filter(c=>c.path===instance+'/actions')).toEqual([]);
});
test('Root withdrawal 202 stays pending until original dedicated execution receipt confirms',async({page})=>{
 const calls=await setup(page);await open(page);await confirm(page);await expect(page.getByText('撤回处理中',{exact:true})).toBeVisible();await expect(page.getByText('已撤回',{exact:true})).toHaveCount(0);await page.getByRole('button',{name:'核查撤回结果',exact:true}).click();await expect(page.getByText('已撤回',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.path===instance+'/actions');expect(writes).toHaveLength(1);expect(writes[0].body).toEqual({operationId:expect.any(String),action:'withdraw',basisToken:'life-basis'});expect(calls.some(c=>c.path.startsWith('application-operations/'))).toBe(false);expect(calls.filter(c=>['PATCH','DELETE'].includes(c.method))).toEqual([]);
});
for(const option of ['denied','notEligible'] as const)test('Root withdrawal '+option+' never exposes a confirmation capability',async({page})=>{
 const calls=await setup(page,{[option]:true});await open(page);await preview(page);if(option==='denied')await expect(page.getByRole('alert').filter({hasText:/权限/})).toBeVisible();else await expect(page.getByText('当前不可撤回',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'撤回审批',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'确认撤回',exact:true})).toHaveCount(0);expect(calls.filter(c=>c.path===instance+'/actions')).toEqual([]);
});
test('Root withdrawal network loss retries only the immutable original request',async({page})=>{
 const calls=await setup(page,{unknown:true});await open(page);await confirm(page);await expect(page.getByText(/撤回结果.*未确认/)).toBeVisible();await page.getByRole('button',{name:'重试原撤回',exact:true}).click();await expect(page.getByText('撤回处理中',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.path===instance+'/actions');expect(writes).toHaveLength(2);expect(writes[1].body).toEqual(writes[0].body);
});
test('Root withdrawal no-effect is not reported as withdrawn',async({page})=>{
 await setup(page,{noEffect:true});await open(page);await confirm(page);await page.getByRole('button',{name:'核查撤回结果',exact:true}).click();await expect(page.getByText('撤回未执行',{exact:true})).toBeVisible();await expect(page.getByText('已撤回',{exact:true})).toHaveCount(0);
});
test('Root withdrawal stale basis requires explicit refresh without a replacement write',async({page})=>{
 const calls=await setup(page,{stale:true});await open(page);await confirm(page);await expect(page.getByText(/依据.*变化|依据.*过期/)).toBeVisible();expect(calls.filter(c=>c.path===instance+'/lifecycle')).toHaveLength(1);expect(calls.filter(c=>c.path===instance+'/actions')).toHaveLength(1);await expect(page.getByRole('button',{name:'确认撤回',exact:true})).toBeDisabled();
});
test('Root withdrawal mismatched instance receipt remains unconfirmed',async({page})=>{
 await setup(page,{wrongResult:true});await open(page);await confirm(page);await page.getByRole('button',{name:'核查撤回结果',exact:true}).click();await expect(page.getByText(/撤回结果.*未确认/)).toBeVisible();await expect(page.getByText('已撤回',{exact:true})).toHaveCount(0);
});
test('Root withdrawal two synchronous confirmation clicks send one operation',async({page})=>{
 const calls=await setup(page);await open(page);await preview(page);await page.getByRole('button',{name:'撤回审批',exact:true}).click();await page.getByRole('button',{name:'确认撤回',exact:true}).evaluate((button:HTMLButtonElement)=>{button.click();button.click();});await expect(page.getByText('撤回处理中',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===instance+'/actions')).toHaveLength(1);
});
test('Root withdrawal dirty record guard never silently saves or opens an action on continue editing',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByRole('button',{name:'编辑记录',exact:true}).click();await page.getByLabel('金额',{exact:true}).fill('25.00');await preview(page);const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('25.00');expect(calls.filter(c=>c.path===instance+'/lifecycle'||c.path===instance+'/actions'||c.method==='PATCH')).toEqual([]);
});
test('Root withdrawal identity preflight mismatch never transmits the action',async({page})=>{
 const calls=await setup(page);await open(page);await preview(page);await page.getByRole('button',{name:'撤回审批',exact:true}).click();let checks=0;await page.route('**/api/v1/sessions/current',async r=>{checks++;return r.fulfill({json:{code:'OK',data:{id:id(77),account:'different'},meta:null}});});await page.getByRole('button',{name:'确认撤回',exact:true}).click();await expect.poll(()=>checks).toBeGreaterThan(0);expect(calls.filter(c=>c.path===instance+'/actions')).toEqual([]);await expect(page.getByText('已撤回',{exact:true})).toHaveCount(0);
});
test('Root closing an accepted withdrawal preserves its record and disables a new edit until recovery',async({page})=>{
 await setup(page);await open(page);await confirm(page);await expect(page.getByText('撤回处理中',{exact:true})).toBeVisible();await page.getByRole('dialog',{name:'撤回财务审批',exact:true}).getByRole('button',{name:'关闭',exact:true}).click();const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('region',{name:'记录详情',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'编辑记录',exact:true})).toBeDisabled();await preview(page);await page.getByRole('button',{name:'核查撤回结果',exact:true}).click();await expect(page.getByText('已撤回',{exact:true})).toBeVisible();
});
test('Root withdrawal definite authorization revocation clears the earlier capability',async({page})=>{
 const calls=await setup(page);await open(page);await preview(page);await page.getByRole('button',{name:'撤回审批',exact:true}).click();await page.route('**/api/v1/'+instance+'/actions',r=>r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null,meta:null}}));await page.getByRole('button',{name:'确认撤回',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/权限/})).toBeVisible();await expect(page.getByRole('button',{name:'确认撤回',exact:true})).toBeDisabled();expect(calls.filter(c=>c.path===instance+'/lifecycle')).toHaveLength(1);
});
