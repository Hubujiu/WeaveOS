import {test,expect,type Page} from '@playwright/test';
test.afterEach(async({page},info)=>{if(info.status!==info.expectedStatus){await info.attach('preset-failure-screen',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});await info.attach('preset-failure-state',{body:JSON.stringify({url:page.url(),body:await page.locator('body').innerText()}),contentType:'application/json'});}});
const id=(n:number)=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'preset-owner'},appId=id(2),viewId=id(3),amount=id(4),title=id(5),recordId=id(6),presetId=id(7);
const path=`applications/${appId}/forms/${viewId}`,routePath='/app/'+path;
const app={id:appId,name:'私人方案应用',ownerUserId:actor.id,policyRevision:1};
const field=(fieldId:string,name:string,kind:string)=>({id:fieldId,name,kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:kind==='money'?{decimal:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}}:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:kind==='money'?['eq','neq','gt','gte','lt','lte']:['eq','neq'],sortable:kind==='money',quickSearchable:false}});
const runtime={appId,viewId,tableId:appId,schemaVersion:2,viewVersion:1,policyRevision:1,fields:[field(amount,'金额','money'),field(title,'事由','text')],layout:[{id:id(10),kind:'field',fieldId:amount},{id:id(11),kind:'field',fieldId:title}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
const filter={operator:'and',children:[{fieldId:amount,operator:'gt',value:'9007199254740993.01'}]};
const preset={id:presetId,appId,viewId,name:'高额方案',version:3,invalid:false,createdAt:'2026-10-10T00:00:00Z',updatedAt:'2026-10-11T00:00:00Z',filter,sort:{fieldId:amount,direction:'desc'},hiddenColumnIds:[title],columnOrder:[amount,'createdAt'],columnWidths:{[amount]:240}};
const row={id:recordId,appId,viewId,tableId:appId,createdBy:actor.id,createdAt:'2026-10-10T00:00:00Z',updatedAt:'2026-10-11T00:00:00Z',recordVersion:1,schemaVersion:2,values:{[amount]:'9999999999999999.99',[title]:'差旅'},referenceDisplays:{}};
async function setup(page:Page,options:{invalid?:boolean;invalidLeak?:boolean;listDenied?:boolean;queryChanged?:boolean;detailInvalid?:boolean;listDelay?:number;writeUnknown?:boolean;writeConflict?:boolean;confirmedReadFails?:boolean;presetOverride?:Partial<typeof preset>}={}){
 const calls:{path:string;method:string;body:any;actor?:string;query:string}[]=[];
 let stored:any={...preset,...options.presetOverride},writeCount=0;
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(),p=new URL(req.url()).pathname.slice(8),method=req.method(),body=req.postData()?req.postDataJSON():null;
  calls.push({path:p,method,body,actor:req.headers()['x-expected-actor-id'],query:new URL(req.url()).search});let data:unknown;
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
  }else if(p===path+'/table-presets/'+presetId&&method==='DELETE'){stored=null;return route.fulfill({status:204,body:''});}
  else if(p.startsWith('application-operations/'))return route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  else if(p===path+'/table-presets'){
   if(options.listDelay)await new Promise(resolve=>setTimeout(resolve,options.listDelay));
   if(options.listDenied)return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
   data={items:stored?[options.invalidLeak?{...redacted,filter:{secret:'private-operand'}}:options.invalid?redacted:stored]:[]};
  }else if(p===path+'/table-presets/'+presetId){if(options.confirmedReadFails&&writeCount>0)return route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});data=options.detailInvalid?redacted:stored;}
  else return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  return route.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return calls;
}
async function open(page:Page){await page.goto(routePath);await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();await expect(page.getByRole('columnheader').filter({has:page.getByText('事由',{exact:true})})).toBeVisible();await page.getByRole('button',{name:/^自定义筛选/}).click();}
test('Root private preset list stays scoped and does not apply on opening',async({page})=>{
 const calls=await setup(page);await open(page);await expect(page.getByText('高额方案',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===path+'/table-presets')).toHaveLength(1);expect(calls.filter(c=>c.path.startsWith('personnel/'))).toEqual([]);expect(calls.filter(c=>c.path.startsWith('applications/')).every(c=>c.actor===actor.id)).toBe(true);expect(calls.filter(c=>c.path===path+'/records/search').every(c=>c.body.filter===null)).toBe(true);
});
test('Root applying a private preset rereads live scope and queries with original token before hiding columns',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toBeVisible();const requests=calls.filter(c=>c.path===path+'/records/search');expect(requests.at(-1)!.body).toMatchObject({page:1,filter,sort:preset.sort,queryVersion:'preset-query-version'});expect(calls.filter(c=>c.path===path+'/table-presets/'+presetId)).toHaveLength(1);expect(calls.filter(c=>c.path===path+'/runtime').length).toBeGreaterThanOrEqual(2);await page.keyboard.press('Escape');await expect(page.getByRole('columnheader').filter({has:page.getByText('事由',{exact:true})})).toHaveCount(0);expect(requests.at(-1)!.body).not.toHaveProperty('fields');expect(calls.filter(c=>['PUT','PATCH','DELETE'].includes(c.method))).toEqual([]);await page.screenshot({path:test.info().outputPath('record-private-preset-applied.png'),fullPage:true});
});
test('Root private preset changed query keeps old layout and does not silently refresh or apply',async({page})=>{
 const calls=await setup(page,{queryChanged:true});await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/刷新|变化/})).toBeVisible();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toHaveCount(0);expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(2);await page.keyboard.press('Escape');await expect(page.getByRole('columnheader').filter({has:page.getByText('事由',{exact:true})})).toBeVisible();
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
test('Root private preset deletion requires confirmation and sends exact version query with no body',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByRole('button',{name:'删除高额方案',exact:true}).click();const confirm=page.getByRole('dialog',{name:'删除筛选方案',exact:true});await expect(confirm).toBeVisible();expect(calls.filter(c=>c.method==='DELETE')).toEqual([]);await confirm.getByRole('button',{name:'返回',exact:true}).click();expect(calls.filter(c=>c.method==='DELETE')).toEqual([]);await page.getByRole('button',{name:'删除高额方案',exact:true}).click();await confirm.getByRole('button',{name:'确认删除',exact:true}).click();await expect(page.getByText('暂无自定义筛选',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.method==='DELETE');expect(writes).toHaveLength(1);expect(writes[0].body).toBeNull();expect(writes[0].path).toBe(path+'/table-presets/'+presetId);expect(Object.fromEntries(new URLSearchParams(writes[0].query))).toEqual({operationId:expect.any(String),expectedVersion:'3'});expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(1);
});
test('Root private preset synchronous double save cannot create two operations',async({page})=>{
 const calls=await setup(page);await open(page);const editor=await newPreset(page);await editor.getByRole('button',{name:'确定',exact:true}).evaluate((button:HTMLButtonElement)=>{button.click();button.click();});await expect(page.getByText('精确金额方案',{exact:true})).toBeVisible();expect(calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets')).toHaveLength(1);
});
test('Root private preset identity preflight rejects a new account before transmitting configuration',async({page})=>{
 const calls=await setup(page);await open(page);const editor=await newPreset(page);let checks=0;await page.route('**/api/v1/sessions/current',r=>{checks++;return r.fulfill({json:{code:'OK',data:{id:id(99),account:'other'},meta:null}});});await editor.getByRole('button',{name:'确定',exact:true}).click();await expect.poll(()=>checks).toBeGreaterThan(0);expect(calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets')).toEqual([]);
});

test('Root unsaved private preset warns browser refresh without writing',async({page})=>{
 const calls=await setup(page);await open(page);await newPreset(page);
 const warned=await page.evaluate(()=>{const event=new Event('beforeunload',{cancelable:true});window.dispatchEvent(event);return event.defaultPrevented;});
 expect(warned).toBe(true);expect(calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets')).toEqual([]);
});
test('Root private preset dirty navigation keeps the draft on continue and leaves only after explicit discard',async({page})=>{
 const calls=await setup(page);await open(page);await newPreset(page);await page.getByRole('button',{name:'返回主页',exact:true}).click();
 const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('自定义筛选名称')).toHaveValue('精确金额方案');
 await page.getByRole('button',{name:'返回主页',exact:true}).click();await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(/\/app$/);expect(calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets')).toEqual([]);
});
test('Root private preset unknown operation blocks browser refresh and keeps editor inputs immutable',async({page})=>{
 await setup(page,{writeUnknown:true});await open(page);const editor=await newPreset(page);await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/未确认/})).toBeVisible();
 await expect(editor.getByLabel('自定义筛选名称')).toBeDisabled();
 const warned=await page.evaluate(()=>{const event=new Event('beforeunload',{cancelable:true});window.dispatchEvent(event);return event.defaultPrevented;});expect(warned).toBe(true);
});


