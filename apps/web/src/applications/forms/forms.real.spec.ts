import {readFileSync} from 'node:fs';
import {expect,test,type Page} from '@playwright/test';
import type {Structure} from './contracts';

// Isolated V030-013 HTTP service with real PostgreSQL 18, Redis session and migrations.
// The route adapter supplies the bridge's test session because the component harness runs on HTTP.
const bridge=JSON.parse(readFileSync('/tmp/v014-browser-bridge.json','utf8')) as
  {url:string;appId:string;actorId:string;sid:string;csrf:string};
const cookie=`__Host-session=${bridge.sid}; __Host-csrf=${bridge.csrf}`;
const headers={'Origin':'https://weaveos.test','Cookie':cookie,'X-CSRF-Token':bridge.csrf,'Content-Type':'application/json'};
async function realApi(page:Page){
  let loseNextDirectoryResponse=false;
  await page.route('**/api/v1/**',async route=>{
    const request=route.request();
    const target=bridge.url+new URL(request.url()).pathname;
    const result=await fetch(target,{method:request.method(),headers:{...headers,
      'X-Expected-Actor-Id':request.headers()['x-expected-actor-id']??''},
      body:request.postData()??undefined});
    const responseBody=Buffer.from(await result.arrayBuffer());
    if(loseNextDirectoryResponse&&request.method()==='POST'&&target.endsWith('/directories')){
      loseNextDirectoryResponse=false;
      await route.abort('failed');
      return;
    }
    await route.fulfill({status:result.status,headers:{'Content-Type':'application/json'},body:responseBody});
  });
  return {loseDirectoryResponse:()=>{loseNextDirectoryResponse=true;}};
}
async function structure():Promise<Structure>{
  const response=await fetch(bridge.url+`/api/v1/applications/${bridge.appId}/structure`,{headers});
  const json=await response.json() as {data:Structure};return json.data;
}

