import { expect, test, type Page } from '@playwright/test';
import type { Definition, Field, LayoutNode, Structure } from './contracts';

// Independent oracle: V030-014 PRD/ADR (2026-10-03), accepted V030-013 ADR,
// original Figma 327:3107, 332:5028, 332:5461, 337:10258, 340:18187.
const appId = '00000000-0000-4000-8000-000000000101';
const tableId = '00000000-0000-4000-8000-000000000102';
const viewId = '00000000-0000-4000-8000-000000000103';
const folderId = '00000000-0000-4000-8000-000000000104';
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
async function fixture(page:Page, mode:'designer'|'structure'='designer') {
  const seen:Seen[]=[];
  let currentDefinition=structuredClone(definition);
  let currentStructure=structuredClone(structure);
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
      currentStructure={...currentStructure,structureVersion:1,directories:[directory]};
      return route.fulfill({status:201,json:ok({directory,structureVersion:1})});
    }
    if(path.endsWith('/forms')&&method==='POST'){
      return route.fulfill({status:201,json:ok({table,form,structureVersion:1})});
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
  await page.getByLabel('目录名称').fill('请假');
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('treeitem',{name:'请假'})).toBeVisible();
  await page.getByRole('button',{name:'新建表单'}).click();
  await page.getByLabel('表单名称').fill('请假申请');
  await page.getByRole('button',{name:'创建表单',exact:true}).click();
  const creates=state.seen.filter(x=>x.method==='POST');
  expect(creates.map(x=>x.path)).toEqual([
    `/api/v1/applications/${appId}/directories`,
    `/api/v1/applications/${appId}/forms`,
  ]);
  expect(creates[1].body).toMatchObject({name:'请假申请',source:{kind:'new_table'},expectedStructureVersion:1});
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
