import {test,expect,type Page} from '@playwright/test';
// Independent V037/V043 wire + FLOW02/06 + V075 P2, not inferred from renderer.
const id=(n:number)=>'33333333-3333-4333-8333-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reviewer'};
const loc={appId:id(2),viewId:id(3),recordId:id(4),instanceId:id(5),taskId:id(6)};
const route='/app/approvals/'+[loc.appId,loc.viewId,loc.recordId,loc.instanceId,loc.taskId].join('/');
const api=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}/workflow-instances/${loc.instanceId}/tasks/${loc.taskId}`;
const field={id:id(9),name:'报销金额',kind:'money',required:true,default:null,config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const preview={basisToken:'b'.repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(7),activationEpoch:1,sequence:0},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(8),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:3,schemaVersion:1,values:{[field.id]:'123456789012345678901234567890.12'},referenceDisplays:{}},fields:[field],editableFieldIds:[]};
async function setup(page:Page,result:(n:number)=>{status?:number;code?:string;data?:unknown}){
 const calls:{path:string;headers:Record<string,string>}[]=[];let reads=0;
 await page.route('**/api/v1/**',async r=>{
  const path=new URL(r.request().url()).pathname;calls.push({path,headers:r.request().headers()});let data:unknown;
  if(path.endsWith('/sessions/current'))data=actor;
  else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path===api){const reply=result(++reads);return r.fulfill({status:reply.status??200,json:{code:reply.code??'OK',data:reply.data??null,meta:null,message:''}});}
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null,message:''}});
 });return calls;
}
test('Root task deep link reads current authorized preview and preserves exact decimal strings',async({page})=>{
 const calls=await setup(page,()=>({data:preview}));await page.goto(route);
 await expect(page.getByRole('heading',{name:'审批处理',exact:true})).toBeVisible();
 await expect(page.getByLabel('报销金额',{exact:false})).toHaveValue('123456789012345678901234567890.12');
 await expect(page.getByLabel('报销金额',{exact:false})).not.toBeEditable();
 await expect(page.getByText('最新 v3',{exact:true})).toBeVisible();
 expect(calls.find(c=>c.path===api)?.headers['x-expected-actor-id']).toBe(actor.id);
 expect(page.url()).not.toContain(preview.basisToken);
 const shell=await page.getByTestId('workspace-shell').elementHandle();
 await page.getByRole('button',{name:'返回待办',exact:true}).click();await expect(page).toHaveURL(/\/app\/approvals$/);expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);
});
for(const [name,data] of [
 ['wrong record',{...preview,record:{...preview.record,id:id(99)}}],
 ['wrong task',{...preview,task:{...preview.task,id:id(99)}}],
 ['hidden extra value',{...preview,record:{...preview.record,values:{...preview.record.values,[id(99)]:'secret'}}}],
 ['edit mask outside visible fields',{...preview,editableFieldIds:[id(99)]}],
] as const)test('Root task refuses '+name,async({page})=>{
 await setup(page,()=>({data}));await page.goto(route);await expect(page.getByRole('alert')).toContainText('审批预览响应格式不正确');
 await expect(page.getByLabel('报销金额',{exact:false})).toHaveCount(0);
});
test('Root task closed or revoked state never offers stale approval controls',async({page})=>{
 await setup(page,()=>({status:409,code:'WORKFLOW_TASK_CHANGED'}));await page.goto(route);
 await expect(page.getByRole('alert')).toContainText('任务已变化');await expect(page.getByRole('button',{name:'同意',exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'返回待办',exact:true}).click();await expect(page).toHaveURL(/\/app\/approvals$/);
});
test('Root task explicit refresh rereads current record rather than trusting inbox version',async({page})=>{
 await setup(page,n=>({data:{...preview,record:{...preview.record,recordVersion:n===1?3:4,values:{[field.id]:n===1?'100.00':'200.00'}}}}));await page.goto(route);
 await expect(page.getByLabel('报销金额',{exact:false})).toHaveValue('100.00');await page.getByRole('button',{name:'刷新预览',exact:true}).click();
 await expect(page.getByLabel('报销金额',{exact:false})).toHaveValue('200.00');await expect(page.getByText('最新 v4',{exact:true})).toBeVisible();
});
test('Root task displays authorized tombstones without resolving extra private data',async({page})=>{
 const member={...field,id:id(10),name:'申请成员',kind:'member',required:false,config:{}};
 const option={...field,id:id(11),name:'费用类别',kind:'single_select',required:false,config:{options:[]}};
 const calls=await setup(page,()=>({data:{...preview,fields:[member,option],record:{...preview.record,values:{[member.id]:id(20),[option.id]:id(21)},referenceDisplays:{[member.id]:{[id(20)]:{id:id(20),label:'原成员',deleted:true}},[option.id]:{[id(21)]:{id:id(21),label:'原类别',deleted:true}}}}}}));
 await page.goto(route);await expect(page.getByText('原成员（已删除）',{exact:true})).toBeVisible();await expect(page.getByText('原类别（已删除）',{exact:true})).toBeVisible();
 expect(calls.filter(c=>/candidates|personnel|members/.test(c.path))).toHaveLength(0);
});