test('Root confirmed preset save with failed current reread remains saved and cannot write again',async({page})=>{
 const calls=await setup(page,{confirmedReadFails:true});await open(page);const editor=await newPreset(page);await editor.getByRole('button',{name:'确定',exact:true}).click();
 await expect(page.getByRole('alert').filter({hasText:'方案已保存'})).toBeVisible();await expect(editor.getByRole('button',{name:'保存中…',exact:true})).toBeDisabled();await expect(editor.getByRole('button',{name:'确定',exact:true})).toHaveCount(0);
 await expect(page.getByRole('button',{name:'重试读取已保存方案',exact:true})).toBeVisible();await page.getByRole('button',{name:'重试读取已保存方案',exact:true}).click();
 await expect(page.getByRole('alert').filter({hasText:'方案已保存'})).toBeVisible();expect(calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets')).toHaveLength(1);
});
test('Root private preset unknown operation survives SPA departure and reopens the original recovery',async({page})=>{
 const calls=await setup(page,{writeUnknown:true});await open(page);const editor=await newPreset(page);await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByRole('alert').filter({hasText:/未确认/})).toBeVisible();
 await page.getByRole('button',{name:'返回主页',exact:true}).click();const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(/\/app$/);await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();await expect(page.getByLabel('自定义筛选名称')).toHaveCount(0);
 await page.goBack();await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();await page.getByRole('button',{name:/^自定义筛选/}).click();await expect(page.getByRole('button',{name:'重试原方案操作',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'重试原方案操作',exact:true}).click();await expect(page.getByText('精确金额方案',{exact:true})).toBeVisible();const writes=calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets');expect(writes).toHaveLength(2);expect(writes[1].body).toEqual(writes[0].body);
});

test('Root private preset dirty refresh requires explicit discard and preserves the editor on continue',async({page})=>{
 const calls=await setup(page);await open(page);await newPreset(page);const runtimeReads=calls.filter(c=>c.path===path+'/runtime').length;await page.getByRole('button',{name:'刷新',exact:true}).evaluate((button:HTMLButtonElement)=>button.click());
 const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('自定义筛选名称')).toHaveValue('精确金额方案');expect(calls.filter(c=>c.path===path+'/runtime')).toHaveLength(runtimeReads);
 await page.getByRole('button',{name:'刷新',exact:true}).evaluate((button:HTMLButtonElement)=>button.click());await guard.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByLabel('自定义筛选名称')).toHaveCount(0);await expect.poll(()=>calls.filter(c=>c.path===path+'/runtime').length).toBe(runtimeReads+1);expect(calls.filter(c=>c.method==='POST'&&c.path===path+'/table-presets')).toEqual([]);
});

