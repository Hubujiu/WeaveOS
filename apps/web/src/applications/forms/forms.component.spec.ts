import { expect, test, type Page } from '@playwright/test';
import type { Definition, Field, LayoutNode, Structure } from './contracts';

// Independent oracle: V030-014 PRD/ADR (2026-10-03), accepted V030-013 ADR,
// original Figma 327:3107, 332:5028, 332:5461, 337:10258, 340:18187.
const appId = '00000000-0000-4000-8000-000000000101';
const tableId = '00000000-0000-4000-8000-000000000102';
const viewId = '00000000-0000-4000-8000-000000000103';
const folderId = '00000000-0000-4000-8000-000000000104';
const actorA='00000000-0000-4000-8000-000000000181';
const actorB='00000000-0000-4000-8000-000000000182';
const createdTableId='00000000-0000-4000-8000-000000000105';
const createdViewId='00000000-0000-4000-8000-000000000106';
const table = {id:tableId,appId,name:'请假申请',directoryId:null,position:0,schemaVersion:0,schemaReady:false};
const form = {id:viewId,appId,tableId,name:'请假申请',directoryId:null,position:0,viewVersion:0};
const definition:Definition = {appId,table,form,fields:[],systemFields:[
  {id:'id',kind:'id',readOnly:true},{id:'createdBy',kind:'member',readOnly:true},
  {id:'createdAt',kind:'datetime',readOnly:true},{id:'updatedAt',kind:'datetime',readOnly:true},
  {id:'recordVersion',kind:'number',readOnly:true},
],layout:[],capabilities:{canManageDefinition:true}};
const structure:Structure = {appId,structureVersion:0,directories:[],tables:[table],forms:[form],capabilities:{canManageDefinition:true}};
const ok = (data: unknown) => ({code:'OK',message:'success',data,meta:{requestId:'test-request'}});
async function capture(page:Page,path:string){
  const dialog=page.getByRole('dialog');
  if(await dialog.count())await expect(dialog).toHaveAttribute('data-motion-ready','true');
  await page.screenshot({path,fullPage:true});
}

type Seen = {method:string;path:string;body:Record<string,unknown>|null;expectedActor:string|null};
type GuardControl=Window&{
  __formsGuardStatus?:()=>string|null;
  __formsGuardPrepare?:(decision:'discard'|'retain_operation')=>{ok:boolean;status?:string}|null;
  __formsGuardActiveId?:()=>number|null;
  __formsGuardOldUnsubscribe?:()=>void;
  __formsStrictMount?:(value:boolean)=>void;
};
async function fixture(page:Page, mode:'designer'|'structure'='designer', initialDefinition:Definition=definition,
  initialStructure:Structure=structure,strict=false,back:'normal'|'none'='normal') {
  const seen:Seen[]=[];
  let currentDefinition=structuredClone(initialDefinition);
  let currentStructure=structuredClone(initialStructure);
  let loseSaveResponse=false;
  let committedSave:Record<string,unknown>|null=null;
  await page.route('**/api/v1/**',async route=>{
    const request=route.request();
    const path=new URL(request.url()).pathname;
    const method=request.method();
    const body=request.postDataJSON() as Record<string,unknown>|null;
    seen.push({method,path,body,expectedActor:request.headers()['x-expected-actor-id']??null});
    if(path.endsWith('/structure')&&method==='GET')return route.fulfill({json:ok(currentStructure)});
    if(path.endsWith('/definition')&&method==='GET')return route.fulfill({json:ok({
      ...currentDefinition,form:{...currentDefinition.form,id:path.split('/')[6]??viewId},
    })});
    if(path.endsWith('/definition/preflight')&&method==='POST')return route.fulfill({json:ok({
      appId,tableId,viewId,schemaVersion:currentDefinition.table.schemaVersion,
      viewVersion:currentDefinition.form.viewVersion,dataRevision:0,dependencyRevision:0,
      plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},impacts:[],dependencies:[],
      blockingIssues:[],saveAllowed:true,confirmation:null,
    })});
    if(path.endsWith('/definition')&&method==='PUT'){
      committedSave=body;
      currentDefinition={...currentDefinition,fields:body?.fields as Field[],layout:body?.layout as LayoutNode[],
        table:{...table,schemaVersion:1,schemaReady:true},form:{...form,viewVersion:1}};
      if(loseSaveResponse)return route.abort('failed');
      return route.fulfill({json:ok({operationId:body?.operationId,definition:currentDefinition,
        appliedPlan:{schemaChanges:[],metadataChanged:true,layoutChanged:true}})});
    }
    if(path.includes('/application-operations/')&&method==='GET'){
      return route.fulfill({status:committedSave?200:404,json:committedSave?ok({operationId:committedSave.operationId,status:'confirmed',httpStatus:200,location:'',result:{operationId:committedSave.operationId,definition:currentDefinition,appliedPlan:{schemaChanges:[],metadataChanged:true,layoutChanged:true}}}):
        {code:'APPLICATION_NOT_FOUND',message:'not found',data:null,meta:{requestId:'test-request'}}});
    }
    if(path.endsWith('/directories')&&method==='POST'){
      const directory={id:folderId,appId,name:String(body?.name),parentId:(body?.parentId??null) as string|null,position:Number(body?.position)};
      currentStructure={...currentStructure,structureVersion:currentStructure.structureVersion+1,directories:[...currentStructure.directories,directory]};
      return route.fulfill({status:201,json:ok({directory,structureVersion:currentStructure.structureVersion})});
    }
    if(path.includes('/directories/')&&method==='PUT'){
      const id=path.split('/').at(-1)!;
      const directory={...currentStructure.directories.find(item=>item.id===id)!,name:String(body?.name),
        parentId:(body?.parentId??null) as string|null,position:Number(body?.position)};
      currentStructure={...currentStructure,structureVersion:currentStructure.structureVersion+1,
        directories:currentStructure.directories.map(item=>item.id===id?directory:item)};
      return route.fulfill({json:ok({directory,structureVersion:currentStructure.structureVersion})});
    }
    if(path.endsWith('/forms')&&method==='POST'){
      const source=body?.source as {kind:'new_table'|'existing_table';tableId?:string};
      const nextTable=source.kind==='new_table'
        ?{...table,id:createdTableId,name:String(body?.name),directoryId:(body?.directoryId??null) as string|null,position:Number(body?.position)}
        :currentStructure.tables.find(item=>item.id===source.tableId)!;
      const nextForm={...form,id:createdViewId,tableId:nextTable.id,name:String(body?.name),
        directoryId:(body?.directoryId??null) as string|null,position:Number(body?.position)};
      currentStructure={...currentStructure,structureVersion:currentStructure.structureVersion+1,
        tables:source.kind==='new_table'?[...currentStructure.tables,nextTable]:currentStructure.tables,
        forms:[...currentStructure.forms,nextForm]};
      return route.fulfill({status:201,json:ok({table:nextTable,form:nextForm,structureVersion:currentStructure.structureVersion})});
    }
    if(path.includes('/forms/')&&method==='PUT'){
      const id=path.split('/').at(-1)!;
      const updated={...currentStructure.forms.find(item=>item.id===id)!,name:String(body?.name),
        directoryId:(body?.directoryId??null) as string|null,position:Number(body?.position)};
      currentStructure={...currentStructure,structureVersion:currentStructure.structureVersion+1,
        forms:currentStructure.forms.map(item=>item.id===id?updated:item)};
      return route.fulfill({json:ok({table:currentStructure.tables.find(item=>item.id===updated.tableId),form:updated,
        structureVersion:currentStructure.structureVersion})});
    }
    return route.fulfill({status:404,json:{code:'APPLICATION_NOT_FOUND',message:'not found',data:null,meta:{requestId:'test-request'}}});
  });
  await page.goto(`/src/applications/forms/${strict?'strict-harness':'harness'}.html?mode=${mode}&appId=${appId}&viewId=${viewId}&actorId=${actorA}&back=${back}`);
  return {seen,loseSaveResponse:()=>{loseSaveResponse=true;},getCommitted:()=>committedSave};
}

test('original designer shows three panels and keyboard-added draft can preview without a write',async({page})=>{
  const state=await fixture(page);
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await expect(page.getByRole('region',{name:'表单画布'})).toBeVisible();
  await expect(page.getByRole('region',{name:'字段属性'})).toBeVisible();
  await page.getByRole('button',{name:'文本',exact:true}).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('region',{name:'表单画布'})).toContainText('新建文本');
  await page.getByLabel('字段名称').fill('申请人');
  await capture(page,test.info().outputPath('designer-main.png'));
  await page.getByRole('button',{name:'预览',exact:true}).click();
  await expect(page.getByText('本地预览，尚未保存')).toBeVisible();
  await expect(page.getByRole('dialog').getByText('申请人')).toBeVisible();
  expect(state.seen.filter(x=>x.method!=='GET')).toHaveLength(0);
  await capture(page,test.info().outputPath('designer-preview.png'));
});