test('real API and PG18 persist created directory, atomic form, field layout and decimal config',async({page},info)=>{
  await realApi(page);
  const suffix=`${info.project.name}-${Date.now()}`;
  const dirName=`业务-${suffix}`,formName=`申请-${suffix}`;
  await page.goto(`/src/applications/forms/harness.html?mode=structure&appId=${bridge.appId}&actorId=${bridge.actorId}`);
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill(dirName);
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('treeitem',{name:dirName,exact:true})).toBeVisible();
  await page.getByRole('treeitem',{name:dirName,exact:true}).getByRole('button').first().click();
  await page.getByRole('button',{name:'新建表单'}).click();
  await page.getByLabel('表单名称').fill(formName);
  await expect(page.getByLabel('所属目录')).not.toHaveValue('');
  await page.getByRole('button',{name:'创建表单',exact:true}).click();
  await expect(page.getByRole('treeitem',{name:formName})).toBeVisible();
  const savedStructure=await structure();
  const directory=savedStructure.directories.find(item=>item.name===dirName);
  const view=savedStructure.forms.find(item=>item.name===formName);
  expect(directory?.id).toBeTruthy();expect(view?.directoryId).toBe(directory?.id);
  const table=savedStructure.tables.find(item=>item.id===view?.tableId);
  expect(table?.name).toBe(formName);
  await page.goto(`/src/applications/forms/harness.html?mode=designer&appId=${bridge.appId}&viewId=${view!.id}&actorId=${bridge.actorId}`);
  await expect(page.getByRole('region',{name:'字段面板'})).toBeVisible();
  await page.getByRole('button',{name:'金额',exact:true}).click();
  await page.getByLabel('字段名称').fill('精确预算');
  await page.getByLabel('总精度').fill('12');
  await page.getByLabel('小数位数').fill('3');
  await page.getByLabel('处理位数').fill('-2');
  await page.getByLabel('舍入规则').selectOption('FLOOR');
  await page.getByLabel('默认值').fill('-149');
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  await page.screenshot({path:info.outputPath('real-saved-designer.png'),fullPage:true});
  await page.reload();
  await expect(page.getByRole('button',{name:'精确预算 金额'})).toBeVisible();
  await page.getByRole('button',{name:'精确预算 金额'}).click();
  // The server owns decimal normalization: FLOOR(-149, -2) is -200.000 at scale 3.
  await expect(page.getByLabel('默认值')).toHaveValue('-200.000');
  await expect(page.getByLabel('处理位数')).toHaveValue('-2');
  await page.getByRole('button',{name:'成员',exact:true}).click();
  const memberSelect=page.getByRole('combobox',{name:'默认成员'});
  await expect(memberSelect.locator('option').nth(1)).toBeAttached();
  await memberSelect.selectOption({index:1});
  const selectedMember=await memberSelect.inputValue();
  await page.getByRole('button',{name:'部门',exact:true}).click();
  const departmentSelect=page.getByRole('combobox',{name:'默认部门'});
  await expect(departmentSelect.locator('option').nth(1)).toBeAttached();
  await departmentSelect.selectOption({index:1});
  const selectedDepartment=await departmentSelect.inputValue();
  await page.getByRole('button',{name:'保存',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('已保存');
  await page.reload();
  await page.getByRole('button',{name:'新建成员 成员'}).click();
  await expect(page.getByLabel('当前默认引用 ID')).toHaveValue(selectedMember);
  await page.getByRole('button',{name:'新建部门 部门'}).click();
  await expect(page.getByLabel('当前默认引用 ID')).toHaveValue(selectedDepartment);
});

test('real operation recovery confirms a committed directory after its response is lost',async({page},info)=>{
  const proxy=await realApi(page);
  const name=`恢复-${info.project.name}-${Date.now()}`;
  await page.goto(`/src/applications/forms/harness.html?mode=structure&appId=${bridge.appId}&actorId=${bridge.actorId}`);
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill(name);
  proxy.loseDirectoryResponse();
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('操作结果暂未确认');
  await expect(page.getByLabel('目录名称')).toHaveValue(name);
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor?.(id),
    '00000000-0000-4000-8000-000000000182');
  await expect(page.getByRole('alert')).toContainText('当前账号已变化');
  await expect(page.getByRole('button',{name:'查询原操作结果'})).toHaveCount(0);
  await page.evaluate((id:string)=>(window as Window & {__formsHarnessSwitchActor?:(id:string)=>void}).__formsHarnessSwitchActor?.(id),bridge.actorId);
  await expect(page.getByLabel('目录名称')).toHaveValue(name);
  await page.getByRole('button',{name:'查询原操作结果'}).click();
  await expect(page.getByRole('treeitem',{name,exact:true})).toBeVisible();
  expect((await structure()).directories.filter(item=>item.name===name)).toHaveLength(1);
});

test('real concurrent structure write returns CAS conflict without discarding entered name',async({page},info)=>{
  await realApi(page);
  const name=`冲突-${info.project.name}-${Date.now()}`;
  await page.goto(`/src/applications/forms/harness.html?mode=structure&appId=${bridge.appId}&actorId=${bridge.actorId}`);
  await page.getByRole('button',{name:'新建目录'}).click();
  await page.getByLabel('目录名称').fill(name);
  const current=await structure();
  const other=await fetch(bridge.url+`/api/v1/applications/${bridge.appId}/directories`,{
    method:'POST',headers,body:JSON.stringify({operationId:crypto.randomUUID(),name:`其他编辑者-${name}`,
      parentId:null,position:999,expectedStructureVersion:current.structureVersion}),
  });
  expect(other.status).toBe(201);
  await page.getByRole('button',{name:'创建目录',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('配置已被其他编辑者修改');
  await expect(page.getByLabel('目录名称')).toHaveValue(name);
  await expect(page.getByRole('button',{name:'放弃输入并加载最新版'})).toBeVisible();
  expect((await structure()).directories.filter(item=>item.name===name)).toHaveLength(0);
});