test('Root private preset manager matches approved desktop surface and searches the bounded private list',async({page},testInfo)=>{
 await page.setViewportSize({width:1440,height:1000});const calls=await setup(page);await open(page);const panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await expect(panel).toBeVisible();await testInfo.attach('private-preset-manager-before-layout',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
 await expect(panel).toHaveCSS('width','660px');await expect(panel).toHaveCSS('border-radius','16px');
 const search=panel.getByRole('textbox',{name:'搜索筛选',exact:true});await search.fill('无匹配方案');await expect(panel.getByText('高额方案',{exact:true})).toHaveCount(0);await search.fill('高额');await expect(panel.getByText('高额方案',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===path+'/table-presets')).toHaveLength(1);
});
test('Root private preset editor uses approved dimensions and numeric comparison wording',async({page},testInfo)=>{
 await page.setViewportSize({width:1440,height:1000});await setup(page);await open(page);const panel=await newPreset(page);await testInfo.attach('private-preset-editor-before-layout',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
 await expect(panel).toHaveCSS('width','800px');await expect(panel).toHaveCSS('border-radius','16px');await expect(panel.getByLabel('自定义筛选名称')).toHaveCSS('height','42px');await expect(panel.getByLabel('条件 1.1 比较').locator('option[value=gt]')).toHaveText('大于');await expect(panel.getByLabel('条件 1.1 比较').locator('option[value=lt]')).toHaveText('小于');
});
test('Root private preset editor stays within a narrow viewport with usable actions',async({page},testInfo)=>{
 await page.setViewportSize({width:390,height:844});await setup(page);await open(page);const panel=await newPreset(page);await expect(panel.getByRole('button',{name:'确定',exact:true})).toBeInViewport();const box=await panel.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(390);await expect(page.locator('body')).toHaveJSProperty('scrollWidth',390);await testInfo.attach('private-preset-editor-mobile',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});

// V071 ADR: only five display system columns; only the two timestamps filter/sort.
test('Root private preset applies system date conditions and preserves system layout columns',async({page})=>{
 const dateFilter={operator:'and',children:[{fieldId:'createdAt',operator:'gte',value:'2026-10-01T00:00:00Z'}]};
 const calls=await setup(page,{presetOverride:{filter:dateFilter,sort:{fieldId:'updatedAt',direction:'asc'},hiddenColumnIds:['id','createdBy'],columnOrder:['updatedAt',amount,'createdAt'],columnWidths:{updatedAt:220}}});await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toBeVisible();
 expect(calls.filter(c=>c.path===path+'/records/search').at(-1)!.body).toMatchObject({filter:dateFilter,sort:{fieldId:'updatedAt',direction:'asc'},queryVersion:'preset-query-version'});await page.keyboard.press('Escape');
 const header=(name:string)=>page.getByRole('columnheader').filter({has:page.getByText(name,{exact:true})});
 await expect(header('更新时间')).toBeVisible();await expect(header('创建时间')).toBeVisible();await expect(header('记录版本')).toBeVisible();await expect(header('记录 ID')).toHaveCount(0);await expect(header('创建人')).toHaveCount(0);await expect(header('更新人')).toHaveCount(0);
});
test('Root preset editor offers only the two system timestamp filters alongside authorized business fields',async({page})=>{
 await setup(page);await open(page);const editor=await newPreset(page);const choices=editor.getByLabel('条件 1.1 字段');await expect(choices.locator('option')).toHaveText(['请选择字段','金额','事由','创建时间','更新时间']);await choices.selectOption('createdAt');await editor.getByLabel('条件 1.1 比较').selectOption('gte');await editor.getByLabel('条件 1.1 值',{exact:true}).fill('2026-10-01T00:00:00Z');
 await editor.getByLabel('自定义筛选名称').fill('本月记录');await editor.getByRole('button',{name:'确定',exact:true}).click();await expect(page.getByText('本月记录',{exact:true})).toBeVisible();
});

test('Root resource preset list search is available independently of layout dimensions',async({page})=>{
 const calls=await setup(page);await open(page);const panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});const search=panel.getByRole('textbox',{name:'搜索筛选',exact:true});await expect(search).toBeVisible();await search.fill('无匹配方案');await expect(panel.getByText('高额方案',{exact:true})).toHaveCount(0);await search.fill('高额');await expect(panel.getByText('高额方案',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===path+'/table-presets')).toHaveLength(1);
});
test('Root resource numeric operators use numeric wording independently of editor dimensions',async({page})=>{
 await setup(page);await open(page);const panel=await newPreset(page);await expect(panel.getByLabel('条件 1.1 比较').locator('option[value=gt]')).toHaveText('大于');await expect(panel.getByLabel('条件 1.1 比较').locator('option[value=lt]')).toHaveText('小于');
});

// Figma486:4734: separate transient column-display dialog, not a data mutation.
test('Root record display-fields panel applies visibility only after confirmation without changing the query',async({page},testInfo)=>{
 const calls=await setup(page);await open(page);await page.keyboard.press('Escape');const header=(name:string)=>page.getByRole('columnheader').filter({has:page.getByText(name,{exact:true})});await expect(header('事由')).toBeVisible();const searches=calls.filter(c=>c.path===path+'/records/search').length;
 await page.getByRole('button',{name:'显示字段',exact:true}).click();const dialog=page.getByRole('dialog',{name:'显示字段',exact:true});await expect(dialog).toHaveCSS('width','500px');await expect(dialog).toHaveCSS('border-radius','16px');await dialog.getByRole('checkbox',{name:'事由',exact:true}).uncheck();await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(header('事由')).toBeVisible();
 await page.getByRole('button',{name:'显示字段',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'事由',exact:true})).toBeChecked();await dialog.getByRole('checkbox',{name:'事由',exact:true}).uncheck();await testInfo.attach('record-display-fields',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});await dialog.getByRole('button',{name:'应用',exact:true}).click();await expect(header('事由')).toHaveCount(0);await expect(header('金额')).toBeVisible();expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(searches);expect(calls.filter(c=>c.path.includes('/table-presets')&&c.method!=='GET')).toEqual([]);
});
test('Root display-fields keeps one actual business column even while all system columns are visible',async({page})=>{
 await setup(page);await open(page);await page.keyboard.press('Escape');await page.getByRole('button',{name:'显示字段',exact:true}).click();const dialog=page.getByRole('dialog',{name:'显示字段',exact:true});await dialog.getByRole('checkbox',{name:'事由',exact:true}).uncheck();await expect(dialog.getByRole('checkbox',{name:'金额',exact:true})).toBeChecked();await expect(dialog.getByRole('checkbox',{name:'金额',exact:true})).toBeDisabled();await expect(dialog.getByRole('checkbox',{name:'记录 ID',exact:true})).toBeEnabled();await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(page.getByRole('button',{name:'显示字段',exact:true})).toBeFocused();
});
test('Root display-fields leaves active filter and selected record unchanged',async({page})=>{
 const calls=await setup(page);await open(page);await page.getByRole('button',{name:'应用高额方案',exact:true}).click();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toBeVisible();await page.keyboard.press('Escape');await page.getByRole('checkbox',{name:'选择记录：'+recordId,exact:true}).check();const searches=calls.filter(c=>c.path===path+'/records/search').length;
 await page.getByRole('button',{name:'显示字段',exact:true}).click();const dialog=page.getByRole('dialog',{name:'显示字段',exact:true});await dialog.getByRole('checkbox',{name:'创建时间',exact:true}).uncheck();await dialog.getByRole('button',{name:'应用',exact:true}).click();await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toBeVisible();await expect(page.getByRole('checkbox',{name:'选择记录：'+recordId,exact:true})).toBeChecked();expect(calls.filter(c=>c.path===path+'/records/search')).toHaveLength(searches);
});
test('Root display-fields panel remains bounded with visible controls on narrow screens',async({page})=>{
 await page.setViewportSize({width:390,height:844});await setup(page);await open(page);await page.keyboard.press('Escape');await page.getByRole('button',{name:'显示字段',exact:true}).click();const dialog=page.getByRole('dialog',{name:'显示字段',exact:true}),box=await dialog.boundingBox();expect(box!.x).toBeGreaterThanOrEqual(0);expect(box!.x+box!.width).toBeLessThanOrEqual(390);await expect(dialog.getByRole('button',{name:'应用',exact:true})).toBeInViewport();
});

