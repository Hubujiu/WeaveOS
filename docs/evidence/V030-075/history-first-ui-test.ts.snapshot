import {test,expect,type Page} from '@playwright/test';
// V062/V063/V064 final 14-field summary, V075 P6. API mocks isolate UI only.
const id=(n:number)=>'99999999-9999-4999-8999-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reviewer'},loc={appId:id(2),viewId:id(3),recordId:id(4),instanceId:id(5),taskId:id(6)};
const route='/app/approvals/'+Object.values(loc).join('/'),record=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}`,task=`${record}/workflow-instances/${loc.instanceId}/tasks/${loc.taskId}`,history=record+'/workflow-events';
const field={id:id(8),name:'当前金额',kind:'money',required:true,default:null,config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const event=(n=20)=>({id:id(n),instanceId:loc.instanceId,flowId:id(7),flowName:'差旅审批',flowNameSource:'captured',nodeId:id(9),targetNodeId:null,actorId:actor.id,action:'agree',outcome:'success',sequence:1,schemaVersion:1,recordVersion:1,occurredAt:'2026-10-10T09:00:00Z'});
async function setup(page:Page,options:{noEffect?:boolean;legacy?:boolean;unavailable?:boolean;wrongDetail?:boolean;nextDenied?:boolean}={}){
 const requests:URL[]=[],writes:string[]=[];let listCalls=0;
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),url=new URL(req.url()),path=url.pathname;requests.push(url);if(req.method()!=='GET'&&path!== '/api/v1/workflow-tasks/search')writes.push(path);
  let data:unknown,meta:unknown=null;
  if(path.endsWith('/sessions/current'))data=actor;
  else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path===task)data={basisToken:'b'.repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(9),activationEpoch:1,sequence:1},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(10),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:2,schemaVersion:1,values:{[field.id]:'200.00'},referenceDisplays:{}},fields:[field],editableFieldIds:[field.id]};
  else if(path===history){
   listCalls++;const later=url.searchParams.has('pageToken');
   if(later&&options.nextDenied)return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null,meta:null}});
   data={items:[{...event(later?21:20),...(options.noEffect?{action:'reject',outcome:'no_effect',sequence:0}:{}),...(options.legacy?{flowNameSource:'legacy_last_known'}:{})}]};meta={pagination:{hasMore:!later,nextPageToken:later?null:'c'.repeat(43)}};
  }else if(path.startsWith(history+'/'))data={event:options.wrongDetail?event(99):event(Number(path.split('-').at(-1))),basis:{status:options.unavailable?'unavailable':'available',fields:options.unavailable?[]:[{fieldId:field.id,fieldName:'历史金额',fieldKind:'money',value:'123456789012345678901234567890.12',valueLabels:{}}]}};
  else if(path==='/api/v1/workflow-tasks/search')data={items:[],total:0,page:1,pageSize:20,queryVersion:'d'.repeat(43)};
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null,meta:null}});
  return r.fulfill({json:{code:'OK',data,meta,message:''}});
 });
 return {requests,writes,get listCalls(){return listCalls;}};
}
async function open(page:Page){await page.goto(route);await expect(page.getByLabel('当前金额')).toHaveValue('200.00');}
test('Root history reads confirmed record-scoped events and opens precise original evidence only on demand',async({page})=>{
 const f=await setup(page);await open(page);const panel=page.getByRole('region',{name:'审批轨迹',exact:true});await expect(panel).toContainText('差旅审批');await expect(panel).toContainText(actor.id);expect(f.requests.filter(u=>u.pathname.startsWith(history+'/'))).toHaveLength(0);
 await panel.getByRole('button',{name:'查看审批依据',exact:true}).first().click();const basis=page.getByRole('region',{name:'历史审批依据',exact:true});await expect(basis).toContainText('历史金额');await expect(basis).toContainText('123456789012345678901234567890.12');expect(f.requests.some(u=>u.pathname===history+'/'+id(20))).toBe(true);expect(f.writes).toHaveLength(0);
});
test('Root history no-effect rejection is displayed as not executed, never a successful rejection',async({page})=>{
 await setup(page,{noEffect:true});await open(page);const panel=page.getByRole('region',{name:'审批轨迹',exact:true});await expect(panel).toContainText('未执行');await expect(panel).not.toContainText('已驳回');
});
test('Root history legacy flow names are identified as last known rather than captured at event time',async({page})=>{
 await setup(page,{legacy:true});await open(page);await expect(page.getByRole('region',{name:'审批轨迹',exact:true})).toContainText('最后已知名称');
});
test('Root history missing original basis never substitutes current record values',async({page})=>{
 const f=await setup(page,{unavailable:true});await open(page);await page.getByRole('region',{name:'审批轨迹',exact:true}).getByRole('button',{name:'查看审批依据',exact:true}).first().click();const basis=page.getByRole('region',{name:'历史审批依据',exact:true});await expect(basis).toContainText('当时审批依据不可用');await expect(basis).not.toContainText('200.00');expect(f.writes).toHaveLength(0);
});
test('Root history pagination and refresh keep local unsaved edits and never write',async({page})=>{
 const f=await setup(page);await open(page);await page.getByLabel('当前金额').fill('300.00');const panel=page.getByRole('region',{name:'审批轨迹',exact:true});await panel.getByRole('button',{name:'加载更多历史',exact:true}).click();await expect.poll(()=>f.listCalls).toBe(2);
 expect(f.requests.filter(u=>u.pathname===history).at(-1)?.searchParams.get('pageToken')).toBe('c'.repeat(43));await panel.getByRole('button',{name:'刷新历史',exact:true}).click();await expect.poll(()=>f.listCalls).toBe(3);expect(f.requests.filter(u=>u.pathname===history).at(-1)?.searchParams.has('pageToken')).toBe(false);await expect(page.getByLabel('当前金额')).toHaveValue('300.00');expect(f.writes).toHaveLength(0);
});
test('Root history revoked access clears prior history without denying independently authorized task editing',async({page})=>{
 await setup(page,{nextDenied:true});await open(page);const panel=page.getByRole('region',{name:'审批轨迹',exact:true});await expect(panel).toContainText('差旅审批');await panel.getByRole('button',{name:'加载更多历史',exact:true}).click();await expect(panel).toContainText('历史权限');await expect(panel).not.toContainText('差旅审批');await expect(page.getByLabel('当前金额')).toBeEditable();
});
test('Root history mismatched detail refuses the other event values',async({page})=>{
 await setup(page,{wrongDetail:true});await open(page);await page.getByRole('region',{name:'审批轨迹',exact:true}).getByRole('button',{name:'查看审批依据',exact:true}).first().click();const basis=page.getByRole('region',{name:'历史审批依据',exact:true});await expect(basis).toContainText('响应格式不正确');await expect(basis).not.toContainText('123456789012345678901234567890.12');
});
