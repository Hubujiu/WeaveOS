import {test,expect,type Page} from '@playwright/test';
// V044 ADR sixth/seventh slices; real Shell/record path with HTTP boundary fixtures.
const id=(n:number)=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'creator'},appId=id(2),viewId=id(3),recordId=id(4),fieldId=id(5),flowId=id(6),instanceId=id(7);
const form=`applications/${appId}/forms/${viewId}`,record=form+'/records/'+recordId,optionsPath=record+'/workflow-start-options/search',startPath=record+'/workflow-starts';
const app={id:appId,name:'申请管理',ownerUserId:id(99),policyRevision:1};
const runtime={appId,viewId,tableId:id(8),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'事由',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}}],layout:[{id:id(9),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const row={id:recordId,appId,viewId,tableId:id(8),createdBy:actor.id,createdAt:'2026-10-12T00:00:00Z',updatedAt:'2026-10-12T00:00:00Z',recordVersion:5,schemaVersion:1,values:{[fieldId]:'出差申请'},referenceDisplays:{}};
const option={flowId,name:'财务审批',workflowRevision:7,definitionVersion:3,schemaVersion:1,recordVersion:5};
async function setup(page:Page,mode:'normal'|'empty'|'denied'|'leaked'|'changed'|'unknown'|'ignored'|'conflict'|'identity'='normal'){
 const calls:{path:string;method:string;body:any;actor?:string}[]=[];let writes=0,changed=false;
 await page.route('**/api/v1/**',async route=>{
  const request=route.request(),path=new URL(request.url()).pathname.slice(8),body=request.postData()?request.postDataJSON():null;calls.push({path,method:request.method(),body,actor:request.headers()['x-expected-actor-id']});let data:unknown;
  if(path==='sessions/current')data=mode==='identity'&&calls.some(c=>c.path===optionsPath)?{id:id(20),account:'other'}:actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===form+'/runtime')data=runtime;
  else if(path===form+'/records/search')data={items:[row],total:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'records-v1',schemaVersion:1,viewVersion:1};
  else if(path===record)data=row;
  else if(path===record+'/workflow-instances/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'flows-v1'};
  else if(path===optionsPath){
   if(mode==='denied')return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   if(mode==='changed'&&body.page===2){changed=true;return route.fulfill({status:409,json:{code:'APPLICATION_QUERY_CHANGED',data:null}});}
   data={items:mode==='empty'?[]:[{...option,...(mode==='leaked'?{graph:{hidden:'private-graph'}}:{})}],total:mode==='empty'?0:mode==='changed'?21:1,page:body.page,pageSize:body.pageSize,queryVersion:changed?'options-v2':'options-v1'};
  }else if(path===startPath){
   writes++;if(mode==='unknown'&&writes===1)return route.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:{operationId:body.operationId}}});
   if(mode==='conflict')return route.fulfill({status:409,json:{code:'APPLICATION_RECORD_CONFLICT',data:null}});
   return route.fulfill({status:202,headers:{Location:'/api/v1/application-operations/'+body.operationId},json:{code:'OK',data:{operationId:body.operationId,flowId,instanceId,status:mode==='ignored'?'ignored':'accepted',ignored:mode==='ignored'},meta:null}});
  }else if(path.startsWith('application-operations/'))return route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  else return route.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return route.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return calls;
}
async function open(page:Page){await page.goto('/app/'+form);await page.getByRole('button',{name:'打开记录：事由 出差申请',exact:true}).click();await page.getByRole('button',{name:'发起流程',exact:true}).click();const panel=page.getByRole('dialog',{name:'发起流程',exact:true});await expect(panel).toBeVisible();return panel;}
async function choose(page:Page){const panel=await open(page);await panel.getByRole('radio',{name:'财务审批',exact:true}).check();return panel;}
test('Root manual start discovers current record choices only and cancel has no write',async({page})=>{
 const calls=await setup(page);const panel=await open(page);await expect(panel.getByRole('radio',{name:'财务审批',exact:true})).toBeVisible();await panel.getByRole('button',{name:'取消',exact:true}).click();expect(calls.filter(c=>c.path===optionsPath).map(c=>c.body)).toEqual([{page:1,pageSize:20}]);expect(calls.filter(c=>c.path===startPath)).toEqual([]);expect(calls.some(c=>c.path.endsWith('/definition'))).toBe(false);expect(calls.filter(c=>c.path===optionsPath).every(c=>c.actor===actor.id)).toBe(true);
});
test('Root manual start sends exact selected CAS and treats accepted as intent not engine success',async({page})=>{
 const calls=await setup(page);const panel=await choose(page);await panel.getByRole('button',{name:'确认发起',exact:true}).click();await expect(panel.getByRole('status')).toContainText('已接受发起请求');await expect(panel).not.toContainText('发起成功');const writes=calls.filter(c=>c.path===startPath);expect(writes).toHaveLength(1);expect(writes[0].body).toEqual({operationId:expect.any(String),flowId,expectedWorkflowRevision:7,expectedSchemaVersion:1,expectedRecordVersion:5});expect(writes[0].actor).toBe(actor.id);expect(calls.filter(c=>['PATCH','PUT','DELETE'].includes(c.method))).toEqual([]);
});
for(const mode of ['denied','leaked'] as const)test('Root manual start '+mode+' fails closed without fabricated empty eligibility',async({page})=>{await setup(page,mode);const panel=await open(page);await expect(panel.getByRole('alert')).toBeVisible();await expect(panel.getByRole('radio')).toHaveCount(0);await expect(panel).not.toContainText('private-graph');await expect(panel).not.toContainText('暂无可发起的流程');});
test('Root manual start authorized empty choices offers no submission',async({page})=>{await setup(page,'empty');const panel=await open(page);await expect(panel).toContainText('暂无可发起的流程');await expect(panel.getByRole('button',{name:'确认发起',exact:true})).toBeDisabled();});
test('Root manual start changed choices require explicit refresh to first page',async({page})=>{const calls=await setup(page,'changed');const panel=await open(page);await panel.getByRole('button',{name:'下一页',exact:true}).click();await expect(panel.getByRole('alert')).toContainText('已变化');await expect(panel.getByRole('radio')).toHaveCount(0);expect(calls.filter(c=>c.path===optionsPath).at(-1)!.body).toEqual({page:2,pageSize:20,queryVersion:'options-v1'});await panel.getByRole('button',{name:'刷新可发起流程',exact:true}).click();await expect(panel.getByRole('radio')).toHaveCount(1);expect(calls.filter(c=>c.path===optionsPath).at(-1)!.body).toEqual({page:1,pageSize:20});});
test('Root manual start unknown preserves original request through failed lookup and retry',async({page})=>{const calls=await setup(page,'unknown');const panel=await choose(page);await panel.getByRole('button',{name:'确认发起',exact:true}).click();await expect(panel.getByRole('alert')).toContainText('尚未确认');await panel.getByRole('button',{name:'核查发起结果',exact:true}).click();await expect(panel.getByRole('alert')).toContainText('尚未确认');await expect(panel.getByRole('radio')).toBeDisabled();await panel.getByRole('button',{name:'重试原发起',exact:true}).click();await expect(panel.getByRole('status')).toContainText('已接受发起请求');const writes=calls.filter(c=>c.path===startPath);expect(writes).toHaveLength(2);expect(writes[1].body).toEqual(writes[0].body);});
test('Root manual start duplicate running flow is shown as ignored not a new instance',async({page})=>{await setup(page,'ignored');const panel=await choose(page);await panel.getByRole('button',{name:'确认发起',exact:true}).click();await expect(panel.getByRole('status')).toContainText('该流程已在进行中');await expect(panel).not.toContainText('发起成功');});
test('Root manual start stale CAS requires explicit refresh without replacement write',async({page})=>{const calls=await setup(page,'conflict');const panel=await choose(page);await panel.getByRole('button',{name:'确认发起',exact:true}).click();await expect(panel.getByRole('alert')).toContainText('刷新');await expect(panel.getByRole('button',{name:'确认发起',exact:true})).toBeDisabled();expect(calls.filter(c=>c.path===startPath)).toHaveLength(1);});
test('Root manual start identity preflight mismatch does not transmit',async({page})=>{const calls=await setup(page,'identity');const panel=await choose(page);await panel.getByRole('button',{name:'确认发起',exact:true}).click();await expect(page.getByText(/登录身份已变化/)).toBeVisible();expect(calls.filter(c=>c.path===startPath)).toEqual([]);});

test('Root manual unknown intent survives real Shell unmount and reopens only its original operation',async({page})=>{
 const calls=await setup(page,'unknown');const panel=await choose(page);await panel.getByRole('button',{name:'确认发起',exact:true}).click();await expect(panel.getByRole('alert')).toContainText('尚未确认');
 await panel.getByRole('button',{name:'关闭',exact:true}).click();const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(panel).toHaveCount(0);
 await page.getByRole('button',{name:'返回主页',exact:true}).click();await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();await page.goBack();await page.getByRole('button',{name:'打开记录：事由 出差申请',exact:true}).click();await page.getByRole('button',{name:'发起流程',exact:true}).click();
 await expect(panel.getByRole('button',{name:'重试原发起',exact:true})).toBeVisible();await expect(panel.getByRole('button',{name:'确认发起',exact:true})).toBeDisabled();expect(calls.filter(c=>c.path===startPath)).toHaveLength(1);await panel.getByRole('button',{name:'重试原发起',exact:true}).click();await expect(panel.getByRole('status')).toContainText('已接受发起请求');const writes=calls.filter(c=>c.path===startPath);expect(writes).toHaveLength(2);expect(writes[1].body).toEqual(writes[0].body);
});
