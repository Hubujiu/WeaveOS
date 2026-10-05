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


// Root-owned V030-025 oracle: user-requested contextual editor, Figma 498:9471.
// Supersedes V023 three-card geometry only; behavior expectations remain.
for(const [width,height] of [[1440,1000],[1280,800]]){
 test('Root focused designer uses shell sidebars and a clean central canvas '+width,async({page})=>{
  await page.setViewportSize({width,height});await fixture(page);await page.goto(routePath+'/design');
  const palette=page.getByRole('region',{name:'字段面板'}),canvas=page.getByRole('region',{name:'表单画布'}),properties=page.getByRole('region',{name:'字段属性'});
  for(const el of [palette,canvas,properties])await expect(el).toBeVisible();
  const [p,c,a]=await Promise.all([palette,canvas,properties].map(async el=>(await el.boundingBox())!));
  const rail=(await page.getByTestId('workspace-rail').boundingBox())!;
  const top=page.locator('.mono-top');const t=(await top.boundingBox())!;
  expect(Math.abs(rail.width-78)).toBeLessThanOrEqual(1);
  expect(Math.abs(p.x-78)).toBeLessThanOrEqual(1);expect(Math.abs(p.width-190)).toBeLessThanOrEqual(1);
  expect(Math.abs(a.width-332)).toBeLessThanOrEqual(1);expect(Math.abs(a.x+a.width-width)).toBeLessThanOrEqual(1);
  expect(Math.abs(t.y)).toBeLessThanOrEqual(1);expect(Math.abs(t.height-64)).toBeLessThanOrEqual(1);
  expect(p.y).toBeGreaterThanOrEqual(64);expect(p.y).toBeLessThanOrEqual(96);
  expect(a.y).toBeGreaterThanOrEqual(64);expect(a.y).toBeLessThanOrEqual(96);
  expect(c.x).toBeGreaterThanOrEqual(p.x+p.width+24);expect(c.x+c.width).toBeLessThanOrEqual(a.x-24);
  expect(c.y).toBeGreaterThanOrEqual(88);expect(c.y).toBeLessThanOrEqual(104);expect(c.width).toBeGreaterThanOrEqual(500);
  for(const name of ['保存','退出','预览']){
   const action=page.getByRole('button',{name,exact:true});await expect(action).toHaveCount(1);await expect(top.getByRole('button',{name,exact:true})).toBeVisible();
   const b=(await action.boundingBox())!;expect(b.y).toBeGreaterThanOrEqual(t.y);expect(b.y+b.height).toBeLessThanOrEqual(64);expect(b.x).toBeGreaterThan(width-380);
  }
  await expect(canvas.getByRole('heading')).toHaveCount(0);
  await expect(page.getByText('拖拽或用键盘添加字段，保存后才会生效',{exact:true})).toHaveCount(0);
  await expect(page.locator('.forms-designer-navigation')).toHaveCount(0);
  const styles=await Promise.all([palette,canvas,properties].map(el=>el.evaluate(n=>getComputedStyle(n).backgroundColor)));
  expect(styles).toEqual(['rgb(255, 255, 255)','rgb(255, 255, 255)','rgb(255, 255, 255)']);
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await expect(page.getByTestId('workspace-shell')).toHaveCount(1);
  await page.screenshot({path:test.info().outputPath('focused-designer-'+width+'.png'),fullPage:true});
 });
}
test('Root Mono designer retains palette, neutral selection and preview source focus',async({page})=>{
 await page.setViewportSize({width:1440,height:1000});const api=await fixture(page);await page.goto(routePath+'/design');
 const palette=page.getByRole('region',{name:'字段面板'});
 for(const name of ['文本','多行文本','数字','金额','日期','日期时间','单选','多选','布尔','成员','部门'])await expect(palette.getByRole('button',{name,exact:true})).toBeVisible();
 await palette.getByRole('button',{name:'文本',exact:true}).click();await page.getByLabel('字段名称',{exact:true}).fill('保留业务字段');
 const selected=page.locator('.forms-canvas-node.selected');await expect(selected).toHaveCount(1);
 const color=await selected.evaluate(n=>getComputedStyle(n).borderTopColor);const rgb=color.match(/[\d.]+/g)!.slice(0,3).map(Number);
 expect(Math.max(...rgb)-Math.min(...rgb)).toBeLessThanOrEqual(1);
 const preview=page.getByRole('button',{name:'预览',exact:true});await expect(preview).toHaveCount(1);await preview.click();
 const modal=page.getByRole('dialog',{name:'表单预览',exact:true});await expect(modal).toBeVisible();await expect(modal.getByText('保留业务字段',{exact:true})).toBeVisible();
 await page.keyboard.press('Escape');await expect(modal).toBeHidden();await expect(preview).toBeFocused();await expect(page.getByLabel('字段名称',{exact:true})).toHaveValue('保留业务字段');
 expect(api.calls.filter(c=>c.method!=='GET')).toHaveLength(0);
});
test('Root Mono designer narrow reduced-motion remains editable without page overflow',async({page})=>{
 await page.setViewportSize({width:390,height:844});await page.emulateMedia({reducedMotion:'reduce'});await fixture(page);await page.goto(routePath+'/design');
 const palette=page.getByRole('region',{name:'字段面板'});await palette.getByRole('button',{name:'文本',exact:true}).click();
 const name=page.getByLabel('字段名称',{exact:true});await name.scrollIntoViewIfNeeded();await name.fill('小屏仍可编辑');await expect(name).toHaveValue('小屏仍可编辑');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.getByRole('button',{name:'预览',exact:true}).click();await expect(page.getByRole('dialog',{name:'表单预览',exact:true})).toBeVisible();
 await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toHaveCount(0);await expect(name).toHaveValue('小屏仍可编辑');
});

