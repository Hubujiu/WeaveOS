import {test,expect,type Page} from '@playwright/test';
// V037 P3 / FLOW02,07,12 / V075 P4. The HTTP boundary is mocked, not the UI.
const id=(n:number)=>'88888888-8888-4888-8888-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reviewer'},loc={appId:id(2),viewId:id(3),recordId:id(4),instanceId:id(5),taskId:id(6)};
const route='/app/approvals/'+Object.values(loc).join('/');
const api=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}/workflow-instances/${loc.instanceId}/tasks/${loc.taskId}`;
const field={id:id(8),name:'报销金额',kind:'money',required:true,default:null,config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
type Reply={status:number;code:string;data?:unknown;location?:string};
async function setup(page:Page,options:{reply?:(body:any,count:number)=>Reply;lookupFails?:boolean;lookup?:(operationId:string)=>unknown}={}){
 const actions:any[]=[],paths:string[]=[];let terminal:'pending'|'success'|'no_effect'='pending',changed=false,reads=0;
 const result=(operationId:string)=>({operationId,commandId:id(20),instanceId:loc.instanceId,status:terminal,...(terminal==='success'?{instanceState:'completed',sequence:1}:terminal==='no_effect'?{instanceState:'unchanged',sequence:0,reason:'task_inactive'}:{})});
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname;paths.push(path);let data:unknown;
  // Independently authorized empty history is unrelated to this fixture's task/action oracle.
  if(path.endsWith('/workflow-events'))return r.fulfill({json:{code:'OK',data:{items:[]},meta:{pagination:{hasMore:false,nextPageToken:null}},message:''}});
  if(path.endsWith('/sessions/current'))data=changed?{id:id(99),account:'other'}:actor;
  else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path===api){reads++;data={basisToken:'b'.repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(7),activationEpoch:1,sequence:0},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(9),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:1,schemaVersion:1,values:{[field.id]:'100.00'},referenceDisplays:{}},fields:[field],editableFieldIds:[field.id]};}
  else if(path===api+'/actions'){
   const body=req.postDataJSON();actions.push({body,method:req.method(),headers:req.headers()});
   const reply=options.reply?.(body,actions.length)??{status:202,code:'OK',data:result(body.operationId),location:'/api/v1/application-workflow-operations/'+body.operationId};
   return r.fulfill({status:reply.status,headers:reply.location?{Location:reply.location}:{},json:{code:reply.code,data:reply.data??null,meta:null}});
  }else if(path.startsWith('/api/v1/application-workflow-operations/')){
   if(options.lookupFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
   data=options.lookup?options.lookup(path.split('/').at(-1)!):result(path.split('/').at(-1)!);
  }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null,message:''}});
 });
 return {actions,paths,result,finish(value:'success'|'no_effect'='success'){terminal=value;},changeIdentity(){changed=true;},get reads(){return reads;}};
}
async function open(page:Page){await page.goto(route);await expect(page.getByLabel('报销金额')).toHaveValue('100.00');}
test('Root Agree keeps 202 pending distinct and confirms only the dedicated execution result',async({page},testInfo)=>{
 const f=await setup(page);await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();
 await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();await expect(page.getByText('审批已同意',{exact:true})).toHaveCount(0);
 await testInfo.attach('task-approval-pending',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
 expect(f.actions).toHaveLength(1);expect(f.actions[0].body).toEqual({operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),action:'agree',basisToken:'b'.repeat(43)});expect(f.actions[0].method).toBe('POST');expect(f.actions[0].headers['x-expected-actor-id']).toBe(actor.id);
 f.finish();await page.getByRole('button',{name:'核查审批结果',exact:true}).click();await expect(page.getByText('审批已同意',{exact:true})).toBeVisible();
 await testInfo.attach('task-approval-completed',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
 expect(f.paths.filter(p=>p.startsWith('/api/v1/application-operations/'))).toHaveLength(0);expect(f.actions).toHaveLength(1);
});
test('Root Agree no-effect is a terminal non-success rather than approval',async({page})=>{
 const f=await setup(page);await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();
 f.finish('no_effect');await page.getByRole('button',{name:'核查审批结果',exact:true}).click();await expect(page.getByText('操作未执行',{exact:true})).toBeVisible();await expect(page.getByText('审批已同意',{exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(1);
});
test('Root Agree unknown response and failed lookup preserve the original operation on retry',async({page})=>{
 let f:Awaited<ReturnType<typeof setup>>;f=await setup(page,{lookupFails:true,reply:(body,n)=>n===1?{status:503,code:'APPLICATION_OPERATION_UNCONFIRMED'}:{status:202,code:'OK',data:f.result(body.operationId),location:'/api/v1/application-workflow-operations/'+body.operationId}});
 await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');
 await page.getByRole('button',{name:'核查审批结果',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');expect(f.actions).toHaveLength(1);
 await page.getByRole('button',{name:'重试原审批',exact:true}).click();await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();expect(f.actions).toHaveLength(2);expect(f.actions[1].body).toEqual(f.actions[0].body);
});
test('Root Agree refuses a success receipt belonging to another instance',async({page})=>{
 const f=await setup(page,{reply:body=>({status:200,code:'OK',data:{operationId:body.operationId,commandId:id(20),instanceId:id(99),status:'success',instanceState:'completed',sequence:1}})});
 await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');await expect(page.getByText('审批已同意',{exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(1);
});
test('Root Agree stale basis requires explicit refresh without auto-resubmission',async({page})=>{
 const f=await setup(page,{reply:()=>({status:409,code:'WORKFLOW_BASIS_CHANGED'})});await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('审批数据已变化');expect(f.reads).toBe(1);await page.getByRole('button',{name:'刷新预览',exact:true}).click();await expect.poll(()=>f.reads).toBe(2);expect(f.actions).toHaveLength(1);
});
test('Root Agree never auto-saves or approves a dirty form',async({page})=>{
 const f=await setup(page);await open(page);await page.getByLabel('报销金额').fill('200.00');await page.getByRole('button',{name:'同意',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('未保存');expect(f.actions).toHaveLength(0);expect(f.paths.filter(p=>p===api+'/record')).toHaveLength(0);await expect(page.getByLabel('报销金额')).toHaveValue('200.00');
});
test('Root Agree two synchronous clicks keep one immutable command request',async({page})=>{
 const f=await setup(page);await open(page);await page.getByRole('button',{name:'同意',exact:true}).evaluate(el=>{(el as HTMLButtonElement).click();(el as HTMLButtonElement).click();});await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();expect(f.actions).toHaveLength(1);
});
test('Root Agree identity preflight changes never transmit the action',async({page})=>{
 const f=await setup(page);await open(page);f.changeIdentity();await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByRole('alert')).toContainText('登录身份已变化');expect(f.actions).toHaveLength(0);
});
test('Root Agree rejects a changed command ID when querying an accepted operation',async({page})=>{
 const f=await setup(page,{lookup:operationId=>({operationId,commandId:id(21),instanceId:loc.instanceId,status:'success',instanceState:'completed',sequence:1})});
 await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'核查审批结果',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');await expect(page.getByText('审批已同意',{exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(1);
});
test('Root Agree never follows an external pending Location or treats it as acceptance',async({page})=>{
 let f:Awaited<ReturnType<typeof setup>>;f=await setup(page,{reply:body=>({status:202,code:'OK',data:f.result(body.operationId),location:'https://example.invalid/api/v1/application-workflow-operations/'+body.operationId})});
 await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');await expect(page.getByText('审批处理中',{exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(1);
});
test('Root accepted Agree locks editing and guards browser refresh until a terminal receipt',async({page})=>{
 const f=await setup(page);await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();
 await expect(page.getByLabel('报销金额')).toBeDisabled();await expect(page.getByRole('button',{name:'保存修改',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'同意',exact:true})).toBeDisabled();
 expect(await page.evaluate(()=>{const event=new Event('beforeunload',{cancelable:true});window.dispatchEvent(event);return event.defaultPrevented;})).toBe(true);
 f.finish();await page.getByRole('button',{name:'核查审批结果',exact:true}).click();await expect(page.getByText('审批已同意',{exact:true})).toBeVisible();
 expect(await page.evaluate(()=>{const event=new Event('beforeunload',{cancelable:true});window.dispatchEvent(event);return event.defaultPrevented;})).toBe(false);expect(f.actions).toHaveLength(1);
});
test('Root Agree definite access revocation hides the old record instead of leaving usable controls',async({page})=>{
 const f=await setup(page,{reply:()=>({status:403,code:'APPLICATION_FORBIDDEN'})});await open(page);await page.getByRole('button',{name:'同意',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('权限');await expect(page.getByLabel('报销金额')).toHaveCount(0);await expect(page.getByRole('button',{name:'同意',exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(1);expect(f.reads).toBe(1);
});

// P25/FLOW required original reason; Figma487:5862 inspected 2026-10-15.
test('Root Reject requires nonblank at most 500 Unicode scalars and sends original text',async({page},testInfo)=>{
 const f=await setup(page);await open(page);await page.getByRole('button',{name:'驳回',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'驳回申请',exact:true}),input=dialog.getByLabel('驳回原因',{exact:false}),confirm=dialog.getByRole('button',{name:'确认驳回',exact:true});
 await expect(dialog).toBeVisible();await expect(confirm).toBeDisabled();await input.fill('　 ');await expect(confirm).toBeDisabled();
 await input.fill('🦊'.repeat(501));await expect(confirm).toBeDisabled();await input.fill('🦊'.repeat(500));await expect(confirm).toBeEnabled();
 const reason='  请补充票据🦊  ';await input.fill(reason);await testInfo.attach('reject-required-original-reason',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});await confirm.click();
 await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();expect(f.actions).toHaveLength(1);expect(f.actions[0].body).toEqual({operationId:expect.stringMatching(/^[0-9a-f-]{36}$/),action:'reject',basisToken:'b'.repeat(43),reason});
});
test('Root Reject cancel and Escape send nothing and restore initiating focus',async({page})=>{
 const f=await setup(page);await open(page);const trigger=page.getByRole('button',{name:'驳回',exact:true});await trigger.click();let dialog=page.getByRole('dialog',{name:'驳回申请',exact:true});await dialog.getByLabel('驳回原因',{exact:false}).fill('不提交');await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(dialog).toHaveCount(0);await expect(trigger).toBeFocused();expect(f.actions).toHaveLength(0);
 await trigger.click();dialog=page.getByRole('dialog',{name:'驳回申请',exact:true});await expect(dialog).toBeVisible();await page.keyboard.press('Escape');await expect(dialog).toHaveCount(0);await expect(trigger).toBeFocused();expect(f.actions).toHaveLength(0);
});
test('Root Reject unknown retry keeps original reason and operation key',async({page})=>{
 let f:Awaited<ReturnType<typeof setup>>;f=await setup(page,{lookupFails:true,reply:(body,n)=>n===1?{status:503,code:'APPLICATION_OPERATION_UNCONFIRMED'}:{status:202,code:'OK',data:f.result(body.operationId),location:'/api/v1/application-workflow-operations/'+body.operationId}});
 await open(page);await page.getByRole('button',{name:'驳回',exact:true}).click();const dialog=page.getByRole('dialog',{name:'驳回申请',exact:true});await dialog.getByLabel('驳回原因',{exact:false}).fill('  原始原因🦊  ');await dialog.getByRole('button',{name:'确认驳回',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');expect(f.actions).toHaveLength(1);
 await page.getByRole('button',{name:'重试原审批',exact:true}).click();await expect(page.getByText('审批处理中',{exact:true})).toBeVisible();expect(f.actions).toHaveLength(2);expect(f.actions[1].body).toEqual(f.actions[0].body);
});
test('Root Reject never silently saves a dirty task form',async({page})=>{
 const f=await setup(page);await open(page);await page.getByLabel('报销金额').fill('200.00');await page.getByRole('button',{name:'驳回',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未保存');await expect(page.getByRole('dialog',{name:'驳回申请',exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(0);expect(f.paths.filter(p=>p===api+'/record')).toHaveLength(0);await expect(page.getByLabel('报销金额')).toHaveValue('200.00');
});
test('Root Reject revoked write clears task values and does not report success',async({page})=>{
 const f=await setup(page,{reply:()=>({status:403,code:'AUTH_FORBIDDEN'})});await open(page);await page.getByRole('button',{name:'驳回',exact:true}).click();const dialog=page.getByRole('dialog',{name:'驳回申请',exact:true});await dialog.getByLabel('驳回原因',{exact:false}).fill('原始原因');await dialog.getByRole('button',{name:'确认驳回',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByLabel('报销金额')).toHaveCount(0);await expect(page.getByText('审批已驳回',{exact:true})).toHaveCount(0);expect(f.actions).toHaveLength(1);
});
