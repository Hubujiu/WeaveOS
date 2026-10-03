import { expect, test, type Page } from '@playwright/test';
import type { Definition, Field, LayoutNode, Structure } from './contracts';

// Independent oracle: V030-014 PRD/ADR (2026-10-03), accepted V030-013 ADR,
// original Figma 327:3107, 332:5028, 332:5461, 337:10258, 340:18187.
const appId = '00000000-0000-4000-8000-000000000101';
const tableId = '00000000-0000-4000-8000-000000000102';
const viewId = '00000000-0000-4000-8000-000000000103';
const folderId = '00000000-0000-4000-8000-000000000104';
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

type Seen = {method:string;path:string;body:Record<string,unknown>|null};
async function fixture(page:Page, mode:'designer'|'structure'='designer', initialDefinition:Definition=definition,
  initialStructure:Structure=structure) {
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
    seen.push({method,path,body});
    if(path.endsWith('/structure')&&method==='GET')return route.fulfill({json:ok(currentStructure)});
    if(path.endsWith('/definition')&&method==='GET')return route.fulfill({json:ok(currentDefinition)});
    if(path.endsWith('/definition/preflight')&&method==='POST')return route.fulfill({json:ok({
      appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:0,dependencyRevision:0,
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
  await page.goto(`/src/applications/forms/harness.html?mode=${mode}&appId=${appId}&viewId=${viewId}`);
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
  await page.screenshot({path:test.info().outputPath('designer-main.png'),fullPage:true});
  await page.getByRole('button',{name:'预览',exact:true}).click();
  await expect(page.getByText('本地预览，尚未保存')).toBeVisible();
  await expect(page.getByRole('dialog').getByText('申请人')).toBeVisible();
  expect(state.seen.filter(x=>x.method!=='GET')).toHaveLength(0);
  await page.screenshot({path:test.info().outputPath('designer-preview.png'),fullPage:true});
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
  await page.screenshot({path:test.info().outputPath('directory-create.png'),fullPage:true});
  await page.getByLabel('目录名称').fill('请假');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('treeitem',{name:'请假',exact:true})).toBeVisible();
  await page.screenshot({path:test.info().outputPath('directory-tree.png'),fullPage:true});
  await page.getByRole('button',{name:'新建表单'}).click();
  await page.screenshot({path:test.info().outputPath('form-create.png'),fullPage:true});
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
  await page.screenshot({path:test.info().outputPath('impact-missing-token.png'),fullPage:true});
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
  await page.screenshot({path:test.info().outputPath('designer-dependency-block.png'),fullPage:true});
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
  await page.screenshot({path:test.info().outputPath('option-mapping.png'),fullPage:true});
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
    return route.fulfill({json:ok({appId,tableId,viewId,schemaVersion:0,viewVersion:0,dataRevision:0,dependencyRevision:0,
      plan:{schemaChanges:[],metadataChanged:true,layoutChanged:true},
      impacts:[{fieldId:'field-old',kind:'column_removal',nonNullRows:1,optionId:null}],
      dependencies:[],blockingIssues:[],saveAllowed:true,
      confirmation:{token:'old-view-token',expiresAt:'2099-01-01T00:00:00Z'}})});
  });
  await page.route(`**/forms/${nextViewId}/definition`,route=>route.fulfill({json:ok({...definition,form:{...form,id:nextViewId,name:'另一个视图'}})}));
  await page.goto(`/src/applications/forms/harness.html?mode=designer&appId=${appId}&viewId=${viewId}&switchViewId=${nextViewId}`);
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await started;
  await page.getByRole('button',{name:'切换视图'}).click();
  await expect(page.getByRole('region',{name:'表单画布'})).toContainText('另一个视图');
  const oldReply=page.waitForResponse(response=>response.url().includes(`/forms/${viewId}/definition/preflight`));
  release();
  await oldReply;
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

test('permission revoked during Save keeps the draft but stops further configuration writes',async({page})=>{
  const state=await fixture(page);
  let writes=0;
  await page.route('**/definition',route=>{if(route.request().method()!=='PUT')return route.fallback();writes++;
    return route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',message:'forbidden',data:null,meta:{requestId:'forbidden'}}});});
  await page.getByRole('button',{name:'文本',exact:true}).click();
  await page.getByLabel('字段名称').fill('保留草稿');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('没有此应用的表单配置权限');
  await expect(page.getByLabel('字段名称')).toHaveValue('保留草稿');
  await page.screenshot({path:test.info().outputPath('permission-revoked.png'),fullPage:true});
  await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();
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
  await page.screenshot({path:test.info().outputPath('impact-confirm.png'),fullPage:true});
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
  await page.screenshot({path:test.info().outputPath('dirty-exit.png'),fullPage:true});
  await page.getByRole('button',{name:'继续编辑'}).click();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('dialog',{name:'配置版本已变化'})).toBeVisible();
  await expect(page.getByLabel('字段名称')).toHaveValue('并发中的草稿');
  await page.screenshot({path:test.info().outputPath('schema-conflict.png'),fullPage:true});
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
  await page.screenshot({path:test.info().outputPath('structure-conflict.png'),fullPage:true});
});

test('structure write permission revocation disables further directory and form mutations',async({page})=>{
  await fixture(page,'structure');
  await page.route('**/directories',route=>route.request().method()==='POST'
    ?route.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',message:'forbidden',data:null,meta:{requestId:'denied'}}})
    :route.fallback());
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill('保留输入');
  await page.getByRole('button',{name:'创建目录'}).click();
  await expect(page.getByRole('dialog')).toContainText('没有此应用的表单配置权限');
  await expect(page.getByLabel('目录名称')).toHaveValue('保留输入');
  await page.getByRole('button',{name:'取消'}).click();
  await expect(page.getByRole('button',{name:'新建目录'})).toBeDisabled();
  await expect(page.getByRole('button',{name:'新建表单'})).toBeDisabled();
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
  await page.screenshot({path:test.info().outputPath('structure-empty.png'),fullPage:true});
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
  await page.getByLabel('目录名称').fill('已改名');
  await page.getByRole('button',{name:'保存变更'}).click();
  await expect(page.getByRole('treeitem',{name:'已改名',exact:true})).toBeVisible();
  await page.getByRole('treeitem',{name:'父目录',exact:true}).getByRole('button').first().click();
  await page.getByRole('button',{name:'移动目录',exact:true}).click();
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