test('save preflights and commits with both versions and stable IDs only after server success',async({page})=>{
  const state=await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('申请人');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const calls=state.seen.filter(x=>x.path.endsWith('/definition')||x.path.endsWith('/definition/preflight'));
  expect(calls.map(x=>x.method)).toEqual(['GET','POST','PUT']);
  const saved=calls[2].body!;
  expect(saved).toMatchObject({expectedSchemaVersion:0,expectedViewVersion:0,optionMappings:[],confirmationToken:null});
  expect(typeof saved.operationId).toBe('string');
  expect((saved.fields as {id:string;name:string}[])[0]).toMatchObject({name:'申请人'});
  expect((saved.fields as {id:string}[])[0].id).toMatch(/^[0-9a-f]{8}-[0-9a-f-]{27,}$/);
});

test('lost save response keeps the original operation and draft until recovery confirms it',async({page})=>{
  const state=await fixture(page);
  state.loseSaveResponse();
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('未确认字段');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('保存结果暂未确认');
  await expect(page.getByLabel('字段名称')).toHaveValue('未确认字段');
  await page.getByRole('button',{name:'查询保存结果'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const put=state.seen.find(x=>x.method==='PUT');
  const query=state.seen.find(x=>x.path.includes('/application-operations/'));
  expect(query?.path.endsWith(String(put?.body?.operationId))).toBe(true);
  expect(state.seen.filter(x=>x.method==='PUT')).toHaveLength(1);
});

test('directory creation and new form creation use coherent structure and one atomic form POST',async({page})=>{
  const state=await fixture(page,'structure');
  await expect(page.getByRole('tree',{name:'应用目录'})).toBeVisible();
  await page.getByRole('button',{name:'新建目录'}).click();
  await capture(page,test.info().outputPath('directory-create.png'));
  await page.getByLabel('目录名称').fill('请假');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('treeitem',{name:'请假',exact:true})).toBeVisible();
  await capture(page,test.info().outputPath('directory-tree.png'));
  await page.getByRole('button',{name:'新建表单'}).click();
  await capture(page,test.info().outputPath('form-create.png'));
  await page.getByLabel('表单名称').fill('差旅申请');
  await page.getByRole('button',{name:'创建表单',exact:true}).click();
  await expect(page.getByRole('treeitem',{name:'差旅申请'})).toBeVisible();
  const creates=state.seen.filter(x=>x.method==='POST');
  expect(creates.map(x=>x.path)).toEqual([
    `/api/v1/applications/${appId}/directories`,
    `/api/v1/applications/${appId}/forms`,
  ]);
  expect(creates[1].body).toMatchObject({name:'差旅申请',source:{kind:'new_table'},expectedStructureVersion:1});
});

test('impact dialog requires a live confirmation token before a schema write',async({page})=>{
  const state=await fixture(page);
  await page.route('**/definition/preflight',route=>route.fulfill({json:ok({
    appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:2,dependencyRevision:1,
    plan:{schemaChanges:[{kind:'add',fieldId:'field-new',beforeKind:null,afterKind:'text'}],metadataChanged:true,layoutChanged:true},
    impacts:[{fieldId:'field-new',kind:'column_removal',nonNullRows:4,optionId:null}],
    dependencies:[],blockingIssues:[],saveAllowed:true,confirmation:null,
  })}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('dialog',{name:'保存预检：表单结构与布局'})).toBeVisible();
  await expect(page.getByRole('dialog').getByText('4 条已有值')).toBeVisible();
  await capture(page,test.info().outputPath('impact-missing-token.png'));
  await expect(page.getByRole('button',{name:'确认保存'})).toBeDisabled();
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('blocked dependencies show the affected resource and prevent PUT',async({page})=>{
  const state=await fixture(page);
  await page.route('**/definition/preflight',route=>route.fulfill({json:ok({
    appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:2,dependencyRevision:1,
    plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},impacts:[],
    dependencies:[{fieldId:'field-in-use',kind:'enabled_flow',resourceId:'flow-42'}],
    blockingIssues:[{code:'APPLICATION_SCHEMA_DEPENDENCY_BLOCKED',fieldIds:['field-in-use']}],
    saveAllowed:false,confirmation:null,
  })}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('dialog').getByText('enabled_flow · flow-42')).toBeVisible();
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
  await capture(page,test.info().outputPath('designer-dependency-block.png'));
});

test('used option deletion maps old option IDs before renewed preflight and confirmed Save',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000111';
  const oldId='00000000-0000-4000-8000-000000000112';
  const keepId='00000000-0000-4000-8000-000000000113';
  const seeded:Definition={...definition,table:{...table,schemaVersion:1,schemaReady:true},form:{...form,viewVersion:1},
    fields:[{id:fieldId,name:'审批结果',kind:'single_select',required:false,default:null,
      config:{options:[{id:oldId,label:'旧选项'},{id:keepId,label:'保留选项'}]},
      presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000114',kind:'field',fieldId,span:12}]};
  const state=await fixture(page,'designer',seeded);
  await page.route('**/definition/preflight',route=>{
    const body=route.request().postDataJSON() as {optionMappings:{fieldId:string;fromOptionId:string;toOptionId:string|null}[]};
    const mapped=body.optionMappings.some(item=>item.fieldId===fieldId&&item.fromOptionId===oldId&&item.toOptionId===keepId);
    return route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:1,viewVersion:1,dataRevision:2,dependencyRevision:1,
      plan:{schemaChanges:[{kind:'change_config',fieldId,beforeKind:'single_select',afterKind:'single_select'}],metadataChanged:true,layoutChanged:false},
      impacts:[{fieldId,kind:'option_mapping',nonNullRows:3,optionId:oldId}],dependencies:[],
      blockingIssues:mapped?[]:[{code:'APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED',fieldIds:[fieldId]}],
      saveAllowed:mapped,confirmation:mapped?{token:'confirmed-token',expiresAt:'2099-01-01T00:00:00Z'}:null})});
  });
  await page.getByRole('button',{name:'审批结果 单选'}).click();
  await page.getByRole('button',{name:'删除选项 旧选项'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('dialog')).toContainText('APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED');
  await capture(page,test.info().outputPath('option-mapping.png'));
  await page.getByLabel('已用选项映射 旧选项').selectOption(keepId);
  await page.getByRole('button',{name:'重新预检'}).click();
  await expect(page.getByRole('button',{name:'确认保存'})).toBeEnabled();
  await page.getByRole('button',{name:'确认保存'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const saves=state.seen.filter(item=>item.method==='PUT');
  expect(saves).toHaveLength(1);
  expect(saves[0].body).toMatchObject({confirmationToken:'confirmed-token',
    optionMappings:[{fieldId,fromOptionId:oldId,toOptionId:keepId}]});
});

test('late preflight for an old view cannot open impact or write into the newly selected view',async({page})=>{
  const nextViewId='00000000-0000-4000-8000-000000000120';
  const state=await fixture(page);
  let release!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  let seenPreflight!:()=>void;
  const started=new Promise<void>(resolve=>{seenPreflight=resolve;});
  await page.route('**/definition/preflight',async route=>{
    seenPreflight();await gate;
    try{await route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:0,dependencyRevision:0,
      plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
      impacts:[{fieldId:'field-old',kind:'column_removal',nonNullRows:1,optionId:null}],
      dependencies:[],blockingIssues:[],saveAllowed:true,
      confirmation:{token:'old-view-token',expiresAt:'2099-01-01T00:00:00Z'}})});}catch{/* scope change aborted the old preflight */}
  });
  await page.route(`**/forms/${nextViewId}/definition`,route=>route.fulfill({json:ok({...definition,form:{...form,id:nextViewId,name:'另一个视图'}})}));
  await page.goto(`/src/applications/forms/harness.html?mode=designer&appId=${appId}&viewId=${viewId}&switchViewId=${nextViewId}`);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await started;
  const oldCancelled=page.waitForEvent('requestfailed',request=>request.url().includes(`/forms/${viewId}/definition/preflight`));
  await page.getByRole('button',{name:'切换视图'}).click();
  await expect(page.getByRole('region',{name:'表单画布'})).toContainText('另一个视图');
  await oldCancelled;
  release();
  await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))));
  await expect(page.getByRole('dialog')).toHaveCount(0);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('keyboard can move an existing field into a group with the same ordered 12-column layout',async({page})=>{
  const state=await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('申请人');
  await page.getByRole('button',{name:'分组',exact:true}).click();
  await page.getByRole('button',{name:'申请人 文本'}).click();
  await page.getByLabel('所属分组').selectOption({label:'新建分组'});
  await page.getByLabel('字段宽度').selectOption('6');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const saved=state.seen.find(item=>item.method==='PUT')?.body;
  const layout=saved?.layout as {kind:string;children?:{kind:string;fieldId?:string;span?:number}[]}[];
  expect(layout).toHaveLength(1);
  expect(layout[0]).toMatchObject({kind:'group',span:12,children:[{kind:'field',span:6}]});
});