test('Root display-fields typography uses approved monochrome dialog heading',async({page})=>{
 await setup(page);await open(page);await page.keyboard.press('Escape');await page.getByRole('button',{name:'显示字段',exact:true}).click();const dialog=page.getByRole('dialog',{name:'显示字段',exact:true});await expect(dialog.getByRole('heading',{name:'显示字段',exact:true})).toHaveCSS('font-size','16px');await expect(dialog.getByRole('heading',{name:'显示字段',exact:true})).toHaveCSS('font-weight','500');
});
test('Root display-fields approved checkbox geometry and monochrome color do not inherit personnel blue',async({page})=>{
 await setup(page);await open(page);await page.keyboard.press('Escape');await page.getByRole('button',{name:'显示字段',exact:true}).click();const dialog=page.getByRole('dialog',{name:'显示字段',exact:true}),checkbox=dialog.getByRole('checkbox',{name:'金额',exact:true});await expect(checkbox).toHaveCSS('width','16px');await expect(checkbox).toHaveCSS('height','16px');await expect(checkbox).toHaveCSS('background-color','rgb(37, 37, 37)');
});
test('Root display-fields checkbox uses monochrome fill independently of its dimensions',async({page})=>{
 await setup(page);await open(page);await page.keyboard.press('Escape');await page.getByRole('button',{name:'显示字段',exact:true}).click();await expect(page.getByRole('dialog',{name:'显示字段',exact:true}).getByRole('checkbox',{name:'金额',exact:true})).toHaveCSS('background-color','rgb(37, 37, 37)');
});
