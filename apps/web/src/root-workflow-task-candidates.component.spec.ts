import {test,expect,type Page} from '@playwright/test';
// V075 P5: current task-scoped enumeration, no ordinary candidate fallback.
const id=(n:number)=>'77777777-7777-4777-8777-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reviewer'},loc={appId:id(2),viewId:id(3),recordId:id(4),instanceId:id(5),taskId:id(6)};
const route='/app/approvals/'+Object.values(loc).join('/');
const api=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}/workflow-instances/${loc.instanceId}/tasks/${loc.taskId}`;
async function setup(page:Page,options:{kind?:'member'|'department';readonly?:boolean;denied?:boolean;malformed?:boolean;nextDenied?:boolean}={}){
 const kind=options.kind??'member',field={id:id(8),name:kind==='member'?'经办人':'所属部门',kind,required:false,default:null,config:{},presentation:{helpText:null,displayTimeZone:null}};
 const candidates:URL[]=[],writes:any[]=[],paths:string[]=[];let selected=id(10),version=1;
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),url=new URL(req.url()),path=url.pathname;paths.push(path);let data:unknown,meta:unknown=null;
  // Independently authorized empty history is unrelated to this fixture's task/action oracle.
  if(path.endsWith('/workflow-events'))return r.fulfill({json:{code:'OK',data:{items:[]},meta:{pagination:{hasMore:false,nextPageToken:null}},message:''}});
  if(path.endsWith('/sessions/current'))data=actor;
  else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path===api)data={basisToken:'b'.repeat(43),task:{id:loc.taskId,instanceId:loc.instanceId,nodeId:id(7),activationEpoch:1,sequence:0},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(9),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:version,schemaVersion:1,values:{[field.id]:selected},referenceDisplays:{[field.id]:{[selected]:{id:selected,label:selected===id(10)?'原引用':'新引用',deleted:selected===id(10)}}}},fields:[field],editableFieldIds:options.readonly?[]:[field.id]};
  else if(path===api+'/reference-candidates'){
   candidates.push(url);
   if(options.denied||options.nextDenied&&url.searchParams.has('pageToken'))return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null,meta:null}});
   const more=!url.searchParams.has('pageToken')&&!url.searchParams.get('q');
   const item={id:more?id(11):id(12),label:more?'第一页候选':'新引用',status:'active',...(kind==='department'?{parentId:null}:{})};
   data={items:[options.malformed?{...item,passwordHash:'must-never-render'}:item]};meta={pagination:{nextPageToken:more?'c'.repeat(43):null,hasMore:more}};
  }else if(path===api+'/record'){
   const body=req.postDataJSON();writes.push(body);selected=body.changes[field.id];version++;
   data={operationId:body.operationId,id:loc.recordId,recordVersion:version,schemaVersion:1,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z'};
  }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta,message:''}});
 });
 return {field,candidates,writes,paths};
}
for(const kind of ['member','department'] as const){
 test(`Root task ${kind} selector uses its trusted field scope and saves only the selected ID`,async({page})=>{
  const f=await setup(page,{kind});await page.goto(route);await expect(page.getByRole('region',{name:'当前审批数据',exact:true})).toContainText('原引用');
  await page.getByRole('button',{name:kind==='member'?'选择成员':'选择部门',exact:true}).click();
  await expect(page.getByRole('button',{name:'第一页候选',exact:true})).toBeVisible();await page.getByRole('button',{name:'加载更多',exact:true}).click();
  await page.getByRole('button',{name:'新引用',exact:true}).click();await expect(page.getByText('新引用',{exact:true})).toBeVisible();
  expect(f.writes).toHaveLength(0);expect(f.candidates).toHaveLength(2);expect(f.candidates[0].searchParams.get('fieldId')).toBe(f.field.id);expect(f.candidates[0].searchParams.get('pageSize')).toBe('20');expect(f.candidates[0].searchParams.has('action')).toBe(false);expect(f.candidates[1].searchParams.get('pageToken')).toBe('c'.repeat(43));
  await page.getByRole('button',{name:'保存修改',exact:true}).click();await expect(page.getByText('修改已保存',{exact:true})).toBeVisible();
  expect(f.writes).toHaveLength(1);expect(f.writes[0].changes).toEqual({[f.field.id]:id(12)});expect(f.paths.filter(p=>p.endsWith('/reference-candidates')&&p!==api+'/reference-candidates')).toEqual([]);
 });
}
test('Root readonly task reference has no selector and does not enumerate a directory',async({page})=>{
 const f=await setup(page,{readonly:true});await page.goto(route);await expect(page.getByText('原引用（已删除）',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'选择成员',exact:true})).toHaveCount(0);expect(f.candidates).toHaveLength(0);
});
test('Root task candidate search starts a new page without the previous cursor',async({page})=>{
 const f=await setup(page);await page.goto(route);await page.getByRole('button',{name:'选择成员',exact:true}).click();await expect(page.getByRole('button',{name:'第一页候选',exact:true})).toBeVisible();
 await page.getByLabel('搜索成员',{exact:true}).fill('新');await expect(page.getByRole('button',{name:'新引用',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'第一页候选',exact:true})).toHaveCount(0);
 expect(f.candidates.at(-1)?.searchParams.get('q')).toBe('新');expect(f.candidates.at(-1)?.searchParams.has('pageToken')).toBe(false);expect(f.writes).toHaveLength(0);
});
for(const failure of ['denied','malformed'] as const){
 test(`Root task candidate ${failure} response never presents selectable items or a successful empty page`,async({page})=>{
  const f=await setup(page,{[failure]:true});await page.goto(route);await page.getByRole('button',{name:'选择成员',exact:true}).click();await expect(page.getByRole('alert')).toContainText('候选加载失败');await expect(page.getByRole('button',{name:'第一页候选',exact:true})).toHaveCount(0);await expect(page.getByText('无可选成员',{exact:true})).toHaveCount(0);expect(f.writes).toHaveLength(0);
 });
}

test('Root task revoked candidate access on the next page clears all previously selectable items',async({page})=>{
 const f=await setup(page,{nextDenied:true});await page.goto(route);await page.getByRole('button',{name:'选择成员',exact:true}).click();await expect(page.getByRole('button',{name:'第一页候选',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'加载更多',exact:true}).click();await expect(page.getByRole('alert')).toContainText('候选加载失败');await expect(page.getByRole('button',{name:'第一页候选',exact:true})).toHaveCount(0);expect(f.writes).toHaveLength(0);
});