test('permission revoked during Save masks the retained draft until access is reverified',async({page})=>{
  const state=await fixture(page);
  let writes=0;
  await page.route('**/definition',route=>{if(route.request().method()!=='PUT')return route.fallback();writes++;
    return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',message:'forbidden',data:null,meta:{requestId:'forbidden'}}});});
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('保留草稿');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('没有此应用的表单配置权限');
  await expect(page.getByLabel('字段名称')).toHaveCount(0);
  await capture(page,test.info().outputPath('permission-revoked.png'));
  await page.unroute('**/definition');
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('字段名称')).toHaveValue('保留草稿');
  expect(writes).toBe(1);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('system fields stay read-only and each inserted node references a distinct server-defined code',async({page})=>{
  const state=await fixture(page);
  await page.getByRole('button',{name:'系统字段',exact:true}).click();
  await page.getByRole('button',{name:'系统字段',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const saved=state.seen.find(item=>item.method==='PUT')?.body;
  expect(saved?.fields).toEqual([]);
  const nodes=saved?.layout as {kind:string;fieldId:string}[];
  expect(nodes).toHaveLength(2);
  expect(new Set(nodes.map(item=>item.fieldId)).size).toBe(2);
  expect(nodes.every(item=>item.kind==='system_field')).toBe(true);
});

test('destructive preflight waits for explicit confirmation and sends its exact token',async({page})=>{
  const state=await fixture(page);
  await page.route('**/definition/preflight',route=>route.fulfill({json:ok({
    appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:2,dependencyRevision:1,
    plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
    impacts:[{fieldId:'field-affected',kind:'column_removal',nonNullRows:7,optionId:null}],
    dependencies:[],blockingIssues:[],saveAllowed:true,
    confirmation:{token:'impact-token-from-server',expiresAt:'2099-01-01T00:00:00Z'},
  })}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('dialog')).toContainText('7 条已有值');
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
  await capture(page,test.info().outputPath('impact-confirm.png'));
  await page.getByRole('button',{name:'确认保存'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  expect(state.seen.find(item=>item.method==='PUT')?.body?.confirmationToken).toBe('impact-token-from-server');
});

test('dirty exit and CAS conflict require an explicit decision while preserving the draft',async({page})=>{
  await fixture(page);
  await page.route('**/definition',route=>route.request().method()==='PUT'
    ?route.fulfill({status:409,json:{code:'APPLICATION_SCHEMA_CONFLICT',message:'conflict',data:{currentSchemaVersion:2},meta:{requestId:'conflict'}}})
    :route.fallback());
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('并发中的草稿');
  await page.getByRole('button',{name:'返回工作台'}).click();
  await expect(page.getByRole('dialog',{name:'结构或布局尚未保存'})).toBeVisible();
  await capture(page,test.info().outputPath('dirty-exit.png'));
  await page.getByRole('button',{name:'继续编辑'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('dialog',{name:'配置版本已变化'})).toBeVisible();
  await expect(page.getByLabel('字段名称')).toHaveValue('并发中的草稿');
  await capture(page,test.info().outputPath('schema-conflict.png'));
  await page.route(`**/forms/${viewId}/definition`,route=>route.request().method()==='GET'
    ?route.fulfill({json:ok({...definition,table:{...table,schemaVersion:2,schemaReady:true},
      form:{...form,viewVersion:2,name:'服务器新版'}})})
    :route.fallback());
  await page.getByRole('button',{name:'放弃草稿并加载最新版'}).click();
  await expect(page.getByRole('heading',{name:'服务器新版'})).toBeVisible();
  await expect(page.getByText('并发中的草稿')).toHaveCount(0);
  await expect(page.getByText('未保存', {exact:true})).toHaveCount(0);
});

test('structure CAS conflict keeps the entered name and offers an explicit latest-version reload',async({page})=>{
  await fixture(page,'structure');
  await page.route('**/directories',route=>route.request().method()==='POST'
    ?route.fulfill({status:409,json:{code:'APPLICATION_STRUCTURE_CONFLICT',message:'stale',
      data:{currentStructureVersion:3},meta:{requestId:'structure-conflict'}}})
    :route.fallback());
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('待核对目录');
  await page.getByRole('button',{name:'创建目录'}).click();
  await expect(page.getByRole('dialog')).toContainText('配置已被其他编辑者修改');
  await expect(page.getByLabel('目录名称')).toHaveValue('待核对目录');
  await expect(page.getByRole('button',{name:'放弃输入并加载最新版'})).toBeVisible();
  await capture(page,test.info().outputPath('structure-conflict.png'));
  await page.route(`**/applications/${appId}/structure`,route=>route.fulfill({json:ok({...structure,
    structureVersion:3,directories:[{id:folderId,appId,name:'服务器新版目录',parentId:null,position:0}]})}));
  await page.getByRole('button',{name:'放弃输入并加载最新版'}).click();
  await expect(page.getByRole('treeitem',{name:'服务器新版目录'})).toBeVisible();
  await expect(page.getByText('待核对目录')).toHaveCount(0);
});

test('structure write permission revocation masks retained input until access is reverified',async({page})=>{
  await fixture(page,'structure');
  await page.route('**/directories',route=>route.request().method()==='POST'
    ?route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',message:'forbidden',data:null,meta:{requestId:'denied'}}})
    :route.fallback());
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('保留输入');
  await page.getByRole('button',{name:'创建目录'}).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('没有此应用的表单配置权限');
  await page.unroute('**/directories');
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('目录名称')).toHaveValue('保留输入');
});

test('late directory create for an old app cannot replace the selected app tree',async({page})=>{
  const nextAppId='00000000-0000-4000-8000-000000000130';
  await fixture(page,'structure');
  let release!:()=>void,started!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  const requestStarted=new Promise<void>(resolve=>{started=resolve;});
  let oldCommitted=false;
  await page.route(`**/applications/${appId}/structure`,route=>route.fulfill({json:ok({
    ...structure,structureVersion:oldCommitted?1:0,
    directories:oldCommitted?[{id:folderId,appId,name:'旧应用目录',parentId:null,position:0}]:[],
  })}));
  await page.route(`**/applications/${appId}/directories`,async route=>{
    started();await gate;oldCommitted=true;
    return route.fulfill({status:201,json:ok({directory:{id:folderId,appId,name:'旧应用目录',parentId:null,position:0},structureVersion:1})});
  });
  await page.route(`**/applications/${nextAppId}/structure`,route=>route.fulfill({json:ok({
    appId:nextAppId,structureVersion:0,directories:[],tables:[],forms:[],capabilities:{canManageDefinition:true},
  })}));
  await page.goto(`/src/applications/forms/harness.html?mode=structure&appId=${appId}&viewId=${viewId}&switchAppId=${nextAppId}`);
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('旧应用目录');
  await page.getByRole('button',{name:'创建目录'}).click();
  await requestStarted;
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchApp?:(id:string)=>void}).__formsHarnessSwitchApp?.(id),nextAppId);
  await expect(page.getByRole('button',{name:'新建目录'})).toBeEnabled();
  const oldReply=page.waitForResponse(response=>response.url().includes(`/applications/${appId}/directories`));
  release();await oldReply;
  await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))));
  await expect(page.getByRole('treeitem',{name:'旧应用目录'})).toHaveCount(0);
  await expect(page.getByText('已保存', {exact:true})).toHaveCount(0);
});

test('empty structure explains the next action to keyboard and screen reader users',async({page})=>{
  await fixture(page,'structure',definition,{...structure,tables:[],forms:[]});
  await expect(page.getByText('暂无目录或表单')).toBeVisible();
  await expect(page.getByRole('button',{name:'新建目录'})).toBeEnabled();
  await expect(page.getByRole('button',{name:'新建表单'})).toBeEnabled();
  await capture(page,test.info().outputPath('structure-empty.png'));
});

test('removing an optional selected default option clears the invalid default before Save',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000140';
  const oldId='00000000-0000-4000-8000-000000000141';
  const keepId='00000000-0000-4000-8000-000000000142';
  const seeded:Definition={...definition,table:{...table,schemaVersion:1,schemaReady:true},form:{...form,viewVersion:1},
    fields:[{id:fieldId,name:'选项字段',kind:'single_select',required:false,default:oldId,
      config:{options:[{id:oldId,label:'旧默认值'},{id:keepId,label:'保留值'}]},
      presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000143',kind:'field',fieldId,span:12}]};
  const state=await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'选项字段 单选'}).click();
  await page.getByRole('button',{name:'删除选项 旧默认值'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const savedField=(state.seen.find(item=>item.method==='PUT')?.body?.fields as Field[])[0];
  expect(savedField.default).toBeNull();
  expect(savedField.config).toMatchObject({options:[{id:keepId,label:'保留值'}]});
});

test('decimal controls keep exact text and signed rounding places on the definition wire',async({page})=>{
  const state=await fixture(page);
  await page.getByRole('button',{name:'金额',exact:true}).click();
  await page.getByLabel('字段名称').fill('预算');
  await page.getByLabel('总精度').fill('12');
  await page.getByLabel('小数位数').fill('3');
  await page.getByLabel('处理位数').fill('-2');
  await page.getByLabel('舍入规则').selectOption('FLOOR');
  await page.getByLabel('默认值').fill('-149');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const savedField=(state.seen.find(item=>item.method==='PUT')?.body?.fields as Field[])[0];
  expect(savedField).toMatchObject({kind:'money',name:'预算',default:'-149',
    config:{precision:12,scale:3,roundingPlaces:-2,roundingMode:'FLOOR'}});
  expect(typeof savedField.default).toBe('string');
});

test('preview dialog restores trigger focus after fast close and under reduced motion',async({page})=>{
  await page.emulateMedia({reducedMotion:'reduce'});
  await fixture(page);
  const trigger=page.getByRole('button',{name:'预览',exact:true});
  await trigger.focus();
  await trigger.click();
  await expect(page.getByRole('dialog',{name:'表单预览'})).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await trigger.click();
  await page.getByRole('button',{name:'关闭弹窗'}).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(trigger).toBeFocused();
});

test('existing-table form source creates another view without creating a table',async({page})=>{
  const state=await fixture(page,'structure');
  await page.getByRole('button',{name:'新建表单'}).click();
  await page.getByLabel('表单名称').fill('第二视图');
  await page.getByLabel('使用现有逻辑表').check();
  await page.getByLabel('现有逻辑表',{exact:true}).selectOption(tableId);
  await page.getByRole('button',{name:'创建表单'}).click();
  await expect(page.getByRole('treeitem',{name:'第二视图'})).toBeVisible();
  await page.getByRole('treeitem',{name:'第二视图'}).getByRole('button').first().click();
  await page.getByRole('button',{name:'编辑表单名称与位置'}).click();
  await capture(page,test.info().outputPath('form-edit.png'));
  const writes=state.seen.filter(item=>item.method==='POST');
  expect(writes).toHaveLength(1);
  expect(writes[0].path).toBe(`/api/v1/applications/${appId}/forms`);
  expect(writes[0].body).toMatchObject({source:{kind:'existing_table',tableId}});
});

test('directory rename and same-app move use current structure CAS while descendants are excluded',async({page})=>{
  const childId='00000000-0000-4000-8000-000000000107';
  const initial:Structure={...structure,structureVersion:2,directories:[
    {id:folderId,appId,name:'父目录',parentId:null,position:0},
    {id:childId,appId,name:'子目录',parentId:folderId,position:0},
  ]};
  const state=await fixture(page,'structure',definition,initial);
  await page.getByRole('treeitem',{name:'子目录',exact:true}).getByRole('button').first().click();
  const rename=page.getByRole('button',{name:'重命名目录'});
  await rename.focus();await page.keyboard.press('Enter');
  await capture(page,test.info().outputPath('directory-rename.png'));
  await page.getByLabel('目录名称').fill('已改名');
  await page.getByRole('button',{name:'保存变更'}).click();
  await expect(page.getByRole('treeitem',{name:'已改名',exact:true})).toBeVisible();
  await page.getByRole('treeitem',{name:'父目录',exact:true}).getByRole('button').first().click();
  await page.getByRole('button',{name:'移动目录',exact:true}).click();
  await capture(page,test.info().outputPath('directory-move.png'));
  await expect(page.getByLabel('所属目录').getByRole('option',{name:'已改名'})).toHaveCount(0);
  await page.getByRole('button',{name:'取消'}).click();
  await page.getByRole('treeitem',{name:'已改名',exact:true}).getByRole('button').first().click();
  await page.getByRole('button',{name:'移动目录',exact:true}).click();
  await page.getByLabel('所属目录').selectOption('');
  await page.getByRole('button',{name:'保存变更'}).click();
  const writes=state.seen.filter(item=>item.method==='PUT');
  expect(writes).toHaveLength(2);
  expect(writes[0].body).toMatchObject({name:'已改名',parentId:folderId,expectedStructureVersion:2});
  expect(writes[1].body).toMatchObject({name:'已改名',parentId:null,expectedStructureVersion:3});
  await expect(page.getByRole('treeitem',{name:'已改名',exact:true})).toBeVisible();
});

test('palette drag into a group preserves the chosen drop parent in saved layout',async({page})=>{
  const state=await fixture(page);
  await page.getByRole('button',{name:'分组',exact:true}).click();
  await page.getByRole('button',{name:'文本',exact:true}).dragTo(
    page.locator('.forms-nested-grid').first());
  await expect(page.locator('.forms-nested-grid').first()).toContainText('新建文本');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const layout=state.seen.find(item=>item.method==='PUT')?.body?.layout as LayoutNode[];
  expect(layout).toHaveLength(1);
  expect(layout[0].kind).toBe('group');
  if(layout[0].kind==='group')expect(layout[0].children).toMatchObject([{kind:'field',span:12}]);
});

test('new view beside a selected form inherits its directory',async({page})=>{
  const initial:Structure={...structure,directories:[{id:folderId,appId,name:'业务目录',parentId:null,position:0}],
    forms:[{...form,directoryId:folderId}]};
  await fixture(page,'structure',definition,initial);
  await page.getByRole('treeitem',{name:'请假申请'}).getByRole('button').first().click();
  await page.getByRole('button',{name:'新建表单'}).click();
  await expect(page.getByLabel('所属目录')).toHaveValue(folderId);
});

test('existing member default remains visible and can be cleared without a candidate source',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000151';
  const memberId='00000000-0000-4000-8000-000000000152';
  const seeded:Definition={...definition,table:{...table,schemaVersion:1,schemaReady:true},
    form:{...form,viewVersion:1},fields:[{id:fieldId,name:'负责人',kind:'member',required:false,
      default:memberId,config:{},presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000153',kind:'field',fieldId,span:12}]};
  const state=await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'负责人 成员'}).click();
  await expect(page.getByLabel('当前默认引用 ID')).toHaveValue(memberId);
  await page.getByRole('button',{name:'清除默认引用'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const saved=(state.seen.find(item=>item.method==='PUT')?.body?.fields as Field[])[0];
  expect(saved.default).toBeNull();
});

test('text empty-string default and boolean unset false true remain distinct on Save',async({page})=>{
  const state=await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('启用默认值').check();
  await expect(page.getByLabel('默认值',{exact:true})).toHaveValue('');
  await page.getByRole('button',{name:'布尔',exact:true}).click();
  await page.getByLabel('默认状态').selectOption('false');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const fields=state.seen.find(item=>item.method==='PUT')?.body?.fields as Field[];
  expect(fields.map(item=>item.default)).toEqual(['',false]);
  await page.getByRole('button',{name:'新建布尔 布尔'}).click();
  await page.getByLabel('默认状态').selectOption('unset');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const latest=state.seen.filter(item=>item.method==='PUT').at(-1)?.body?.fields as Field[];
  expect(latest[1].default).toBeNull();
});

test('no-op Save with unknown result blocks exit even when definition is not dirty',async({page})=>{
  const ready:Definition={...definition,table:{...table,schemaReady:true,schemaVersion:1},
    form:{...form,viewVersion:1}};
  const state=await fixture(page,'designer',ready);
  state.loseSaveResponse();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('保存结果暂未确认');
  await expect(page.getByText('未保存',{exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'返回工作台'}).click();
  await expect(page.getByRole('dialog')).toContainText('保存结果未确认');
  const blocked=await page.evaluate(()=>window.dispatchEvent(new Event('beforeunload',{cancelable:true})));
  expect(blocked).toBe(false);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(1);
});

test('A to B to A navigation ignores a late first-generation preflight',async({page})=>{
  // Root: establish a real B mount before returning to A; seeing the generic
  // field region alone does not prove that React committed the new resource.
  const nextViewId='00000000-0000-4000-8000-000000000160';
  const state=await fixture(page);
  await page.route(`**/forms/${nextViewId}/definition`,route=>route.fulfill({json:ok({
    ...definition,form:{...form,id:nextViewId,name:'B 表单'},
  })}));
  let release!:()=>void;
  const gate=new Promise<void>(resolve=>{release=resolve;});
  let announceStarted!:()=>void;
  const started=new Promise<void>(resolve=>{announceStarted=resolve;});
  let announceSettled!:()=>void;
  const settled=new Promise<void>(resolve=>{announceSettled=resolve;});
  const wireSettled=Promise.race([
    page.waitForEvent('requestfinished',{predicate:request=>request.method()==='POST'&&request.url().endsWith('/definition/preflight')}),
    page.waitForEvent('requestfailed',{predicate:request=>request.method()==='POST'&&request.url().endsWith('/definition/preflight')}),
  ]);
  await page.route('**/definition/preflight',async route=>{
    announceStarted();await gate;
    try {
      await route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:0,
        dependencyRevision:0,plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
        impacts:[{fieldId:'old-field',kind:'column_removal',nonNullRows:1,optionId:null}],
        dependencies:[],blockingIssues:[],saveAllowed:true,confirmation:{token:'old-token',expiresAt:'2099-01-01T00:00:00Z'}})});
    } catch(error) {
      // An aborted old request is allowed. A non-aborted fulfillment failure
      // is a fixture error and must fail the test rather than be swallowed.
      if(!route.request().failure())throw error;
    } finally { announceSettled(); }
  });
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await started;
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),nextViewId);
  await expect(page.locator('.forms-toolbar-title strong')).toHaveText('B 表单');
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),viewId);
  await expect(page.locator('.forms-toolbar-title strong')).toHaveText('请假申请');
  release();
  await Promise.all([wireSettled,settled]);
  await page.evaluate(()=>new Promise<void>(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve()))));
  await expect(page.getByRole('dialog',{name:'保存预检：表单结构与布局'})).toHaveCount(0);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('definition requests bind verified actor and B cannot replay A unknown Save',async({page})=>{
  const state=await fixture(page);
  state.loseSaveResponse();
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('A 未确认字段');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('保存结果暂未确认');
  expect(state.seen.filter(item=>item.method!=='GET').every(item=>item.expectedActor===actorA)).toBe(true);
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor?.(id),actorB);
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await expect(page.getByLabel('字段名称')).toHaveCount(0);
  await expect(page.getByRole('button',{name:'查询保存结果'})).toHaveCount(0);
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor?.(id),actorA);
  await expect(page.getByLabel('字段名称')).toHaveValue('A 未确认字段');
  await page.getByRole('button',{name:'查询保存结果'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const recovery=state.seen.find(item=>item.path.includes('/application-operations/'));
  expect(recovery?.expectedActor).toBe(actorA);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(1);
});

