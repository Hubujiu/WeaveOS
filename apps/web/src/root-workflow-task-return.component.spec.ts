import {test,expect,type Page} from '@playwright/test';
// V045 current actor/actual visited target qualification; V075 P25 original reasons.
// Figma484:3324 task decision row and487:5645 return dialog, read before declaration.
const id=(n:number)=>'56565656-5656-4565-8565-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reviewer'},loc={appId:id(2),viewId:id(3),recordId:id(4),instanceId:id(5),taskId:id(6)};
const route='/app/approvals/'+Object.values(loc).join('/'),base=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}`,instance=base+'/workflow-instances/'+loc.instanceId,task=instance+'/tasks/'+loc.taskId;
const field={id:id(8),name:'金额',kind:'money',required:true,default:null,config:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const record={id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(9),createdBy:id(10),createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:2,schemaVersion:1,values:{[field.id]:'100.00'},referenceDisplays:{}};
async function setup(page:Page,denied=false){
 const calls:{path:string;method:string;body:any}[]=[];let operationId='';
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname,method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
  if(path==='/api/v1/sessions/current')data=actor;
  else if(path==='/api/v1/me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path===task)data={basisToken:'b'.repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(7),activationEpoch:2,sequence:2},record,fields:[field],editableFieldIds:[field.id]};
  else if(path===base+'/workflow-events')return r.fulfill({json:{code:'OK',data:{items:[]},meta:{pagination:{hasMore:false,nextPageToken:null}}}});
  else if(path===instance+'/lifecycle'){
   if(denied)return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   data={basisToken:'separate-lifecycle-basis',instance:{id:loc.instanceId,sequence:2,state:'active'},record,fields:[field],canWithdraw:false,returnTargets:[{nodeId:id(11),taskId:id(12),activationEpoch:1}]};
  }else if(path===instance+'/actions'){
   operationId=body.operationId;return r.fulfill({status:202,headers:{Location:'/api/v1/application-workflow-operations/'+operationId},json:{code:'OK',data:{operationId,commandId:id(13),instanceId:loc.instanceId,status:'pending'},meta:null}});
  }else if(path==='/api/v1/application-workflow-operations/'+operationId)data={operationId,commandId:id(13),instanceId:loc.instanceId,status:'success',instanceState:'active',sequence:3};
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null}});
 });return calls;
}
async function open(page:Page){await page.goto(route);await expect(page.getByLabel('金额',{exact:true})).toHaveValue('100.00');}
test('Root task return reads current dedicated qualification on demand and submits original target and reason',async({page})=>{
 const calls=await setup(page);await open(page);expect(calls.filter(c=>c.path===instance+'/lifecycle')).toHaveLength(0);await page.getByRole('button',{name:'退回节点',exact:true}).click();const dialog=page.getByRole('dialog',{name:'退回到节点',exact:true});await expect(dialog).toBeVisible();await dialog.getByLabel('目标节点',{exact:true}).selectOption(id(11));const reason='  退回核对 🦊  ';await dialog.getByLabel('退回原因',{exact:false}).fill(reason);await dialog.getByRole('button',{name:'确认退回',exact:true}).click();await expect(dialog).toContainText('退回处理中');const writes=calls.filter(c=>c.method!=='GET');expect(writes).toHaveLength(1);expect(writes[0]).toEqual({path:instance+'/actions',method:'POST',body:{operationId:expect.any(String),action:'return',basisToken:'separate-lifecycle-basis',targetNodeId:id(11),reason}});await dialog.getByRole('button',{name:'核查退回结果',exact:true}).click();await expect(dialog.getByText('已退回',{exact:true})).toBeVisible();expect(calls.filter(c=>c.method!=='GET')).toHaveLength(1);
});
test('Root task return never auto-saves or discards local unsaved field changes',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByLabel('金额',{exact:true}).fill('200.00');await page.getByRole('button',{name:'退回节点',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未保存');await expect(page.getByLabel('金额',{exact:true})).toHaveValue('200.00');expect(calls.filter(c=>c.path===instance+'/lifecycle'||c.method!=='GET')).toEqual([]);await expect(page.getByRole('dialog',{name:'退回到节点',exact:true})).toHaveCount(0);
});
test('Root task return denied current qualification cannot expose targets or submit',async({page})=>{
 const calls=await setup(page,true);await open(page);await page.getByRole('button',{name:'退回节点',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('option',{name:'节点 '+id(11),exact:true})).toHaveCount(0);expect(calls.filter(c=>c.method!=='GET')).toEqual([]);
});
