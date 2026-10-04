import {test,expect,type Page} from '@playwright/test';
const actor={id:'11111111-1111-4111-8111-111111111111',account:'record-owner'};
const appId='22222222-2222-4222-8222-222222222222',viewId='33333333-3333-4333-8333-333333333333',fieldId='44444444-4444-4444-8444-444444444444',recordId='77777777-7777-4777-8777-777777777777';
const app={id:appId,name:'报销管理',ownerUserId:actor.id,policyRevision:1};
const formPath='applications/'+appId+'/forms/'+viewId,routePath='/app/'+formPath;
const ok=(data:unknown)=>({code:'OK',data,meta:null});
const runtime={appId,viewId,tableId:appId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:false,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:20,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq','gt','gte','lt','lte'],sortable:true,quickSearchable:false}}],layout:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000001',kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
async function fixture(page:Page,options:{ordinary?:boolean;createOnly?:boolean;detailFailure?:boolean;staleOpen?:boolean;reference?:boolean;rejectCreate?:boolean;rejectEditRefresh?:boolean}={}){
 const calls:{path:string;method:string;body:any;actor?:string}[]=[];let value:string|null=null,version=0,searches=0,rejectedEdits=0,failedRefreshedReads=0;
 const referenceId='66666666-6666-4666-8666-666666666666',candidateId='88888888-8888-4888-8888-888888888888';
 const currentRuntime: any=structuredClone(runtime);
 if(options.reference){currentRuntime.fields.push({id:referenceId,name:'申请人',kind:'member',required:false,presentation:{helpText:null,displayTimeZone:null},input:{referenceKind:'member'},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:false}});currentRuntime.layout.push({id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000002',kind:'field',fieldId:referenceId});}if(options.createOnly){currentRuntime.capabilities.read='none';currentRuntime.capabilities.search=false;}
 const item=()=>({id:recordId,appId,viewId,tableId:appId,createdBy:actor.id,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z',recordVersion:version,schemaVersion:1,values:{[fieldId]:value},referenceDisplays:{}});
 await page.route('**/api/v1/**',async route=>{
  const request=route.request(),path=new URL(request.url()).pathname.slice('/api/v1/'.length),method=request.method(),body=request.postData()?request.postDataJSON():null;
  calls.push({path,method,body,actor:request.headers()['x-expected-actor-id']});
  let data:unknown;
  if(path==='sessions/current')data=actor;
  else if(path==='me/access')data={user:actor,bootstrapAdmin:!options.ordinary,personnelManage:!options.ordinary,identities:[],permissions:[],applications:[]};
  else if(path==='applications')data={items:[options.ordinary?{...app,ownerUserId:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'}:app]};
  else if(path==='applications/'+appId)data=options.ordinary?{...app,ownerUserId:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'}:app;
  else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
  else if(path===formPath+'/definition'&&!options.ordinary)data={appId,table:{id:appId,appId,name:'报销单',directoryId:null,position:0,schemaVersion:1,schemaReady:true},form:{id:viewId,appId,tableId:appId,name:'报销单',directoryId:null,position:0,viewVersion:1},fields:[],systemFields:[],layout:[],capabilities:{canManageDefinition:true}};
  else if(path===formPath+'/runtime')data=currentRuntime;
  else if(path===formPath+'/reference-candidates')return route.fulfill({status:200,json:{code:'OK',data:{items:[{id:candidateId,label:'普通成员',status:'active'}]},meta:{pagination:{nextPageToken:null,hasMore:false}}}});
  else if(path===formPath+'/records/search'){
   searches++;
   if(options.staleOpen&&searches>1)return route.fulfill({status:409,json:{code:'APPLICATION_QUERY_CHANGED',data:null}});
   data={items:value===null?[]:[item()],total:value===null?0:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'query-'+version,schemaVersion:1,viewVersion:1};
  }else if(path===formPath+'/records'&&method==='POST'){
   if(options.rejectCreate)return route.fulfill({status:409,json:{code:'APPLICATION_QUERY_CONTEXT_EXPIRED',data:null}});
   value=body.values[fieldId]==='12.345'?'12.35':body.values[fieldId];version=1;
   return route.fulfill({status:201,headers:{Location:'/api/v1/'+formPath+'/records/'+recordId},json:ok({operationId:body.operationId,id:recordId,recordVersion:version,schemaVersion:1,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z'})});
  }else if(path===formPath+'/records/'+recordId&&method==='PATCH'){
   if(options.rejectEditRefresh){rejectedEdits++;return route.fulfill({status:409,json:{code:'APPLICATION_RECORD_CONFLICT',data:null}});}
   value=body.changes[fieldId]==='20.105'?'20.11':body.changes[fieldId];version++;
   return route.fulfill({status:200,json:ok({operationId:body.operationId,id:recordId,recordVersion:version,schemaVersion:1,createdAt:'2026-10-03T09:00:00Z',updatedAt:'2026-10-03T09:01:00Z'})});
  }else if(path===formPath+'/records/'+recordId&&method==='GET'){
   if(options.detailFailure||options.rejectEditRefresh&&rejectedEdits>0&&failedRefreshedReads++===0)return route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
   data=item();
  }else return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  return route.fulfill({status:200,json:ok(data)});
 });
 return {calls,seed:(next:string)=>{value=next;version=1;}};
}

// Root oracle: V030-021 approved Monochrome Shell, not current CSS implementation.
const shell = (page:Page)=>page.getByTestId('workspace-shell');
async function monoGeometry(page:Page,width:number,height:number){
 await expect(shell(page)).toHaveCount(1);
 const surface=page.getByTestId('workspace-surface'), rail=page.getByTestId('workspace-rail'), well=page.getByTestId('workspace-content-well');
 for(const el of [surface,rail,well]) await expect(el).toBeVisible();
 const s=await surface.boundingBox(),r=await rail.boundingBox(),w=await well.boundingBox();
 expect(s).not.toBeNull();expect(r).not.toBeNull();expect(w).not.toBeNull();
 expect(Math.abs(s!.x-78)).toBeLessThanOrEqual(1);expect(Math.abs(s!.y)).toBeLessThanOrEqual(1);
 expect(Math.abs(s!.x+s!.width-width)).toBeLessThanOrEqual(1);
 expect(Math.abs(s!.y+s!.height-height)).toBeLessThanOrEqual(1);
 expect(r!.x).toBe(0);expect(Math.abs(r!.width-78)).toBeLessThanOrEqual(1);
 expect(w!.x).toBeGreaterThanOrEqual(268);expect(w!.y).toBeGreaterThanOrEqual(64);
 const appearance=await surface.evaluate(n=>{const s=getComputedStyle(n);return{color:s.backgroundColor,tl:parseFloat(s.borderTopLeftRadius),tr:parseFloat(s.borderTopRightRadius),br:parseFloat(s.borderBottomRightRadius)}});
 expect(appearance.color).toBe('rgb(255, 255, 255)');expect(appearance.tl).toBeGreaterThanOrEqual(20);expect(appearance.tr).toBe(0);expect(appearance.br).toBe(0);
 const palette=await page.getByTestId('workspace-shell').evaluate(n=>{const s=getComputedStyle(n);return s.backgroundColor;});
 const rgb=palette.match(/[\d.]+/g)!.slice(0,3).map(Number);expect(Math.max(...rgb)-Math.min(...rgb)).toBeLessThanOrEqual(1);expect(Math.min(...rgb)).toBeGreaterThanOrEqual(248);
 const scroll=await page.evaluate(()=>({width:document.documentElement.scrollWidth,viewport:innerWidth}));expect(scroll.width).toBeLessThanOrEqual(scroll.viewport);
}
for(const [width,height] of [[1440,1000],[1280,800]]){
 test('Root Mono P1 unified full-bleed desktop geometry '+width,async({page})=>{
  await page.setViewportSize({width,height});await fixture(page);await page.goto('/app');await monoGeometry(page,width,height);
  await expect(page.getByRole('navigation',{name:'全局应用标签'})).toBeVisible();
  await expect(page.getByRole('navigation',{name:'应用导航'})).toBeVisible();
  await page.screenshot({path:test.info().outputPath('monochrome-home-'+width+'.png'),fullPage:true});
 });
}
test('Root Mono P1 ordinary record route retains live table and authorization under the same shell',async({page})=>{
 await page.setViewportSize({width:1440,height:1000});const api=await fixture(page,{ordinary:true});api.seed('18.00');await page.goto(routePath);
 await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();await monoGeometry(page,1440,1000);
 await expect(page.getByRole('button',{name:'打开记录：金额 18.00',exact:true})).toBeVisible();
 expect(api.calls.filter(c=>c.path.endsWith('/definition')||c.path.endsWith('/structure'))).toEqual([]);
 await page.screenshot({path:test.info().outputPath('monochrome-records.png'),fullPage:true});
});
test('Root Mono P1 administration and back preserve shell and opened application tab',async({page})=>{
 await fixture(page);await page.goto(routePath);await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();
 const root=await shell(page).elementHandle();await expect(root).not.toBeNull();
 const tabs=page.getByRole('navigation',{name:'全局应用标签'});await expect(tabs.getByRole('button',{name:app.name,exact:true})).toBeVisible();
 await page.getByTestId('workspace-nav-admin').click();await expect(page).toHaveURL(/\/app\/admin$/);await expect(shell(page)).toHaveCount(1);
 expect(await root!.evaluate(n=>n.isConnected)).toBe(true);
 await expect(tabs.getByRole('button',{name:app.name,exact:true})).toBeVisible();
 await page.getByTestId('workspace-nav-home').click();await expect(page).toHaveURL(/\/app$/);expect(await root!.evaluate(n=>n.isConnected)).toBe(true);
});
test('Root Mono P1 no invented business data when real catalogue is empty',async({page})=>{
 await fixture(page);await page.route('**/api/v1/applications',r=>r.fulfill({json:ok({items:[]})}));await page.goto('/app');await expect(shell(page)).toBeVisible();
 await expect(page.getByText('暂无可用应用',{exact:true})).toBeVisible();
 for(const text of ['上海出差 · 差旅报销','产品周会','林晓','核对差旅报销']) await expect(page.getByText(text,{exact:true})).toHaveCount(0);
});
test('Root Mono P1 confirmed record save still uses authoritative value after visual migration',async({page})=>{
 const api=await fixture(page);await page.goto(routePath);await page.getByRole('button',{name:'新建记录',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'新建记录',exact:true});await dialog.getByLabel('金额',{exact:true}).fill('12.345');await dialog.getByRole('button',{name:'保存记录',exact:true}).click();
 await expect(page.getByRole('status').filter({hasText:'记录已保存'})).toBeVisible();
 await expect(page.getByRole('dialog',{name:'记录详情',exact:true}).getByLabel('金额',{exact:true})).toHaveValue('12.35');
 expect(api.calls.filter(c=>c.path===formPath+'/records'&&c.method==='POST')).toHaveLength(1);
});
test('Root Mono P1 reduced-motion route transition does not hide content or duplicate shell',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await fixture(page);await page.goto(routePath);
 await expect(page.getByRole('region',{name:'记录工作区'})).toBeVisible();const root=await shell(page).elementHandle();
 await page.getByTestId('workspace-nav-home').click();await expect(page).toHaveURL(/\/app$/);await expect(shell(page)).toHaveCount(1);expect(await root!.evaluate(n=>n.isConnected)).toBe(true);
 const geometry=await shell(page).boundingBox();expect(geometry!.width).toBeGreaterThan(0);
});
test('Root Mono P1 account remains a single accessible control and home cannot be closed',async({page})=>{
 await fixture(page);await page.goto('/app');await expect(shell(page)).toBeVisible();
 await expect(page.getByRole('button',{name:'账号',exact:true})).toHaveCount(1);
 const tabs=page.getByRole('navigation',{name:'全局应用标签'});
 await expect(tabs.getByRole('button',{name:/关闭.*(首页|工作台)/})).toHaveCount(0);
 await expect(tabs.getByText('固定',{exact:true})).toHaveCount(0);
});