test('401 and actor mismatch are handed to Shell without losing the designer draft',async({page})=>{
  await fixture(page);
  await page.route('**/definition/preflight',route=>route.fulfill({status:401,json:{code:'AUTH_REQUIRED',message:'',data:null,meta:{}}}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('待恢复');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('登录已失效');
  await expect(page.getByLabel('字段名称')).toHaveCount(0);
  expect(await page.evaluate(()=>(window as Window & {__formsHarnessAuthEvents?:string[]}).__formsHarnessAuthEvents)).toContain('401');
  await page.unroute('**/definition/preflight');
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('字段名称')).toHaveValue('待恢复');
  await page.route('**/definition/preflight',route=>route.fulfill({status:409,json:{code:'AUTH_SESSION_CHANGED',message:'',data:null,meta:{}}}));
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('当前账号已变化');
  expect(await page.evaluate(()=>(window as Window & {__formsHarnessAuthEvents?:string[]}).__formsHarnessAuthEvents)).toContain('AUTH_SESSION_CHANGED');
  await expect(page.getByLabel('字段名称')).toHaveCount(0);
  await page.unroute('**/definition/preflight');
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('字段名称')).toHaveValue('待恢复');
});

test('structure unknown operation stays with A through an actor switch',async({page})=>{
  const state=await fixture(page,'structure');
  await page.route('**/directories',route=>route.abort('failed'));
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('A 的目录');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor?.(id),actorB);
  await expect(page.getByRole('button',{name:'查询原操作结果'})).toHaveCount(0);
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor?.(id),actorA);
  await expect(page.getByLabel('目录名称')).toHaveValue('A 的目录');
  await expect(page.getByRole('button',{name:'查询原操作结果'})).toBeVisible();
  expect(state.seen.filter(item=>item.method==='POST').every(item=>item.expectedActor===actorA)).toBe(true);
});

