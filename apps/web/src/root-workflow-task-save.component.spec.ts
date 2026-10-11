import {test,expect,type Page} from '@playwright/test';
// Oracle: FLOW02/06/07/12, V043 frozen PATCH/receipt and V075 P3.
const id=(n:number)=>'55555555-5555-4555-8555-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'finance'};
const loc={appId:id(2),viewId:id(3),recordId:id(4),instanceId:id(5),taskId:id(6)};
const route='/app/approvals/'+[loc.appId,loc.viewId,loc.recordId,loc.instanceId,loc.taskId].join('/');
const api=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}/workflow-instances/${loc.instanceId}/tasks/${loc.taskId}`;
const money={id:id(8),name:'报销金额',kind:'money',required:true,default:null,config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const reason={id:id(9),name:'事由',kind:'text',required:false,default:null,config:{maxLength:null},presentation:{helpText:null,displayTimeZone:null}};
type Options={write?:(body:any,count:number)=>{status:number;code:string;data?:unknown};lookupFails?:boolean;readAfterFails?:boolean};
async function setup(page:Page,options:Options={}){
 const writes:any[]=[],requests:string[]=[];let saved=false,reads=0,changedIdentity=false,amount='100.00';
 const receipt=(operationId:string)=>({operationId,id:loc.recordId,recordVersion:2,schemaVersion:1,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z'});
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname;requests.push(path);let data:unknown;
  if(path.endsWith('/sessions/current')){data=changedIdentity?{id:id(99),account:'other'}:actor;}
  else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path===api){
   reads++;
   if(saved&&options.readAfterFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
   data={basisToken:(saved?'c':'b').repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(7),activationEpoch:1,sequence:0},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(10),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:saved?2:1,schemaVersion:1,values:{[money.id]:amount,[reason.id]:'原事由'},referenceDisplays:{}},fields:[money,reason],editableFieldIds:[money.id]};
  }else if(path===api+'/record'){
   const body=req.postDataJSON();writes.push({body,method:req.method(),path,headers:req.headers()});
   const reply=options.write?.(body,writes.length)??{status:200,code:'OK',data:receipt(body.operationId)};
   if(reply.status===200&&reply.code==='OK'){saved=true;amount=body.changes[money.id];}
   return r.fulfill({status:reply.status,json:{code:reply.code,data:reply.data??null,meta:null}});
  }else if(path.startsWith('/api/v1/application-operations/')){
   if(options.lookupFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
   data={operationId:path.split('/').at(-1),status:'confirmed',httpStatus:200,location:null,result:receipt(path.split('/').at(-1)!)};
  }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null,message:''}});
 });
 return {writes,requests,get reads(){return reads;},changeIdentity(){changedIdentity=true;},receipt};
}
async function edit(page:Page,value='200.00'){await page.goto(route);await expect(page.getByLabel('报销金额')).toBeEditable();await page.getByLabel('报销金额').fill(value);}
test('Root task Save uses current field mask and exact PATCH then refreshes basis without approval',async({page})=>{
 const f=await setup(page);await edit(page,'123456789012345678901234567890.12');
 await expect(page.getByLabel('事由')).not.toBeEditable();await page.getByRole('button',{name:'保存修改',exact:true}).click();
 await expect(page.getByText('修改已保存',{exact:true})).toBeVisible();await expect(page.getByText('最新 v2',{exact:true})).toBeVisible();
 expect(f.writes).toHaveLength(1);expect(f.writes[0].method).toBe('PATCH');expect(f.writes[0].path).toBe(api+'/record');
 expect(f.writes[0].headers['x-expected-actor-id']).toBe(actor.id);
 expect(f.writes[0].body).toEqual({basisToken:'b'.repeat(43),changes:{[money.id]:'123456789012345678901234567890.12'},operationId:expect.stringMatching(/^[0-9a-f-]{36}$/)});
 expect(f.requests.filter(p=>p.endsWith('/actions')||p.includes('workflow-starts'))).toHaveLength(0);expect(f.reads).toBe(2);
});
test('Root task dirty leave requires explicit discard and never writes on departure',async({page})=>{
 const f=await setup(page);await edit(page);await page.getByRole('button',{name:'返回待办',exact:true}).click();
 await expect(page.getByRole('dialog',{name:'有未保存的修改'})).toBeVisible();await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('报销金额')).toHaveValue('200.00');
 await page.getByRole('button',{name:'返回待办',exact:true}).click();await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(/\/app\/approvals$/);expect(f.writes).toHaveLength(0);
});
test('Root task expired basis does not auto-refresh or silently resend edits',async({page})=>{
 const f=await setup(page,{write:()=>({status:409,code:'WORKFLOW_BASIS_EXPIRED'})});await edit(page);await page.getByRole('button',{name:'保存修改',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('审批依据已过期');expect(f.writes).toHaveLength(1);expect(f.reads).toBe(1);await expect(page.getByLabel('报销金额')).toHaveValue('200.00');
 await page.getByRole('button',{name:'刷新预览',exact:true}).click();await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByLabel('报销金额')).toHaveValue('100.00');expect(f.writes).toHaveLength(1);
});
test('Root task unknown Save and failed lookup retain exact original request for retry',async({page})=>{
 let f:Awaited<ReturnType<typeof setup>>;
 f=await setup(page,{lookupFails:true,write:(body,n)=>n===1?{status:503,code:'APPLICATION_OPERATION_UNCONFIRMED'}:{status:200,code:'OK',data:f.receipt(body.operationId)}});
 await edit(page);await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByRole('alert')).toContainText('尚未确认');
 await page.getByRole('button',{name:'核查保存结果',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');expect(f.writes).toHaveLength(1);
 await page.getByRole('button',{name:'重试原保存',exact:true}).click();await expect(page.getByText('修改已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(2);expect(f.writes[1].body).toEqual(f.writes[0].body);
});
test('Root task malformed Save receipt stays unconfirmed and does not reread as success',async({page})=>{
 const f=await setup(page,{write:body=>({status:200,code:'OK',data:{operationId:body.operationId,id:id(99),recordVersion:2,schemaVersion:1,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z'}})});
 await edit(page);await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByRole('alert')).toContainText('尚未确认');await expect(page.getByText('修改已保存',{exact:true})).toHaveCount(0);expect(f.reads).toBe(1);
});
test('Root task confirmed Save remains saved when subsequent preview fails',async({page})=>{
 const f=await setup(page,{readAfterFails:true});await edit(page);await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByText('修改已保存',{exact:true})).toBeVisible();await expect(page.getByRole('alert')).toBeVisible();
 await expect(page.getByRole('button',{name:'保存修改',exact:true})).toHaveCount(0);expect(f.writes).toHaveLength(1);
});
test('Root task two synchronous Save clicks send only one immutable operation',async({page})=>{
 const f=await setup(page);await edit(page);await page.getByRole('button',{name:'保存修改',exact:true}).evaluate(el=>{(el as HTMLButtonElement).click();(el as HTMLButtonElement).click();});
 await expect(page.getByText('修改已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);
});
test('Root task identity preflight change prevents Save transmission',async({page})=>{
 const f=await setup(page);await edit(page);f.changeIdentity();await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByRole('alert')).toContainText('登录身份已变化');expect(f.writes).toHaveLength(0);
});
