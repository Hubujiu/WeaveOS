import {test,expect,type Page} from '@playwright/test';
// V046 + V075 P7: real Shell and record-opening UI, mocked HTTP boundary only.
const id=(n:number)=>'bbbbbbbb-bbbb-4bbb-8bbb-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reader'},appId=id(2),viewId=id(3),fieldId=id(4),recordId=id(5);
const app={id:appId,name:'报销管理',ownerUserId:actor.id,policyRevision:1};
const form=`applications/${appId}/forms/${viewId}`,record=form+'/records/'+recordId,search=record+'/workflow-instances/search';
const runtime={appId,viewId,tableId:id(6),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:false,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq','gt','gte','lt','lte'],sortable:true,quickSearchable:false}}],layout:[{id:id(7),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const item={id:recordId,appId,viewId,tableId:id(6),createdBy:actor.id,createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:1,schemaVersion:1,values:{[fieldId]:'10.00'},referenceDisplays:{}};
// V072/V075 P19; actual525:9714,528:9835,528:10108 designs read before declarations.
async function setup(page:Page,options:{member?:boolean;deleted?:boolean;unknown?:boolean;malformed?:boolean;blocked?:boolean;stale?:boolean}={}){
 const calls:{path:string;method:string;body:any}[]=[];let deleted=!!options.deleted,version=deleted?3:1,denied=false,posts=0;let original:any=null;
 const receipt=(body:any,action:string)=>({operationId:body.operationId,id:recordId,recordVersion:body.expectedRecordVersion+1,schemaVersion:1,deleted:action==='deletion'});
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
  const currentApp={...app,ownerUserId:options.member?id(99):actor.id};
  if(path==='sessions/current')data=actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[currentApp]};
  else if(path==='applications/'+appId)data=currentApp;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===form+'/runtime')data=runtime;
  else if(path===form+'/records/search')data={items:deleted?[]:[{...item,recordVersion:version}],total:deleted?0:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'normal-'+version,schemaVersion:1,viewVersion:1};
  else if(path===record){if(deleted)return r.fulfill({status:404,json:{code:'APPLICATION_NOT_FOUND',data:null}});data={...item,recordVersion:version};}
  else if(path===search)data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'no-flows'};
  else if(path===record+'/lifecycle')data={id:recordId,recordVersion:version,schemaVersion:1,deleted};
  else if(path===form+'/records/deleted/search'){
   if(denied)return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   data={items:deleted?[{id:recordId,recordVersion:version,deletedAt:'2026-10-14T01:10:00Z',...(options.malformed?{values:{secret:'must-not-display'}}:{})}]:[],total:deleted?1:0,page:body.page,pageSize:body.pageSize,queryVersion:'deleted-'+version,schemaVersion:1};
  }else if(path===record+'/deletion'||path===record+'/restoration'){
   posts++;if(options.stale)return r.fulfill({status:409,json:{code:'APPLICATION_RECORD_CONFLICT',data:{currentRecordVersion:2}}});if(options.blocked)return r.fulfill({status:409,json:{code:'APPLICATION_RECORD_FENCED',data:null}});
   original??=structuredClone(body);if(options.unknown&&posts===1)return r.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:{operationId:body.operationId}}});
   data=receipt(body,path.endsWith('/deletion')?'deletion':'restoration');deleted=path.endsWith('/deletion');version=body.expectedRecordVersion+1;
  }else if(path.startsWith('application-operations/')&&original)data={operationId:original.operationId,status:'unknown'};
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null}});
 });return{calls,revoke(){denied=true;}};
}
async function open(page:Page){await page.goto('/app/'+form);await page.getByRole('button',{name:'打开记录：金额 10.00',exact:true}).click();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');}
const writes=(calls:{path:string;method:string;body:any}[])=>calls.filter(c=>c.path===record+'/deletion'||c.path===record+'/restoration');
test('Root record deletion cancel preserves data and returns focus without writing',async({page})=>{
 const f=await setup(page);await open(page);const trigger=page.getByRole('button',{name:'删除记录',exact:true});await trigger.click();const dialog=page.getByRole('dialog',{name:'删除这条记录？',exact:true});await expect(dialog).toContainText('审批历史保留');await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(dialog).toHaveCount(0);await expect(trigger).toBeFocused();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');expect(writes(f.calls)).toEqual([]);
});
test('Root record deletion sends exact original CAS and hides only after confirmation',async({page})=>{
 const f=await setup(page);await open(page);await page.getByRole('button',{name:'删除记录',exact:true}).click();await page.getByRole('dialog',{name:'删除这条记录？',exact:true}).getByRole('button',{name:'确认删除',exact:true}).click();await expect.poll(()=>writes(f.calls).length).toBe(1);expect(writes(f.calls)[0].body).toEqual({operationId:expect.any(String),expectedSchemaVersion:1,expectedRecordVersion:1});await expect(page.getByRole('region',{name:'记录详情',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'打开记录：金额 10.00',exact:true})).toHaveCount(0);
});
test('Root edit-only member gets no deletion or deleted-record management entry',async({page})=>{
 const f=await setup(page,{member:true});await open(page);await expect(page.getByRole('button',{name:'编辑记录',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'删除记录',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'已删除记录',exact:true})).toHaveCount(0);expect(writes(f.calls)).toEqual([]);
});
test('Root deleted list restores minimal identity with exact CAS and no workflow restart',async({page},testInfo)=>{
 const f=await setup(page,{deleted:true});await page.goto('/app/'+form);await page.getByRole('button',{name:'已删除记录',exact:true}).click();await expect(page.getByRole('heading',{name:'已删除记录',exact:true})).toBeVisible();await expect(page.getByText(recordId,{exact:true})).toBeVisible();await expect(page.getByRole('columnheader')).toHaveText(['记录编号','删除时间','记录版本','操作']);await testInfo.attach('deleted-record-minimal-list',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
 await page.getByRole('button',{name:'恢复',exact:true}).click();const dialog=page.getByRole('dialog',{name:'恢复这条记录？',exact:true});await expect(dialog).toContainText('不会自动发起审批');await dialog.getByRole('button',{name:'确认恢复',exact:true}).click();await expect.poll(()=>writes(f.calls).length).toBe(1);expect(writes(f.calls)[0].body).toEqual({operationId:expect.any(String),expectedSchemaVersion:1,expectedRecordVersion:3});await expect(page.getByRole('button',{name:'恢复',exact:true})).toHaveCount(0);expect(f.calls.filter(c=>/start|rounds|workflows.*actions/.test(c.path)&&c.method==='POST')).toEqual([]);
});
test('Root deleted list rejects forbidden extra business values before display',async({page})=>{
 await setup(page,{deleted:true,malformed:true});await page.goto('/app/'+form);await page.getByRole('button',{name:'已删除记录',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText('must-not-display',{exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'恢复',exact:true})).toHaveCount(0);
});
test('Root deleted list revocation clears earlier rows and restoration controls',async({page})=>{
 const f=await setup(page,{deleted:true});await page.goto('/app/'+form);await page.getByRole('button',{name:'已删除记录',exact:true}).click();await expect(page.getByText(recordId,{exact:true})).toBeVisible();f.revoke();await page.getByRole('button',{name:'刷新列表',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText(recordId,{exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'恢复',exact:true})).toHaveCount(0);
});
test('Root unknown record deletion retries the same original operation and CAS',async({page})=>{
 const f=await setup(page,{unknown:true});await open(page);await page.getByRole('button',{name:'删除记录',exact:true}).click();await page.getByRole('dialog',{name:'删除这条记录？',exact:true}).getByRole('button',{name:'确认删除',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');await page.getByRole('button',{name:'重试原操作',exact:true}).click();await expect.poll(()=>writes(f.calls).length).toBe(2);expect(writes(f.calls)[1].body).toEqual(writes(f.calls)[0].body);
});
test('Root in-flight fenced record deletion does not hide its data or claim success',async({page})=>{
 const f=await setup(page,{blocked:true});await open(page);await page.getByRole('button',{name:'删除记录',exact:true}).click();await page.getByRole('dialog',{name:'删除这条记录？',exact:true}).getByRole('button',{name:'确认删除',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');expect(writes(f.calls)).toHaveLength(1);
});

test('Root stale record version keeps visible data and never rebases deletion automatically',async({page})=>{
 const f=await setup(page,{stale:true});await open(page);await page.getByRole('button',{name:'删除记录',exact:true}).click();await page.getByRole('dialog',{name:'删除这条记录？',exact:true}).getByRole('button',{name:'确认删除',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByLabel('金额',{exact:true})).toHaveValue('10.00');expect(writes(f.calls)).toHaveLength(1);expect(writes(f.calls)[0].body.expectedRecordVersion).toBe(1);expect(f.calls.filter(c=>c.path===record&&c.method==='GET')).toHaveLength(1);
});
test('Root unknown deletion can close with protection and recover the original operation',async({page})=>{
 const f=await setup(page,{unknown:true});await open(page);await page.getByRole('button',{name:'删除记录',exact:true}).click();const dialog=page.getByRole('dialog',{name:'删除这条记录？',exact:true});await dialog.getByRole('button',{name:'确认删除',exact:true}).click();await expect(dialog.getByRole('alert')).toContainText('未确认');const first=structuredClone(writes(f.calls)[0].body);
 await dialog.getByRole('button',{name:'关闭弹窗',exact:true}).click();const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('region',{name:'记录详情',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'编辑记录',exact:true})).toBeDisabled();await page.getByRole('button',{name:'核查记录操作',exact:true}).click();await expect(page.getByRole('alert')).toContainText('未确认');await page.getByRole('button',{name:'重试原操作',exact:true}).click();await expect.poll(()=>writes(f.calls).length).toBe(2);expect(writes(f.calls)[1].body).toEqual(first);
});
test('Root deleted list uses source header typography and no data grid separators',async({page},testInfo)=>{
 await setup(page,{deleted:true});await page.goto('/app/'+form);await page.getByRole('button',{name:'已删除记录',exact:true}).click();await expect(page.getByText(recordId,{exact:true})).toBeVisible();const cells=page.getByRole('columnheader');await expect(cells).toHaveCount(4);const style=await cells.first().evaluate(el=>{const s=getComputedStyle(el);return {height:Math.round(el.getBoundingClientRect().height),fontSize:s.fontSize,borderRight:s.borderRightWidth,borderBottom:s.borderBottomWidth};});expect(style).toEqual({height:44,fontSize:'12px',borderRight:'0px',borderBottom:'0px'});await testInfo.attach('deleted-list-final-typography',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});