test('dialog returns focus to its source after quick reverse and reopens',async({page})=>{
  await fixture(page);
  const source=page.getByRole('button',{name:'预览',exact:true});
  await source.click();
  await page.getByRole('button',{name:'关闭弹窗'}).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(source).toBeFocused();
  await source.click();
  await expect(page.getByRole('dialog',{name:'表单预览'})).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
});

test('dialog closes immediately when reduced motion is requested',async({page})=>{
  await page.emulateMedia({reducedMotion:'reduce'});
  await fixture(page);
  await page.getByRole('button',{name:'预览',exact:true}).click();
  await page.getByRole('button',{name:'关闭弹窗'}).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
});

test('owner member default picker searches, pages and saves the selected stable ID',async({page})=>{
  const member1='00000000-0000-4000-8000-000000000221';
  const member2='00000000-0000-4000-8000-000000000222';
  const state=await fixture(page);
  const queries:{q:string;token:string|null;actor:string|null}[]=[];
  await page.route('**/member-candidates?*',route=>{
    const url=new URL(route.request().url()),token=url.searchParams.get('pageToken');
    queries.push({q:url.searchParams.get('q')??'',token,actor:route.request().headers()['x-expected-actor-id']??null});
    return route.fulfill({json:{...ok({items:token?[{id:member2,label:'成员甲二',status:'active'}]:
      [{id:member1,label:'成员甲一',status:'active'}]}),
      meta:{pagination:{nextPageToken:token?null:'opaque-next',hasMore:!token}}}});
  });
  await page.getByRole('button',{name:'成员',exact:true}).click();
  await expect(page.getByRole('combobox',{name:'默认成员'})).toContainText('成员甲一');
  await page.getByRole('button',{name:'加载更多成员'}).click();
  await page.getByRole('combobox',{name:'默认成员'}).selectOption(member2);
  await page.getByLabel('搜索成员').fill(' 成员甲 ');
  await page.getByRole('button',{name:'查询成员'}).click();
  await expect.poll(()=>queries.at(-1)?.q).toBe('成员甲');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  expect((state.seen.find(item=>item.method==='PUT')?.body?.fields as Field[])[0].default).toBe(member2);
  expect(queries.every(item=>item.actor===actorA)).toBe(true);
});

test('owner department default picker displays hierarchy candidate and saves ID',async({page})=>{
  const departmentId='00000000-0000-4000-8000-000000000223';
  const state=await fixture(page);
  await page.route('**/department-candidates?*',route=>route.fulfill({json:{
    ...ok({items:[{id:departmentId,label:'财务部',parentId:null,status:'active'}]}),
    meta:{pagination:{nextPageToken:null,hasMore:false}},
  }}));
  await page.getByRole('button',{name:'部门',exact:true}).click();
  await page.getByRole('combobox',{name:'默认部门'}).selectOption(departmentId);
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  expect((state.seen.find(item=>item.method==='PUT')?.body?.fields as Field[])[0].default).toBe(departmentId);
});

test('second view can place an existing table field without creating another field ID',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000191';
  const seeded:Definition={...definition,table:{...table,schemaReady:true,schemaVersion:1},form:{...form,viewVersion:0},
    fields:[{id:fieldId,name:'申请人',kind:'text',required:false,default:null,
      config:{maxLength:null},presentation:{helpText:null,displayTimeZone:null}}],layout:[]};
  const state=await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'将已有字段加入布局 申请人'}).click();
  await expect(page.getByRole('region',{name:'表单画布'})).toContainText('申请人');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const saved=state.seen.find(item=>item.method==='PUT')!.body!;
  expect((saved.fields as Field[]).map(item=>item.id)).toEqual([fieldId]);
  expect(saved.layout).toMatchObject([{kind:'field',fieldId,span:12}]);
});

test('removing a group leaves its table field available to place again',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000192';
  const seeded:Definition={...definition,table:{...table,schemaReady:true,schemaVersion:1},form:{...form,viewVersion:1},
    fields:[{id:fieldId,name:'申请人',kind:'text',required:false,default:null,
      config:{maxLength:null},presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000193',kind:'group',title:'基本信息',span:12,
      children:[{id:'00000000-0000-4000-8000-000000000194',kind:'field',fieldId,span:12}]}]};
  const state=await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'基本信息 分组'}).click();
  await page.getByRole('button',{name:'移除布局节点'}).click();
  await page.getByRole('button',{name:'将已有字段加入布局 申请人'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const saved=state.seen.find(item=>item.method==='PUT')!.body!;
  expect((saved.fields as Field[]).map(item=>item.id)).toEqual([fieldId]);
  expect(saved.layout).toMatchObject([{kind:'field',fieldId,span:12}]);
});

test('preview explicit null selection does not bounce back to the configured default',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000195',optionId='00000000-0000-4000-8000-000000000196';
  const seeded:Definition={...definition,fields:[{id:fieldId,name:'状态',kind:'single_select',required:false,
    default:optionId,config:{options:[{id:optionId,label:'进行中'}]},
    presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000197',kind:'field',fieldId,span:12}]};
  await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'预览',exact:true}).click();
  const select=page.getByRole('dialog').getByRole('combobox',{name:'状态'});
  await expect(select).toHaveValue(optionId);
  await select.selectOption('');
  await expect(select).toHaveValue('');
});

test('malformed successful structure receipt remains unconfirmed with the original operation',async({page})=>{
  await fixture(page,'structure');
  let writes=0;
  await page.route('**/directories',route=>{writes++;return route.fulfill({status:200,json:ok({})});});
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('保留目录');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect(page.getByLabel('目录名称')).toHaveValue('保留目录');
  await expect(page.getByRole('button',{name:'按原请求重试'})).toBeVisible();
  expect(writes).toBe(1);
});

