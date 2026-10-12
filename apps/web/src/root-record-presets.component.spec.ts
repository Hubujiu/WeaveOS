import {test,expect,type Page} from '@playwright/test';
const id=(n:number)=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'preset-owner'},appId=id(2),viewId=id(3),amount=id(4),title=id(5),recordId=id(6),presetId=id(7);
const path=`applications/${appId}/forms/${viewId}`,routePath='/app/'+path;
const app={id:appId,name:'私人方案应用',ownerUserId:actor.id,policyRevision:1};
const field=(fieldId:string,name:string,kind:string)=>({id:fieldId,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:kind==='money'?{decimal:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}}:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:kind==='money'?['eq','neq','gt','gte','lt','lte']:['eq','neq'],sortable:kind==='money',quickSearchable:false}});
const runtime={appId,viewId,tableId:appId,schemaVersion:2,viewVersion:1,policyRevision:1,fields:[field(amount,'金额','money'),field(title,'事由','text')],layout:[{id:id(10),kind:'field',fieldId:amount},{id:id(11),kind:'field',fieldId:title}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const filter={operator:'and',children:[{fieldId:amount,operator:'gt',value:'9007199254740993.01'}]};
const preset={id:presetId,appId,viewId,name:'高额方案',version:3,invalid:false,createdAt:'2026-10-10T00:00:00Z',updatedAt:'2026-10-11T00:00:00Z',filter,sort:{fieldId:amount,direction:'desc'},hiddenColumnIds:[title],columnOrder:[amount,'createdAt'],columnWidths:{[amount]:240}};
const row={id:recordId,appId,viewId,tableId:appId,createdBy:actor.id,createdAt:'2026-10-10T00:00:00Z',updatedAt:'2026-10-11T00:00:00Z',recordVersion:1,schemaVersion:2,values:{[amount]:'9999999999999999.99',[title]:'差旅'},referenceDisplays:{}};
async function setup(page:Page,options:{invalid?:boolean;invalidLeak?:boolean;listDenied?:boolean;queryChanged?:boolean;detailInvalid?:boolean;listDelay?:number;writeUnknown?:boolean;writeConflict?:boolean}={}){
 const calls:{path:string;method:string;body:any;actor?:string}[]=[];
 let stored:any=preset,writeCount=0;
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(),p=new URL(req.url()).pathname.slice(8),method=req.method(),body=req.postData()?req.postDataJSON():null;
  calls.push({path:p,method,body,actor:req.headers()['x-expected-actor-id']});let data:unknown;
  const redacted={id:presetId,name:'已失效方案',version:3,invalid:true,reason:'PERMISSION_CHANGED'};
  if(p==='sessions/current')data=actor;
  else if(p==='me/access')data={user:actor,bootstrapAdmin:true,personnelManage:true,identities:[],permissions:[],applications:[]};
  else if(p==='applications')data={items:[app]};
  else if(p===`applications/${appId}`)data=app;
  else if(p===`applications/${appId}/access`)data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(p===path+'/runtime')data=runtime;
  else if(p===path+'/records/search'){
   if(options.queryChanged&&body.filter)return route.fulfill({status:409,json:{code:'APPLICATION_QUERY_CHANGED',data:null}});
   data={items:[row],total:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'preset-query-version',schemaVersion:2,viewVersion:1};
  }else if((p===path+'/table-presets'&&method==='POST')||(p===path+'/table-presets/'+presetId&&method==='PUT')){
   writeCount++;
   if(options.writeConflict)return route.fulfill({status:409,json:{code:'APPLICATION_PRESET_CONFLICT',data:null}});
   if(options.writeUnknown&&writeCount===1)return route.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:null}});
   stored={...preset,name:body.name,filter:body.filter,sort:body.sort,hiddenColumnIds:body.hiddenColumnIds,columnOrder:body.columnOrder,columnWidths:body.columnWidths,version:method==='POST'?1:4};
   return route.fulfill({status:method==='POST'?201:200,headers:method==='POST'?{Location:'/api/v1/'+path+'/table-presets/'+presetId}:{},json:{code:'OK',data:{operationId:body.operationId,id:presetId,version:stored.version},meta:null}});
  }else if(p.startsWith('application-operations/'))return route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  else if(p===path+'/table-presets'){
   if(options.listDelay)await new Promise(resolve=>setTimeout(resolve,options.listDelay));
   if(options.listDenied)return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   data={items:[options.invalidLeak?{...redacted,filter:{secret:'private-operand'}}:options.invalid?redacted:stored]};
  }else if(p===path+'/table-presets/'+presetId)data=options.detailInvalid?redacted:stored;
  else return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  return route.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return calls;
}
async function open(page:Page){await page.goto(routePath);await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();await page.getByRole('button',{name:/^自定义筛选/}).click();}
test('Root private preset list stays scoped and does not apply on opening',async({page})=>{
 const calls=await setup(page);await open(page);await expect(page.getByText('高额方案',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===path+'/table-presets')).toHaveLength(1);expect(calls.filter(c=>c.path.startsWith('personnel/'))).toEqual([]);expect(calls.filter(c=>c.path.startsWith('applications/')).every(c=>c.actor===actor.id)).toBe(true);expect(calls.filter(c=>c.path===path+'/records/search').every(c=>c.body.filter===null)).toBe(true);
});
test('Root applying a private preset rereads live scope and queries with original token before hiding columns',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toBeVisible();const requests=calls.filter(c=>c.path===path+'/records/search');expect(requests.at(-1)!.body).toMatchObject({page:1,filter,sort:preset.sort,queryVersion:'preset-query-version'});expect(calls.filter(c=>c.path===path+'/table-presets/'+presetId)).toHaveLength(1);expect(calls.filter(c=>c.path===path+'/runtime').length).toBeGreaterThanOrEqual(2);await page.keyboard.press('Escape');await expect(page.getByRole('columnheader',{name:'事由',exact:true})).toHaveCount(0);expect(requests.at(-1)!.body).not.toHaveProperty('fields');expect(calls.filter(c=>['PUT','PATCH','DELETE'].includes(c.method))).toEqual([]);await page.screenshot({path:test.info().outputPath('record-private-preset-applied.png'),fullPage:true});
});
test('Root private preset changed query keeps old layout and does not silently refresh or apply',async({page})=>{
 const calls=await setup(page,{queryChanged:true});await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/刷新|变化/})).toBeVisible();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toHaveCount(0);expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(2);await page.keyboard.press('Escape');await expect(page.getByRole('columnheader',{name:'事由',exact:true})).toBeVisible();
});
test('Root invalid private preset is visibly unusable without exposing old conditions',async({page})=>{
 const calls=await setup(page,{invalid:true});await open(page);await expect(page.getByText('已失效方案',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'应用已失效方案',exact:true})).toBeDisabled();expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(1);await expect(page.getByRole('dialog')).not.toContainText(amount);
});
for(const option of ['invalidLeak','listDenied'] as const)test('Root private preset '+option+' fails closed rather than displaying an empty successful list',async({page})=>{
 await setup(page,{[option]:true});await open(page);await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText('暂无自定义筛选',{exact:true})).toHaveCount(0);await expect(page.getByText('private-operand')).toHaveCount(0);
});
test('Root private preset revoked between list and apply never changes the records query',async({page})=>{
 const calls=await setup(page,{detailInvalid:true});await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('alert')).toContainText(/失效|权限/);expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(1);await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toHaveCount(0);
});
async function newPreset(page:Page){
 await page.getByRole('button',{name:'新增筛选',exact:true}).click();const editor=page.getByRole('dialog',{name:'新增自定义筛选',exact:true});await editor.getByLabel('自定义筛选名称').fill('精确金额方案');await editor.getByRole('button',{name:'或条件',exact:true}).click();await editor.getByLabel('条件 1.1 字段').selectOption(amount);await editor.getByLabel('条件 1.1 比较').selectOption('gt');await editor.getByLabel('条件 1.1 值',{exact:true}).fill('9007199254740993.01');return editor;
}
test('Root private preset create sends complete configuration and a minimal operation without applying',async({page})=>{
 const calls=await setup(page);await open(page);const editor=await newPreset(page);await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByText('精确金额方案',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.path===path+'/table-presets'&&c.method==='POST');expect(writes).toHaveLength(1);expect(writes[0].body).toEqual({operationId:expect.any(String),expectedSchemaVersion:2,name:'精确金额方案',filter,sort:null,hiddenColumnIds:[],columnOrder:[],columnWidths:{}});expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(1);await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toHaveCount(0);
});
test('Root private preset edit keeps saved layout and uses exact preset CAS without applying',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByRole('button',{name:'编辑高额方案',exact:true}).click();const editor=page.getByRole('dialog',{name:'编辑自定义筛选',exact:true});await editor.getByLabel('自定义筛选名称').fill('改名方案');await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByText('改名方案',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.method==='PUT');expect(writes).toHaveLength(1);expect(writes[0].body).toEqual({operationId:expect.any(String),expectedSchemaVersion:2,expectedVersion:3,name:'改名方案',filter,sort:preset.sort,hiddenColumnIds:preset.hiddenColumnIds,columnOrder:preset.columnOrder,columnWidths:preset.columnWidths});expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(1);
});
test('Root private preset unknown submission retries the immutable original operation only',async({page})=>{
 const calls=await setup(page,{writeUnknown:true});await open(page);const editor=await newPreset(page);await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/未确认/})).toBeVisible();await page.getByRole('button',{name:'核查方案操作',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/未确认/})).toBeVisible();await page.getByRole('button',{name:'重试原方案操作',exact:true}).click();await expect(page.getByText('精确金额方案',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets');expect(writes).toHaveLength(2);expect(writes[1].body).toEqual(writes[0].body);expect(calls.some(c=>c.path==='application-operations/'+writes[0].body.operationId)).toBe(true);
});
test('Root private preset CAS conflict retains user input without a replacement write',async({page})=>{
 const calls=await setup(page,{writeConflict:true});await open(page);await page.getByRole('button',{name:'编辑高额方案',exact:true}).click();const editor=page.getByRole('dialog',{name:'编辑自定义筛选',exact:true});await editor.getByLabel('自定义筛选名称').fill('保留此名称');await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByRole('alert')).toContainText(/变化|冲突|更新/);await expect(editor.getByLabel('自定义筛选名称')).toHaveValue('保留此名称');expect(calls.filter(c=>c.method==='PUT')).toHaveLength(1);expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(1);
});