test('Root focused designer exit keeps unsaved edits until explicit discard',async({page})=>{
 await page.setViewportSize({width:1440,height:1000});const api=await fixture(page);await page.goto(routePath+'/design');
 await page.getByRole('region',{name:'字段面板'}).getByRole('button',{name:'文本',exact:true}).click();
 await page.getByLabel('字段名称',{exact:true}).fill('退出前的草稿');
 await page.locator('.mono-top').getByRole('button',{name:'退出',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'结构或布局尚未保存',exact:true});await expect(dialog).toBeVisible();
 await dialog.getByRole('button',{name:'继续编辑',exact:true}).click();
 await expect(dialog).toBeHidden();await expect(page.getByLabel('字段名称',{exact:true})).toHaveValue('退出前的草稿');
 await page.locator('.mono-top').getByRole('button',{name:'退出',exact:true}).click();
 await dialog.getByRole('button',{name:'放弃修改并离开',exact:true}).click();
 await expect(page).toHaveURL(new RegExp(routePath+'$'));await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
 await expect(page.getByRole('navigation',{name:'应用导航',exact:true})).toBeVisible();
 await expect(page.getByTestId('workspace-shell')).toHaveCount(1);
 expect(api.calls.filter(c=>c.method!=='GET'&&!(c.method==='POST'&&c.path===formPath+'/records/search'))).toHaveLength(0);
});
test('Root focused designer header save waits for preflight and preserves edit on rejection',async({page})=>{
 await page.setViewportSize({width:1440,height:1000});await fixture(page);
 let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});let seen=0;
 await page.route('**/api/v1/'+formPath+'/definition/preflight',async route=>{seen++;await gate;await route.fulfill({status:400,json:{code:'INVALID_INPUT',data:null}});});
 await page.goto(routePath+'/design');
 await page.getByRole('region',{name:'字段面板'}).getByRole('button',{name:'文本',exact:true}).click();
 await page.getByLabel('字段名称',{exact:true}).fill('等待确认的字段');
 await page.locator('.mono-top').getByRole('button',{name:'保存',exact:true}).click();
 try{
  await expect.poll(()=>seen).toBe(1);
  await expect(page.getByText('正在预检…',{exact:true})).toBeVisible();
  await expect(page.locator('.mono-top').getByRole('button',{name:'保存',exact:true})).toBeDisabled();
  await expect(page.getByText('已保存',{exact:true})).toHaveCount(0);
 }finally{release();}
 await expect(page.getByRole('alert')).toContainText('输入不符合字段或布局规则');
 await expect(page.getByLabel('字段名称',{exact:true})).toHaveValue('等待确认的字段');
 await expect(page.locator('.mono-top').getByRole('button',{name:'保存',exact:true})).toBeEnabled();
});