test('malformed successful definition receipt does not erase draft or issue a new key',async({page})=>{
  await fixture(page);
  let writes=0;
  await page.route('**/definition',route=>route.request().method()==='PUT'
    ?(writes++,route.fulfill({status:200,json:ok({operationId:'wrong',definition:{}})}))
    :route.fulfill({json:ok(definition)}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('保留字段');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect(page.getByLabel('字段名称')).toHaveValue('保留字段');
  await expect(page.getByRole('button',{name:'按原请求重试'})).toBeVisible();
  expect(writes).toBe(1);
});

test('null JSON success envelope retains the original Save for recovery',async({page})=>{
  await fixture(page);
  let writes=0;
  await page.route('**/definition',route=>route.request().method()==='PUT'
    ?(writes++,route.fulfill({status:200,json:null}))
    :route.fulfill({json:ok(definition)}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('回执丢失');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect(page.getByLabel('字段名称')).toHaveValue('回执丢失');
  await expect(page.getByRole('button',{name:'按原请求重试'})).toBeVisible();
  expect(writes).toBe(1);
});

test('server failure after a Save request keeps its operation until verified',async({page})=>{
  await fixture(page);
  let writes=0;
  await page.route('**/definition',route=>route.request().method()==='PUT'
    ?(writes++,route.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',message:'',data:null,meta:{}}}))
    :route.fulfill({json:ok(definition)}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('待核查字段');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect(page.getByLabel('字段名称')).toHaveValue('待核查字段');
  await expect(page.getByRole('button',{name:'按原请求重试'})).toBeVisible();
  expect(writes).toBe(1);
});

test('server failure after a directory request keeps its name and operation',async({page})=>{
  await fixture(page,'structure');
  let writes=0;
  await page.route('**/directories',route=>{writes++;return route.fulfill({status:503,
    json:{code:'COMMON_SERVICE_UNAVAILABLE',message:'',data:null,meta:{}}});});
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('待核查目录');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect(page.getByLabel('目录名称')).toHaveValue('待核查目录');
  await expect(page.getByRole('button',{name:'按原请求重试'})).toBeVisible();
  expect(writes).toBe(1);
});

test('StrictMode designer boots after effect setup cleanup setup and repeated component mounting',async({page})=>{
  await fixture(page,'designer',definition,structure,true);
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
});

test('StrictMode structure boots after effect setup cleanup setup and repeated component mounting',async({page})=>{
  await fixture(page,'structure',definition,structure,true);
  await expect(page.getByRole('region',{name:'目录与视图管理'})).toBeVisible();
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await expect(page.getByRole('region',{name:'目录与视图管理'})).toHaveCount(0);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByRole('region',{name:'目录与视图管理'})).toBeVisible();
});

test('StrictMode designer ignores a late first setup response after the replayed setup',async({page})=>{
  await fixture(page,'designer',definition,structure,true);
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  const nextView='00000000-0000-4000-8000-000000000321';
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  let count=0;
  await page.route(`**/forms/${nextView}/definition`,async route=>{
    const first=++count===1;
    if(first){started();await gate;}
    try{await route.fulfill({json:ok({...definition,form:{...form,id:nextView,name:first?'旧响应':'新响应'}})});}
    catch{ /* first StrictMode request was aborted by cleanup */ }
  });
  await page.evaluate((id:string)=>(window as Window&{__formsStrictView?:(value:string)=>void}).__formsStrictView?.(id),nextView);
  await firstStarted;
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await expect(page.locator('.forms-toolbar-title strong')).toHaveText('新响应');
  release();await page.waitForTimeout(50);
  await expect(page.locator('.forms-toolbar-title strong')).toHaveText('新响应');
  expect(count).toBeGreaterThanOrEqual(2);
});

test('StrictMode structure ignores a late first setup response after the replayed setup',async({page})=>{
  await fixture(page,'structure',definition,structure,true);
  await expect(page.getByRole('region',{name:'目录与视图管理'})).toBeVisible();
  const nextApp='00000000-0000-4000-8000-000000000322';
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  let count=0;
  await page.route(`**/applications/${nextApp}/structure`,async route=>{
    const first=++count===1;
    if(first){started();await gate;}
    try{await route.fulfill({json:ok({...structure,appId:nextApp,forms:[],tables:[],directories:first?[]:
      [{...structure.directories[0],id:folderId,appId:nextApp,name:'新目录',parentId:null,position:0}]})});}
    catch{ /* first StrictMode request was aborted by cleanup */ }
  });
  await page.evaluate((id:string)=>(window as Window&{__formsStrictApp?:(value:string)=>void}).__formsStrictApp?.(id),nextApp);
  await firstStarted;
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await expect(page.getByRole('region',{name:'目录与视图管理'})).toHaveCount(0);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByRole('treeitem',{name:'新目录'})).toBeVisible();
  release();await page.waitForTimeout(50);
  await expect(page.getByRole('treeitem',{name:'新目录'})).toBeVisible();
  expect(count).toBeGreaterThanOrEqual(2);
});

test('cached designer draft is masked when return GET is forbidden',async({page})=>{
  await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('私有草稿');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),
    '00000000-0000-4000-8000-000000000331');
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  let rechecks=0;
  await page.route(`**/forms/${viewId}/definition`,route=>{rechecks++;return route.fulfill({status:403,
    json:{code:'APPLICATION_FORBIDDEN',message:'',data:null,meta:{}}});});
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),viewId);
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('没有此应用');
  await expect(page.getByText('私有草稿')).toHaveCount(0);
  expect(rechecks).toBeGreaterThan(0);
});

test('StrictMode auth-route unmount and same actor return reverify before restoring input',async({page})=>{
  await fixture(page,'designer',definition,structure,true);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('重新登录后恢复');
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(false);
  await page.route(`**/forms/${viewId}/definition`,route=>route.fulfill({status:401,
    json:{code:'AUTH_REQUIRED',message:'',data:null,meta:{}}}));
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByRole('alert')).toContainText('登录');
  await expect(page.getByText('重新登录后恢复')).toHaveCount(0);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await page.unroute(`**/forms/${viewId}/definition`);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByLabel('字段名称')).toHaveValue('重新登录后恢复');
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(true);
});

test('cached designer masks on auth version rejection and same actor reauth restores draft',async({page})=>{
  await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('A 待恢复草稿');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),
    '00000000-0000-4000-8000-000000000332');
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await page.route(`**/forms/${viewId}/definition`,route=>route.fulfill({status:401,
    json:{code:'AUTH_REQUIRED',message:'',data:null,meta:{}}}));
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),viewId);
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('登录已失效');
  expect(await page.evaluate(()=>(window as Window&{__formsHarnessAuthEvents?:string[]}).__formsHarnessAuthEvents)).toContain('401');
  await page.unroute(`**/forms/${viewId}/definition`);
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('字段名称')).toHaveValue('A 待恢复草稿');
});

test('cached designer masks a changed actor guard before showing any old field',async({page})=>{
  await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('A 受保护草稿');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),
    '00000000-0000-4000-8000-000000000337');
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await page.route(`**/forms/${viewId}/definition`,route=>route.fulfill({status:409,
    json:{code:'AUTH_SESSION_CHANGED',message:'',data:null,meta:{}}}));
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),viewId);
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('当前账号已变化');
  await expect(page.getByText('A 受保护草稿')).toHaveCount(0);
  expect(await page.evaluate(()=>(window as Window&{__formsHarnessAuthEvents?:string[]}).__formsHarnessAuthEvents))
    .toContain('AUTH_SESSION_CHANGED');
  await page.unroute(`**/forms/${viewId}/definition`);
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('字段名称')).toHaveValue('A 受保护草稿');
});

test('cached designer masks server GET failure then preserves draft after retry',async({page})=>{
  await fixture(page);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('未覆盖输入');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),
    '00000000-0000-4000-8000-000000000333');
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await page.route(`**/forms/${viewId}/definition`,route=>route.fulfill({status:503,
    json:{code:'COMMON_SERVICE_UNAVAILABLE',message:'',data:null,meta:{}}}));
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),viewId);
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('服务暂时不可用');
  await page.unroute(`**/forms/${viewId}/definition`);
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('字段名称')).toHaveValue('未覆盖输入');
});

test('revalidated newer definition warns and keeps dirty versions without rebasing',async({page})=>{
  const seeded:Definition={...definition,table:{...table,schemaReady:true,schemaVersion:1},
    form:{...form,viewVersion:1}};
  await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('本地编辑');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),
    '00000000-0000-4000-8000-000000000338');
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await page.route(`**/forms/${viewId}/definition`,route=>route.fulfill({json:ok({...seeded,
    table:{...seeded.table,schemaVersion:2},form:{...seeded.form,viewVersion:2,name:'服务器新版'},
    fields:[],layout:[]})}));
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchView?:(id:string)=>void}).__formsHarnessSwitchView?.(id),viewId);
  await expect(page.getByRole('alert')).toContainText('服务器配置版本已变化');
  await expect(page.getByLabel('字段名称')).toHaveValue('本地编辑');
  let proposal:{expectedSchemaVersion:number;expectedViewVersion:number}|null=null;
  await page.route('**/definition/preflight',route=>{proposal=route.request().postDataJSON();
    return route.fulfill({status:409,json:{code:'APPLICATION_SCHEMA_CONFLICT',message:'',data:null,meta:{}}});});
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('配置已被其他编辑者修改');
  expect(proposal).toMatchObject({expectedSchemaVersion:1,expectedViewVersion:1});
});

test('cached structure dialog is masked after permission revocation',async({page})=>{
  await fixture(page,'structure');
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('私有目录名');
  // Observe the intermediate application before returning; setState alone is not a commit barrier.
  await page.route('**/applications/00000000-0000-4000-8000-000000000334/structure',route=>route.fulfill({json:ok({
    appId:'00000000-0000-4000-8000-000000000334',structureVersion:0,directories:[],tables:[],forms:[],capabilities:{canManageDefinition:true},
  })}));
  const otherLoaded=page.waitForResponse(response=>response.url().endsWith('/applications/00000000-0000-4000-8000-000000000334/structure')&&response.request().method()==='GET');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchApp?:(id:string)=>void}).__formsHarnessSwitchApp?.(id),
    '00000000-0000-4000-8000-000000000334');
  expect((await otherLoaded).status()).toBe(200);
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('button',{name:'新建目录',exact:true})).toBeEnabled();
  await page.route(`**/applications/${appId}/structure`,route=>route.fulfill({status:403,
    json:{code:'APPLICATION_FORBIDDEN',message:'',data:null,meta:{}}}));
  const revalidated=page.waitForResponse(response=>response.url().endsWith('/applications/'+appId+'/structure')&&response.request().method()==='GET');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchApp?:(id:string)=>void}).__formsHarnessSwitchApp?.(id),appId);
  expect((await revalidated).status()).toBe(403);
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText('私有目录名')).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('没有此应用');
});

