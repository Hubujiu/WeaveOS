import {test,expect,type Page} from '@playwright/test';
// V015 §§8.4/8.7/14; V017 draft consumption; Figma483:2701 footer.
const id=(n:number)=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'draft-owner'},appId=id(2),viewId=id(3),fieldId=id(4),tableId=id(5),draftId=id(6),recordId=id(7);
const form=`applications/${appId}/forms/${viewId}`,draftPath=form+'/drafts',recordPath=form+'/records';
const runtime={appId,viewId,tableId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:true,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq','gt','gte','lt','lte'],sortable:true,quickSearchable:false}}],layout:[{id:id(8),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
type Mode='normal'|'unknown'|'read-error'|'conflict'|'edit';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[];let draftVersion=0,draftValue:unknown=null,recordValue='10.00',posts=0;
 const app={id:appId,name:'报销管理',ownerUserId:id(99),policyRevision:1};
 const row=()=>({id:recordId,appId,viewId,tableId,createdBy:actor.id,createdAt:'2026-10-12T09:00:00Z',updatedAt:'2026-10-12T10:00:00Z',recordVersion:9,schemaVersion:1,values:{[fieldId]:recordValue},referenceDisplays:{}});
 const draft=()=>({id:draftId,viewId,tableId,targetRecordId:mode==='edit'?recordId:null,schemaVersion:1,baseRecordVersion:mode==='edit'?9:null,draftVersion,createdAt:'2026-10-12T09:00:00Z',updatedAt:'2026-10-12T10:00:00Z',hasConflicts:false,values:{[fieldId]:draftValue},conflicts:[]});
 await page.route('**/api/v1/**',async r=>{
  const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
  if(path==='sessions/current')data=actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[app]};
  else if(path==='applications/'+appId)data=app;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===form+'/runtime')data=runtime;
  else if(path===recordPath+'/search')data={items:mode==='edit'?[row()]:[],total:mode==='edit'?1:0,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'record-context',schemaVersion:1,viewVersion:1};
  else if(path===draftPath&&method==='POST'){
   posts++;if(mode==='unknown'&&posts===1)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
   draftValue=body.values[fieldId];draftVersion=1;return r.fulfill({status:201,headers:{Location:'/api/v1/'+draftPath+'/'+draftId},json:{code:'OK',data:{operationId:body.operationId,id:draftId,draftVersion},meta:null}});
  }else if(path===draftPath+'/'+draftId&&method==='GET'){
   if(mode==='read-error')return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});data=draft();
  }else if(path===draftPath+'/'+draftId&&method==='PATCH'){
   if(mode==='conflict')return r.fulfill({status:409,json:{code:'APPLICATION_DRAFT_CONFLICT',data:null}});
   if(Object.hasOwn(body.changes,fieldId))draftValue=body.changes[fieldId];draftVersion++;data={operationId:body.operationId,id:draftId,draftVersion};
  }else if(path===recordPath&&method==='POST'){
   recordValue=body.values[fieldId];return r.fulfill({status:201,headers:{Location:'/api/v1/'+recordPath+'/'+recordId},json:{code:'OK',data:{operationId:body.operationId,id:recordId,recordVersion:9,schemaVersion:1,createdAt:row().createdAt,updatedAt:row().updatedAt},meta:null}});
  }else if(path===recordPath+'/'+recordId&&method==='GET')data=row();
  else if(path===recordPath+'/'+recordId+'/workflow-instances/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'no-flows'};
  else if(path.startsWith('application-operations/'))return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return calls;
}
async function create(page:Page){await page.goto('/app/'+form);await page.getByRole('button',{name:'新建记录',exact:true}).click();return page.getByRole('dialog',{name:'新建记录',exact:true});}
const draftWrites=(calls:Awaited<ReturnType<typeof setup>>)=>calls.filter(c=>c.path.startsWith(draftPath)&&['POST','PATCH'].includes(c.method));
test('Root draft saves incomplete required decimal without creating record or starting workflow',async({page})=>{
 const calls=await setup(page);const editor=await create(page);await editor.getByRole('textbox',{name:/^金额/}).fill('-');await editor.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();
 expect(draftWrites(calls)).toHaveLength(1);expect(draftWrites(calls)[0].body).toEqual({operationId:expect.any(String),targetRecordId:null,schemaVersion:1,baseRecordVersion:null,values:{[fieldId]:'-'}});expect(calls.filter(c=>c.path===recordPath&&c.method==='POST'||c.path.endsWith('/workflow-starts'))).toEqual([]);await expect(page.getByRole('textbox',{name:/^金额/})).toHaveValue('-');
});
test('Root saved draft edits use exact draft CAS and preserve explicit empty value',async({page})=>{
 const calls=await setup(page);const editor=await create(page);await editor.getByRole('textbox',{name:/^金额/}).fill('-');await editor.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();await page.getByRole('textbox',{name:/^金额/}).fill('');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect.poll(()=>draftWrites(calls).length).toBe(2);const writes=draftWrites(calls);expect(writes[1].path).toBe(draftPath+'/'+draftId);expect(writes[1].method).toBe('PATCH');expect(writes[1].body).toEqual({operationId:expect.any(String),expectedDraftVersion:1,changes:{[fieldId]:''},removeFieldIds:[]});expect(writes[1].body.operationId).not.toBe(writes[0].body.operationId);
});
test('Root formal submission consumes exact draft reference through ordinary record create',async({page})=>{
 const calls=await setup(page);const editor=await create(page);await editor.getByRole('textbox',{name:/^金额/}).fill('-');await editor.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();await page.getByRole('textbox',{name:/^金额/}).fill('12.00');await page.getByRole('button',{name:'保存记录',exact:true}).click();await expect(page.getByText('记录已保存',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.path===recordPath&&c.method==='POST');expect(writes).toHaveLength(1);expect(writes[0].body).toMatchObject({expectedSchemaVersion:1,values:{[fieldId]:'12.00'},draftRef:{id:draftId,draftVersion:1}});expect(calls.some(c=>c.path.endsWith('/submit')||c.path===draftPath+'/'+draftId&&c.method==='DELETE')).toBe(false);
});
test('Root editing record saves an independent draft without changing formal record',async({page})=>{
 const calls=await setup(page,'edit');await page.goto('/app/'+form);await page.getByRole('button',{name:'打开记录：金额 10.00',exact:true}).click();await page.getByRole('button',{name:'编辑记录',exact:true}).click();await page.getByRole('textbox',{name:/^金额/}).fill('-');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();expect(draftWrites(calls)[0].body).toEqual({operationId:expect.any(String),targetRecordId:recordId,schemaVersion:1,baseRecordVersion:9,values:{[fieldId]:'-'}});expect(calls.filter(c=>c.path===recordPath+'/'+recordId&&c.method==='PATCH')).toEqual([]);
});
test('Root unknown draft retains input locks both writes and retries original request only',async({page})=>{
 const calls=await setup(page,'unknown');const editor=await create(page);await editor.getByRole('textbox',{name:/^金额/}).fill('-');await editor.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿保存结果待确认',{exact:true})).toBeVisible();await expect(page.getByRole('textbox',{name:/^金额/})).toBeDisabled();await expect(page.getByRole('button',{name:'保存草稿',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();await page.getByRole('button',{name:'重试原草稿操作',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();expect(draftWrites(calls)).toHaveLength(2);expect(draftWrites(calls)[1]).toEqual(draftWrites(calls)[0]);
});
test('Root confirmed draft with failed reread is saved and retry cannot write again',async({page})=>{
 const calls=await setup(page,'read-error');const editor=await create(page);await editor.getByRole('textbox',{name:/^金额/}).fill('-');await editor.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();await expect(page.getByRole('alert')).toContainText('无法读取草稿');await page.getByRole('button',{name:'重试读取草稿',exact:true}).click();await expect.poll(()=>calls.filter(c=>c.path===draftPath+'/'+draftId&&c.method==='GET').length).toBe(2);expect(draftWrites(calls)).toHaveLength(1);await expect(page.getByRole('button',{name:'保存草稿',exact:true})).toHaveCount(0);
});
test('Root draft version conflict retains text and blocks replacement write until explicit review',async({page})=>{
 const calls=await setup(page,'conflict');const editor=await create(page);await editor.getByRole('textbox',{name:/^金额/}).fill('-');await editor.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByText('草稿已保存',{exact:true})).toBeVisible();await page.getByRole('textbox',{name:/^金额/}).fill('12.');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByRole('alert')).toContainText('刷新');await expect(page.getByRole('textbox',{name:/^金额/})).toHaveValue('12.');await expect(page.getByRole('button',{name:'保存草稿',exact:true})).toBeDisabled();await expect(page.getByRole('button',{name:'保存记录',exact:true})).toBeDisabled();expect(draftWrites(calls)).toHaveLength(2);
});
