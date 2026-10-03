# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: root-record-shell.component.spec.ts >> Root Shell ordinary form route mounts records under the original persistent chrome
- Location: src/root-record-shell.component.spec.ts:44:1

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: getByRole('region', { name: '记录工作区' })
Expected: visible
Timeout: 5000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" getByRole('region', { name: '记录工作区' }) with timeout 5000ms
  - waiting for getByRole('region', { name: '记录工作区' })

```

```yaml
- banner:
  - text: WaveOS
  - navigation "全局应用标签":
    - button "首页"
    - button "报销管理"
    - button "关闭应用：报销管理": ×
    - button "全部应用"
  - button "账号"
- navigation "应用导航":
  - paragraph: 报销管理
  - button "全部应用"
  - paragraph: 应用 个人导航
  - button "应用中心"
- main "应用工作台":
  - heading "报销管理" [level=1]
  - tablist "应用表单":
    - tab "应用目录"
    - tab "当前表单" [selected]
  - status:
    - alert: 没有此应用的表单配置权限
    - button "重试"
```

# Test source

```ts
  1   | import {test,expect,type Page} from '@playwright/test';
  2   | const actor={id:'11111111-1111-4111-8111-111111111111',account:'record-owner'};
  3   | const appId='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333',fieldId='44444444-4444-4444-8444-444444444444',recordId='77777777-7777-4777-8777-777777777777';
  4   | const app={id:appId,name:'报销管理',ownerUserId:actor.id,policyRevision:1};
  5   | const formPath='applications/'+appId+'/forms/'+viewId,routePath='/app/'+formPath;
  6   | const ok=(data:unknown)=>({code:'OK',data,meta:null});
  7   | const runtime={appId,viewId,tableId:appId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:false,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq','gt','gte','lt','lte'],sortable:true,quickSearchable:false}}],layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000001',kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
  8   | async function fixture(page:Page,options:{ordinary?:boolean;createOnly?:boolean;detailFailure?:boolean;staleOpen?:boolean;reference?:boolean}={}){
  9   |  const calls:{path:string;method:string;body:any;actor?:string}[]=[];let value:string|null=null,version=0,searches=0;
  10  |  const referenceId='66666666-6666-4666-8666-666666666666',candidateId='88888888-8888-4888-8888-888888888888';
  11  |  const currentRuntime: any=structuredClone(runtime);
  12  |  if(options.reference){currentRuntime.fields.push({id:referenceId,name:'申请人',kind:'member',required:false,presentation:{helpText:null,displayTimeZone:null},input:{referenceKind:'member'},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});currentRuntime.layout.push({id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000002',kind:'field',fieldId:referenceId});}if(options.createOnly){currentRuntime.capabilities.read='none';currentRuntime.capabilities.search=false;}
  13  |  const item=()=>({id:recordId,appId,viewId,tableId:appId,createdBy:actor.id,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z',recordVersion:version,schemaVersion:1,values:{[fieldId]:value},referenceDisplays:{}});
  14  |  await page.route('**/api/v1/**',async route=>{
  15  |   const request=route.request(),path=new URL(request.url()).pathname.slice('/api/v1/'.length),method=request.method(),body=request.postData()?request.postDataJSON():null;
  16  |   calls.push({path,method,body,actor:request.headers()['x-expected-actor-id']});
  17  |   let data:unknown;
  18  |   if(path==='sessions/current')data=actor;
  19  |   else if(path==='me/access')data={user:actor,bootstrapAdmin:!options.ordinary,personnelManage:!options.ordinary,identities:[],permissions:[],applications:[]};
  20  |   else if(path==='applications')data={items:[options.ordinary?{...app,ownerUserId:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'}:app]};
  21  |   else if(path==='applications/'+appId)data=options.ordinary?{...app,ownerUserId:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'}:app;
  22  |   else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  23  |   else if(path===formPath+'/definition'&&!options.ordinary)data={appId,table:{id:appId,appId,name:'报销单',directoryId:null,position:0,schemaVersion:1,schemaReady:true},form:{id:viewId,appId,tableId:appId,name:'报销单',directoryId:null,position:0,viewVersion:1},fields:[],systemFields:[],layout:[],capabilities:{canManageDefinition:true}};
  24  |   else if(path===formPath+'/runtime')data=currentRuntime;
  25  |   else if(path===formPath+'/reference-candidates')return route.fulfill({status:200,json:{code:'OK',data:{items:[{id:candidateId,label:'普通成员',status:'active'}]},meta:{pagination:{nextPageToken:null,hasMore:false}}}});
  26  |   else if(path===formPath+'/records/search'){
  27  |    searches++;
  28  |    if(options.staleOpen&&searches>1)return route.fulfill({status:409,json:{code:'APPLICATION_QUERY_CHANGED',data:null}});
  29  |    data={items:value===null?[]:[item()],total:value===null?0:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'query-'+version,schemaVersion:1,viewVersion:1};
  30  |   }else if(path===formPath+'/records'&&method==='POST'){
  31  |    value=body.values[fieldId]==='12.345'?'12.35':body.values[fieldId];version=1;
  32  |    return route.fulfill({status:201,headers:{Location:'/api/v1/'+formPath+'/records/'+recordId},json:ok({operationId:body.operationId,id:recordId,recordVersion:version,schemaVersion:1,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z'})});
  33  |   }else if(path===formPath+'/records/'+recordId&&method==='PATCH'){
  34  |    value=body.changes[fieldId];version++;
  35  |    return route.fulfill({status:200,json:ok({operationId:body.operationId,id:recordId,recordVersion:version,schemaVersion:1,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z'})});
  36  |   }else if(path===formPath+'/records/'+recordId&&method==='GET'){
  37  |    if(options.detailFailure)return route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  38  |    data=item();
  39  |   }else return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  40  |   return route.fulfill({status:200,json:ok(data)});
  41  |  });
  42  |  return {calls,seed:(next:string)=>{value=next;version=1;}};
  43  | }
  44  | test('Root Shell ordinary form route mounts records under the original persistent chrome',async({page})=>{
  45  |  const api=await fixture(page,{ordinary:true});await page.goto(routePath);
> 46  |  await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();
      |                                                        ^ Error: expect(locator).toBeVisible() failed
  47  |  await expect(page.getByTestId('application-shell')).toHaveCount(1);
  48  |  await expect(page.getByRole('navigation',{name:'全局应用标签'})).toBeVisible();
  49  |  await expect(page.getByRole('navigation',{name:'应用导航'})).toBeVisible();
  50  |  await expect(page.getByRole('button',{name:'配置表单',exact:true})).toHaveCount(0);
  51  |  expect(api.calls.filter(c=>c.path.endsWith('/definition')||c.path.endsWith('/structure'))).toEqual([]);
  52  |  expect(api.calls.filter(c=>c.path.startsWith('applications/')).every(c=>c.actor===actor.id)).toBe(true);
  53  | });
  54  | test('Root Shell create then read authoritative normalized data and edit a record',async({page})=>{
  55  |  const api=await fixture(page);await page.goto(routePath);await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();
  56  |  const shell=await page.getByTestId('application-shell').elementHandle();
  57  |  await page.getByRole('button',{name:'新建记录',exact:true}).click();
  58  |  const create=page.getByRole('dialog',{name:'新建记录',exact:true});await expect(create).toBeVisible();
  59  |  await create.getByLabel('金额',{exact:true}).fill('12.345');await create.getByRole('button',{name:'保存记录',exact:true}).click();
  60  |  await expect(page.getByRole('status').filter({hasText:'记录已保存'})).toBeVisible();
  61  |  const detail=page.getByRole('dialog',{name:'记录详情',exact:true});await expect(detail).toBeVisible();await expect(detail.getByLabel('金额',{exact:true})).toHaveValue('12.35');
  62  |  await detail.getByRole('button',{name:'编辑记录',exact:true}).click();
  63  |  await detail.getByLabel('金额',{exact:true}).fill('20.10');await detail.getByRole('button',{name:'保存记录',exact:true}).click();
  64  |  await expect(detail.getByLabel('金额',{exact:true})).not.toBeEditable();await expect(detail.getByLabel('金额',{exact:true})).toHaveValue('20.10');
  65  |  const writes=api.calls.filter(c=>c.path.startsWith(formPath+'/records')&&['POST','PATCH'].includes(c.method)&&!c.path.endsWith('/search'));
  66  |  expect(writes).toHaveLength(2);expect(writes[0].body.values).toEqual({[fieldId]:'12.345'});
  67  |  expect(writes[1].body).toMatchObject({expectedSchemaVersion:1,expectedRecordVersion:1,changes:{[fieldId]:'20.10'}});
  68  |  expect(writes[1].body.operationId).not.toBe(writes[0].body.operationId);
  69  |  expect(api.calls.filter(c=>c.path===formPath+'/records/'+recordId&&c.method==='GET').length).toBeGreaterThanOrEqual(2);
  70  |  expect(await shell!.evaluate(node=>node.isConnected)).toBe(true);
  71  |  await page.screenshot({path:test.info().outputPath('record-shell-saved-detail.png'),fullPage:true});
  72  | });
  73  | test('Root confirmed write with failed reread remains saved and cannot submit again',async({page})=>{
  74  |  const api=await fixture(page,{detailFailure:true});await page.goto(routePath);await page.getByRole('button',{name:'新建记录',exact:true}).click();
  75  |  const dialog=page.getByRole('dialog',{name:'新建记录',exact:true});await dialog.getByLabel('金额',{exact:true}).fill('12.00');await dialog.getByRole('button',{name:'保存记录',exact:true}).click();
  76  |  await expect(page.getByRole('status').filter({hasText:'记录已保存'})).toBeVisible();
  77  |  await expect(page.getByRole('alert')).toContainText('无法读取');
  78  |  await expect(page.getByRole('button',{name:'重试读取记录',exact:true})).toBeVisible();
  79  |  expect(api.calls.filter(c=>c.path===formPath+'/records'&&c.method==='POST')).toHaveLength(1);
  80  |  await expect(page.getByRole('button',{name:'保存记录',exact:true})).toHaveCount(0);
  81  | });
  82  | test('Root create-only actor confirms a receipt without an unauthorized detail or search read',async({page})=>{
  83  |  const api=await fixture(page,{ordinary:true,createOnly:true});await page.goto(routePath);await page.getByRole('button',{name:'新建记录',exact:true}).click();
  84  |  const dialog=page.getByRole('dialog',{name:'新建记录',exact:true});await dialog.getByLabel('金额',{exact:true}).fill('12.00');await dialog.getByRole('button',{name:'保存记录',exact:true}).click();
  85  |  await expect(page.getByRole('status').filter({hasText:'记录已保存'})).toBeVisible();
  86  |  expect(api.calls.filter(c=>c.path===formPath+'/records/search'||c.path===formPath+'/records/'+recordId&&c.method==='GET')).toEqual([]);
  87  | });
  88  | test('Root opening an existing row validates the original query before reading detail',async({page})=>{
  89  |  const api=await fixture(page,{ordinary:true,staleOpen:true});api.seed('18.00');await page.goto(routePath);
  90  |  await page.getByRole('button',{name:'打开记录：金额 18.00',exact:true}).click();
  91  |  await expect(page.getByRole('alert')).toContainText('刷新');await expect(page.getByRole('dialog',{name:'记录详情',exact:true})).toHaveCount(0);
  92  |  const searches=api.calls.filter(c=>c.path===formPath+'/records/search');expect(searches).toHaveLength(2);expect(searches[1].body.queryVersion).toBe('query-1');
  93  |  expect(api.calls.filter(c=>c.path===formPath+'/records/'+recordId)).toEqual([]);
  94  | });
  95  | test('Root dirty record modal close requires an explicit discard and restores the origin focus',async({page})=>{
  96  |  await fixture(page);await page.goto(routePath);const trigger=page.getByRole('button',{name:'新建记录',exact:true});await trigger.click();
  97  |  const editor=page.getByRole('dialog',{name:'新建记录',exact:true});await editor.getByLabel('金额',{exact:true}).fill('7.00');await page.keyboard.press('Escape');
  98  |  const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});await expect(guard).toBeVisible();await guard.getByRole('button',{name:'继续编辑',exact:true}).click();
  99  |  await expect(editor.getByLabel('金额',{exact:true})).toHaveValue('7.00');await page.keyboard.press('Escape');await guard.getByRole('button',{name:'放弃修改',exact:true}).click();
  100 |  await expect(editor).toHaveCount(0);await expect(trigger).toBeFocused();
  101 | });
  102 | 
  103 | test('Root ordinary actor cannot load owner definition by entering the design URL',async({page})=>{
  104 |  const api=await fixture(page,{ordinary:true});await page.goto(routePath+'/design');
  105 |  await expect(page.getByRole('alert')).toContainText('权限');
  106 |  expect(api.calls.filter(c=>c.path.endsWith('/definition'))).toEqual([]);
  107 |  await expect(page.getByRole('region',{name:'表单设计器'})).toHaveCount(0);
  108 | });
  109 | 
  110 | test('Root ordinary record editor searches only field-scoped reference candidates',async({page})=>{
  111 |  const api=await fixture(page,{ordinary:true,reference:true});const candidateUrls:URL[]=[];
  112 |  page.on('request',request=>{if(request.url().includes('reference-candidates'))candidateUrls.push(new URL(request.url()));});
  113 |  await page.goto(routePath);await page.getByRole('button',{name:'新建记录',exact:true}).click();
  114 |  const dialog=page.getByRole('dialog',{name:'新建记录',exact:true});await dialog.getByRole('button',{name:'选择成员',exact:true}).click();
  115 |  await page.getByRole('searchbox',{name:'搜索成员',exact:true}).fill('普通');await page.getByRole('button',{name:'普通成员',exact:true}).click();
  116 |  await expect(dialog.getByText('普通成员',{exact:true})).toBeVisible();await dialog.getByLabel('金额',{exact:true}).fill('9.00');await dialog.getByRole('button',{name:'保存记录',exact:true}).click();
  117 |  await expect(page.getByRole('status').filter({hasText:'记录已保存'})).toBeVisible();
  118 |  const q=candidateUrls.find(url=>url.searchParams.get('q')==='普通');expect(q).toBeDefined();
  119 |  expect(q!.pathname).toBe('/api/v1/'+formPath+'/reference-candidates');
  120 |  expect(q!.searchParams.get('fieldId')).toBe('66666666-6666-4666-8666-666666666666');
  121 |  expect(q!.searchParams.get('action')).toBe('create');expect(q!.searchParams.has('recordId')).toBe(false);
  122 |  expect(api.calls.some(c=>/\/(member|department)-candidates$/.test(c.path))).toBe(false);
  123 |  const write=api.calls.find(c=>c.path===formPath+'/records'&&c.method==='POST');
  124 |  expect(write!.body.values['66666666-6666-4666-8666-666666666666']).toBe('88888888-8888-4888-8888-888888888888');
  125 | });
  126 | 
  127 | test('Root owner returns from configuration to the runtime page that opened it',async({page})=>{
  128 |  await fixture(page);await page.goto(routePath);await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();
  129 |  const shell=await page.getByTestId('application-shell').elementHandle();
  130 |  await page.getByRole('button',{name:'配置表单',exact:true}).click();await expect(page).toHaveURL(new RegExp(routePath+'/design$'));
  131 |  await expect(page.getByRole('region',{name:'表单设计器'})).toBeVisible();
  132 |  await page.getByRole('button',{name:'返回工作台',exact:true}).click();await expect(page).toHaveURL(new RegExp(routePath+'$'));
  133 |  await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();expect(await shell!.evaluate(node=>node.isConnected)).toBe(true);
  134 | });
  135 | 
```