test('revalidated newer structure retains original name and CAS version',async({page})=>{
  await fixture(page,'structure');
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('本地目录');
  // Observe the intermediate application before returning; setState alone is not a commit barrier.
  await page.route('**/applications/00000000-0000-4000-8000-000000000339/structure',route=>route.fulfill({json:ok({
    appId:'00000000-0000-4000-8000-000000000339',structureVersion:0,directories:[],tables:[],forms:[],capabilities:{canManageDefinition:true},
  })}));
  const otherLoaded=page.waitForResponse(response=>response.url().endsWith('/applications/00000000-0000-4000-8000-000000000339/structure')&&response.request().method()==='GET');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchApp?:(id:string)=>void}).__formsHarnessSwitchApp?.(id),
    '00000000-0000-4000-8000-000000000339');
  expect((await otherLoaded).status()).toBe(200);
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('button',{name:'新建目录',exact:true})).toBeEnabled();
  await page.route(`**/applications/${appId}/structure`,route=>route.fulfill({json:ok({...structure,
    structureVersion:1,directories:[{id:folderId,appId,name:'服务器目录',parentId:null,position:0}]})}));
  const revalidated=page.waitForResponse(response=>response.url().endsWith('/applications/'+appId+'/structure')&&response.request().method()==='GET');
  await page.evaluate((id:string)=>(window as Window&{__formsHarnessSwitchApp?:(id:string)=>void}).__formsHarnessSwitchApp?.(id),appId);
  expect((await revalidated).status()).toBe(200);
  await expect(page.getByLabel('目录名称')).toHaveValue('本地目录');
  await expect(page.getByRole('alert')).toContainText('目录版本已变化');
  let version:number|null=null;
  await page.route('**/directories',route=>{version=(route.request().postDataJSON() as {expectedStructureVersion:number}).expectedStructureVersion;
    return route.fulfill({status:409,json:{code:'APPLICATION_STRUCTURE_CONFLICT',message:'',data:null,meta:{}}});});
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('配置已被其他编辑者修改');
  expect(version).toBe(0);
});

for(const scenario of [
  {status:401,code:'AUTH_REQUIRED'},
  {status:403,code:'APPLICATION_FORBIDDEN'},
  {status:409,code:'AUTH_SESSION_CHANGED'},
])test(`unknown Save retry ${scenario.status} retains original operation for later confirmation`,async({page})=>{
  const state=await fixture(page);
  state.loseSaveResponse();
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('原请求字段');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  const originalId=state.seen.find(item=>item.method==='PUT')?.body?.operationId;
  const originalPacket=state.seen.find(item=>item.method==='PUT')?.body;
  let retryPacket:Record<string,unknown>|null=null;
  await page.route('**/definition',route=>route.request().method()==='PUT'
    ?(retryPacket=route.request().postDataJSON(),route.fulfill({status:scenario.status,
      json:{code:scenario.code,message:'',data:null,meta:{}}}))
    :route.fulfill({json:ok(definition)}));
  await page.getByRole('button',{name:'按原请求重试'}).click();
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await expect(page.getByRole('alert')).toBeVisible();
  await expect(page.getByRole('dialog',{name:'配置版本已变化'})).toHaveCount(0);
  await page.unroute('**/definition');
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByRole('button',{name:'查询保存结果'})).toBeVisible();
  await expect(page.getByLabel('字段名称')).toHaveValue('原请求字段');
  await page.getByRole('button',{name:'查询保存结果'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  expect(retryPacket).toEqual(originalPacket);
  expect(state.seen.find(item=>item.path.includes('/application-operations/'))?.path).toContain(String(originalId));
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(1);
});

test('StrictMode guard clears on unmount while retaining an unknown Save packet',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true);
  state.loseSaveResponse();
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('稍后核查');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(true);
  await page.getByRole('button',{name:'返回工作台'}).click();
  await page.getByRole('button',{name:'离开并保留待核查操作'}).click();
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(false);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByLabel('字段名称')).toHaveValue('稍后核查');
  await expect(page.getByRole('button',{name:'查询保存结果'})).toBeVisible();
});

test('StrictMode structure guard clears on unmount while retaining the unknown operation',async({page})=>{
  await fixture(page,'structure',definition,structure,true);
  await page.route('**/directories',route=>route.abort('failed'));
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('稍后核查目录');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(true);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(false));
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(false);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByLabel('目录名称')).toHaveValue('稍后核查目录');
  await expect(page.getByRole('button',{name:'查询原操作结果'})).toBeVisible();
});

test('unknown structure retry denial retains original directory request after revalidation',async({page})=>{
  await fixture(page,'structure');
  await page.route('**/directories',route=>route.abort('failed'));
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('原目录请求');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  await page.unroute('**/directories');
  await page.route('**/directories',route=>route.fulfill({status:403,
    json:{code:'APPLICATION_FORBIDDEN',message:'',data:null,meta:{}}}));
  await page.getByRole('button',{name:'按原请求重试'}).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByRole('alert')).toContainText('没有此应用');
  await page.unroute('**/directories');
  await page.getByRole('button',{name:'重试'}).click();
  await expect(page.getByLabel('目录名称')).toHaveValue('原目录请求');
  await expect(page.getByRole('button',{name:'查询原操作结果'})).toBeVisible();
});

test('slow preflight is abandoned on Back before PUT and guard teardown is explicit',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true);
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  await page.route('**/definition/preflight',async route=>{started();await gate;
    try{await route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,
      dataRevision:0,dependencyRevision:0,plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
      impacts:[],dependencies:[],blockingIssues:[],saveAllowed:true,confirmation:null})});}catch{/* unmounted */}});
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await firstStarted;
  await page.getByRole('button',{name:'返回工作台'}).click();
  await page.getByRole('button',{name:'放弃修改并离开'}).click();
  await expect(page.getByRole('region',{name:'字段面板'})).toHaveCount(0);
  release();await page.waitForTimeout(80);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(false);
});

test('discard cancels a gated preflight even when onBack is absent and the designer stays mounted',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true,'none');
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  await page.route('**/definition/preflight',async route=>{started();await gate;
    try{await route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,
      dataRevision:0,dependencyRevision:0,plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
      impacts:[],dependencies:[],blockingIssues:[],saveAllowed:true,confirmation:null})});}catch{/* discarded request */}});
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('已放弃的慢预检');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await firstStarted;
  await page.getByRole('button',{name:'返回工作台'}).click();
  await page.getByRole('button',{name:'放弃修改并离开'}).click();
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await expect(page.getByText('已放弃的慢预检')).toHaveCount(0);
  release();
  await page.waitForTimeout(100);
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
  await expect(page.getByText('已保存',{exact:true})).toHaveCount(0);
});

test('external designer discard clears the scoped draft before delayed unmount',async({page})=>{
  const seeded:Definition={...definition,table:{...table,schemaReady:true}};
  await fixture(page,'designer',seeded,structure,true,'none');
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('外部已放弃草稿');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('draft');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:true});
  await expect(page.getByText('外部已放弃草稿')).toHaveCount(0);
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(false));
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(true));
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await expect(page.getByText('外部已放弃草稿')).toHaveCount(0);
});

test('external structure discard clears entered directory before route remount',async({page})=>{
  await fixture(page,'structure',definition,structure,true);
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('外部已放弃目录');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('draft');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:true});
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(false));
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(true));
  await expect(page.getByRole('region',{name:'目录与视图管理'})).toBeVisible();
  await expect(page.getByLabel('目录名称')).toHaveCount(0);
  await expect(page.getByText('外部已放弃目录')).toHaveCount(0);
});

test('external preflight discard cancels PUT while route unmount is delayed',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true,'none');
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  await page.route('**/definition/preflight',async route=>{started();await gate;
    try{await route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,
      dataRevision:0,dependencyRevision:0,plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
      impacts:[],dependencies:[],blockingIssues:[],saveAllowed:true,confirmation:null})});}catch{/* aborted */}});
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await firstStarted;
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('preflight');
  const failed=page.waitForEvent('requestfailed',request=>request.url().endsWith('/definition/preflight'));
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:true});
  await failed;release();
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('external leave retains an already sent unknown Save and its original key',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true,'none');
  state.loseSaveResponse();
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('外部保留原请求');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  const original=state.seen.find(item=>item.method==='PUT')!.body!.operationId;
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('unknown');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:false,status:'unknown'});
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('unknown');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('retain_operation'))).toEqual({ok:true});
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(false));
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(true));
  await expect(page.getByLabel('字段名称')).toHaveValue('外部保留原请求');
  await page.getByRole('button',{name:'查询保存结果'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  expect(state.seen.find(item=>item.path.includes('/application-operations/'))?.path).toContain(String(original));
});

test('external leave during sent PUT retains the packet until a same-actor result lookup',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true,'none');
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  let original:string|null=null;
  await page.route('**/definition',async route=>{
    if(route.request().method()==='PUT'){
      original=(route.request().postDataJSON() as {operationId:string}).operationId;
      started();await gate;
    }
    await route.fallback();
  });
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('发送后保留');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await firstStarted;
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('write_in_flight');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard')))
    .toEqual({ok:false,status:'write_in_flight'});
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('write_in_flight');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('retain_operation'))).toEqual({ok:true});
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(false));
  const completed=page.waitForResponse(response=>response.request().method()==='PUT'&&response.url().endsWith('/definition'));
  release();await completed;
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(true));
  await expect(page.getByRole('button',{name:'查询保存结果'})).toBeVisible();
  await page.getByRole('button',{name:'查询保存结果'}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  expect(state.seen.find(item=>item.path.includes('/application-operations/'))?.path).toContain(String(original));
});

test('external structure leave keeps an unknown directory operation and entered name',async({page})=>{
  const state=await fixture(page,'structure',definition,structure,true);
  let original:string|null=null;
  await page.route('**/directories',route=>{original=(route.request().postDataJSON() as {operationId:string}).operationId;
    return route.abort('failed');});
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('外部保留目录');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('结果暂未确认');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('unknown');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('retain_operation'))).toEqual({ok:true});
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(false));
  await page.evaluate(()=>(window as GuardControl).__formsStrictMount?.(true));
  await expect(page.getByLabel('目录名称')).toHaveValue('外部保留目录');
  await page.getByRole('button',{name:'查询原操作结果'}).click();
  await expect.poll(()=>state.seen.find(item=>item.path.includes('/application-operations/'))?.path)
    .toContain(String(original));
});

test('StrictMode old same-scope unsubscribe cannot clear the current controller',async({page})=>{
  await fixture(page,'designer',definition,structure,true);
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  const current=await page.evaluate(()=>(window as GuardControl).__formsGuardActiveId?.());
  expect(current).toBeGreaterThan(1);
  await page.evaluate(()=>(window as GuardControl).__formsGuardOldUnsubscribe?.());
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardActiveId?.())).toBe(current);
});

test('external leave rejects a decision made before draft becomes preflight',async({page})=>{
  const state=await fixture(page,'designer',definition,structure,true,'none');
  let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
  let started!:()=>void;const firstStarted=new Promise<void>(resolve=>{started=resolve;});
  await page.route('**/definition/preflight',async route=>{started();await gate;
    try{await route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,
      dataRevision:0,dependencyRevision:0,plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
      impacts:[],dependencies:[],blockingIssues:[],saveAllowed:true,confirmation:null})});}catch{/* aborted */}});
  await page.getByRole('button',{name:'文本',exact:true}).click();
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('draft');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await firstStarted;
  // A status read for display must not silently replace the decision the user already saw.
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('preflight');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:false,status:'preflight'});
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('preflight');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:true});
  release();
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('external leave rejects a stale confirmation after draft content changes',async({page})=>{
  await fixture(page,'designer',{...definition,table:{...table,schemaReady:true}},structure,true,'none');
  await page.getByRole('button',{name:'文本',exact:true}).click();
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('draft');
  await page.getByLabel('字段名称').fill('确认期间新输入');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('draft');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard')))
    .toEqual({ok:false,status:'draft'});
  expect(await page.getByLabel('字段名称').inputValue()).toBe('确认期间新输入');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardStatus?.())).toBe('draft');
  expect(await page.evaluate(()=>(window as GuardControl).__formsGuardPrepare?.('discard'))).toEqual({ok:true});
  await expect(page.getByText('确认期间新输入')).toHaveCount(0);
});

test('explicit discard removes an ordinary draft while an unknown packet is never discarded implicitly',async({page})=>{
  await fixture(page,'designer',definition,structure,true);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('已放弃字段');
  await page.getByRole('button',{name:'返回工作台'}).click();
  await page.getByRole('button',{name:'放弃修改并离开'}).click();
  await expect.poll(()=>page.evaluate(()=>(window as Window&{__formsStrictDirty?:boolean}).__formsStrictDirty)).toBe(false);
  await page.evaluate(()=>(window as Window&{__formsStrictMount?:(value:boolean)=>void}).__formsStrictMount?.(true));
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await expect(page.getByText('已放弃字段')).toHaveCount(0);
});

test('datetime to non-datetime conversion clears timezone in preflight DTO',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000335';
  const seeded:Definition={...definition,table:{...table,schemaReady:true,schemaVersion:1},
    form:{...form,viewVersion:1},fields:[{id:fieldId,name:'发生时间',kind:'datetime',required:false,
      default:null,config:{precision:'second'},presentation:{helpText:'提示',displayTimeZone:'Asia/Shanghai'}}],
    layout:[{id:'00000000-0000-4000-8000-000000000336',kind:'field',fieldId,span:12}]};
  const state=await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'发生时间 日期时间'}).click();
  await page.getByLabel('字段类型').selectOption('text');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  const preflight=state.seen.find(item=>item.path.endsWith('/definition/preflight'))?.body;
  expect((preflight?.fields as Field[])[0]).toMatchObject({kind:'text',
    presentation:{helpText:'提示',displayTimeZone:null}});
});

test('existing text field can request a guarded number type conversion',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000198';
  const seeded:Definition={...definition,table:{...table,schemaReady:true,schemaVersion:1},form:{...form,viewVersion:1},
    fields:[{id:fieldId,name:'预算',kind:'text',required:false,default:null,config:{maxLength:null},
      presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000199',kind:'field',fieldId,span:12}]};
  const state=await fixture(page,'designer',seeded);
  await page.getByRole('button',{name:'预算 文本'}).click();
  await page.getByLabel('字段类型').selectOption('number');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  const preflight=state.seen.find(item=>item.path.endsWith('/definition/preflight'))!.body!;
  expect((preflight.fields as Field[])[0]).toMatchObject({id:fieldId,name:'预算',kind:'number'});
});

test('designer canvas renders the actual field control in a nonediting design state',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000200';
  const seeded:Definition={...definition,fields:[{id:fieldId,name:'申请人',kind:'text',required:false,
    default:null,config:{maxLength:null},presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000201',kind:'field',fieldId,span:12}]};
  await fixture(page,'designer',seeded);
  const control=page.getByRole('region',{name:'表单画布'}).getByRole('textbox',{name:'申请人'});
  await expect(control).toBeVisible();
  await expect(control).toBeDisabled();
});

test('changing an option mapping after successful preflight invalidates its token and pending packet',async({page})=>{
  const fieldId='00000000-0000-4000-8000-000000000211';
  const oldId='00000000-0000-4000-8000-000000000212';
  const keepId='00000000-0000-4000-8000-000000000213';
  const seeded:Definition={...definition,table:{...table,schemaVersion:1,schemaReady:true},form:{...form,viewVersion:1},
    fields:[{id:fieldId,name:'审批结果',kind:'single_select',required:false,default:null,
      config:{options:[{id:oldId,label:'旧选项'},{id:keepId,label:'保留选项'}]},
      presentation:{helpText:null,displayTimeZone:null}}],
    layout:[{id:'00000000-0000-4000-8000-000000000214',kind:'field',fieldId,span:12}]};
  const state=await fixture(page,'designer',seeded);
  await page.route('**/definition/preflight',route=>{
    const body=route.request().postDataJSON() as {optionMappings:{toOptionId:string|null}[]};
    const mapped=body.optionMappings.some(item=>item.toOptionId===keepId);
    return route.fulfill({json:ok({
    appId,tableId,viewId,schemaVersion:1,viewVersion:1,dataRevision:2,dependencyRevision:1,
    plan:{schemaChanges:[{kind:'change_config',fieldId,beforeKind:'single_select',afterKind:'single_select'}],metadataChanged:true,layoutChanged:false},
    impacts:[{fieldId,kind:'option_mapping',nonNullRows:3,optionId:oldId}],dependencies:[],
    blockingIssues:mapped?[]:[{code:'APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED',fieldIds:[fieldId]}],
    saveAllowed:mapped,confirmation:mapped?{token:'first-token',expiresAt:'2099-01-01T00:00:00Z'}:null,
  })});});
  await page.getByRole('button',{name:'审批结果 单选'}).click();
  await page.getByRole('button',{name:'删除选项 旧选项'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await page.getByLabel('已用选项映射 旧选项').selectOption(keepId);
  await page.getByRole('button',{name:'重新预检'}).click();
  await expect(page.getByRole('button',{name:'确认保存'})).toBeEnabled();
  await page.getByLabel('已用选项映射 旧选项').selectOption('__null__');
  await expect(page.getByRole('button',{name:'确认保存'})).toBeDisabled();
  await expect(page.getByRole('dialog')).toContainText('重新预检');
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});

test('expired confirmation token asks for another preflight instead of silently ignoring click',async({page})=>{
  const state=await fixture(page);
  await page.route('**/definition/preflight',route=>route.fulfill({json:ok({
    appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:1,dependencyRevision:0,
    plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
    impacts:[{fieldId:'impact',kind:'column_removal',nonNullRows:1,optionId:null}],
    dependencies:[],blockingIssues:[],saveAllowed:true,
    confirmation:{token:'short-lived',expiresAt:new Date(Date.now()+700).toISOString()},
  })}));
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('button',{name:'确认保存'})).toBeEnabled();
  await page.waitForTimeout(900);
  await page.getByRole('button',{name:'确认保存'}).click();
  await expect(page.getByRole('dialog')).toContainText('确认已过期，请重新预检');
  expect(state.seen.filter(item=>item.method==='PUT')).toHaveLength(0);
});
