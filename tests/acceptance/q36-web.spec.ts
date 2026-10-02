import {test,expect,type Page,type BrowserContext,type APIResponse} from '@playwright/test';
import {readFileSync,mkdirSync} from 'node:fs';
import {rename} from 'node:fs/promises';
import {resolve} from 'node:path';
import type {Member,Definition,Department} from '../../apps/web/src/workspace-types';
import type {MembersQueryPage,PersonnelDraft,PersonnelDraftSummary} from '../../apps/web/src/query-contracts';

// Approved Q36 PLAN §§1–6. All requests reach the isolated BFF/PG/Redis;
// there are no business API mocks. No credential-bearing traces or videos.
test.use({trace:'off',screenshot:'off',video:'off'});
test.describe.configure({mode:'serial'});
const base=process.env.WEAVEOS_WEB_URL||'https://localhost:19444';
if(!['localhost','127.0.0.1'].includes(new URL(base).hostname))throw Error('Q36 requires an isolated loopback instance');
const privateFixtures=process.env.WEAVEOS_ACCEPTANCE_FIXTURES||resolve('.work/q36-b2-runtime/fixtures.json');
const evidence=resolve('docs/evidence/V010-020/filter-manager/real/q36-regression');mkdirSync(evidence,{recursive:true});
const fixture=JSON.parse(readFileSync(privateFixtures,'utf8')) as {admin:{account:string;password:string};user:{account:string;password:string};userId:string};
let identity:Definition,template:Definition,department:Department,rootDepartment:Department;
const stamp=Date.now();let presetCounter=0;const createdPresets:string[]=[];
async function login(context:BrowserContext,credentials=fixture.admin){const response=await context.request.post(base+'/api/v1/sessions',{headers:{Origin:base},data:credentials});expect(response.status(),'isolated prerequisite login').toBe(201);}
async function response(page:Page,path:string,method='GET',data?:object):Promise<APIResponse>{
 const cookie=(await page.context().cookies(base)).find(c=>c.name==='__Host-csrf');
 return page.request.fetch(base+'/api/v1/'+path,{method,headers:{Origin:base,...(cookie?{'X-CSRF-Token':cookie.value}:{})},...(data?{data}:{})});
}
async function api<T>(page:Page,path:string,method='GET',data?:object):Promise<T>{const res=await response(page,path,method,data);expect(res.ok(),'isolated '+method+' '+path.split('?')[0]+' status '+res.status()).toBe(true);if(res.status()===204)return undefined as T;const body=await res.json();expect(body.code).toBe('OK');return body.data as T;}
async function enter(page:Page){await login(page.context());page.on('dialog',dialog=>void dialog.accept());await page.goto('/app/admin');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();await expect(page.locator('.member-table tbody tr[data-row-id]').first()).toBeVisible();}
async function shot(page:Page,name:string,filterOpen=false){
 await page.evaluate(()=>document.fonts.ready);
 // Base UI intentionally remains open while the native close animation runs.
 // Assert the requested screenshot state instead of treating its aria-expanded
 // flag as the user's intent during that interval.
 if(filterOpen){await expect(page.locator('.preset-popup')).toHaveAttribute('aria-hidden','false');await expect(page.locator('.q36-filter-content')).toBeVisible();}
 else await expect(page.getByRole('dialog',{name:'管理自定义筛选',exact:true})).toBeHidden();
 await page.evaluate(()=>new Promise<void>(done=>requestAnimationFrame(()=>requestAnimationFrame(()=>done()))));
 await page.waitForFunction(()=>!document.documentElement.classList.contains('q36-filter-transition-active'));
 await page.evaluate(()=>Promise.allSettled(document.getAnimations().map(animation=>animation.finished)));
 await page.screenshot({path:resolve(evidence,test.info().project.name+'-'+name+'.png'),fullPage:true});
}
async function newFilterEditor(page:Page){
 await page.getByRole('button',{name:'自定义筛选',exact:true}).click();const manager=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await manager.getByRole('button',{name:'新增筛选',exact:true}).click();const editor=page.getByRole('dialog',{name:'新增自定义筛选',exact:true});const name='Q36-'+test.info().project.name+'-'+stamp+'-'+(++presetCounter);await editor.getByLabel('自定义筛选名称',{exact:true}).fill(name);return {manager,editor,name};
}
async function saveApplyFilter(page:Page,entry:Awaited<ReturnType<typeof newFilterEditor>>){
 const saved=page.waitForResponse(r=>r.request().method()==='POST'&&new URL(r.url()).pathname.endsWith('/table-presets'));await entry.editor.getByRole('button',{name:'确定',exact:true}).click();const body=await (await saved).json();createdPresets.push(body.data.id);await entry.manager.getByRole('button',{name:'应用'+entry.name,exact:true}).click();await expect(entry.manager.getByRole('button',{name:'关闭筛选管理',exact:true})).toBeEnabled();await entry.manager.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await expect(entry.manager).toBeHidden();
}
async function cancelFilter(page:Page){
 await page.getByRole('button',{name:'自定义筛选，已应用',exact:true}).click();const manager=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await manager.getByRole('button',{name:'取消应用',exact:true}).click();await page.getByRole('dialog',{name:'取消当前筛选',exact:true}).getByRole('button',{name:'确认取消',exact:true}).click();await expect(manager.getByRole('button',{name:'关闭筛选管理',exact:true})).toBeEnabled();await manager.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await expect(manager).toBeHidden();
}
test.afterEach(async({page})=>{for(const id of createdPresets.splice(0)){const stored=await api<{version:number}>(page,'personnel/table-presets/'+id);await api(page,'personnel/table-presets/'+id+'?version='+stored.version,'DELETE');}});

async function refresh(page:Page){await page.getByRole('button',{name:'刷新查询',exact:true}).first().click();await expect(page.locator('.member-table tbody tr[data-row-id]').first()).toBeVisible();}
async function saved(page:Page){await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'草稿已保存'})).toBeVisible();}
async function closeSavedForm(page:Page){await page.getByRole('dialog').getByRole('button',{name:'关闭',exact:true}).click();const dirty=page.getByRole('dialog',{name:'有未保存的修改',exact:true});if(await dirty.isVisible())await dirty.getByRole('button',{name:'放弃修改',exact:true}).click();}
async function restoreKind(page:Page,kind:string){await page.getByRole('button',{name:'草稿箱',exact:true}).click();const row=page.getByRole('dialog',{name:'草稿箱',exact:true}).locator('.personnel-draft-item').filter({has:page.getByText(kind,{exact:true})});await row.getByRole('button',{name:'恢复草稿',exact:true}).first().click();}

test.beforeAll(async({browser})=>{
 const context=await browser.newContext({ignoreHTTPSErrors:true});await login(context);const page=await context.newPage();
 // This private synthetic account is reused when the cloud matrix is rerun.
 // Establish the suite's empty draft precondition before its serial lifecycle;
 // preserve all five kinds throughout the subsequent fresh-login assertions.
 const priorDrafts=await api<{items:PersonnelDraftSummary[]}>(page,'personnel/drafts');
 for(const item of priorDrafts.items)await api(page,'personnel/drafts/'+item.id+'?version='+item.version,'DELETE');
 const query=await api<MembersQueryPage<Member>>(page,'personnel/members/search','POST',{page:1,pageSize:20});
 rootDepartment=(await api<{items:Department[]}>(page,'personnel/departments')).items.find(d=>d.isRoot)!;
 identity=await api<Definition>(page,'personnel/identities','POST',{name:'Q36 测试身份 '+stamp,description:'原说明',templateIds:[],permissionCodes:[]});
 template=await api<Definition>(page,'personnel/templates','POST',{name:'Q36 测试模板 '+stamp,description:'原模板说明',permissionCodes:[]});
 department=await api<Department>(page,'personnel/departments','POST',{name:'Q36 测试部门 '+stamp,parentId:rootDepartment.id,queryVersion:query.queryVersion});
 // Populate two real pages with synthetic registrations via supported APIs.
 for(let i=0;i<24;i++){
  const invite=await api<{invitationCode:string}>(page,'invitations','POST',{});
  const res=await page.request.post(base+'/api/v1/registrations',{headers:{Origin:base},data:{account:'q36-'+stamp+'-'+String(i).padStart(2,'0'),password:'Synthetic@Test123',invitationCode:invite.invitationCode}});
  expect(res.status(),'isolated synthetic registration').toBe(201);
 }
 await context.close();
});

test('Q36 actual members: complete filtering, arbitrary pages, selection, column preservation and screenshots',async({page})=>{
 await enter(page);await page.setViewportSize({width:1440,height:1000});await shot(page,'members-default-desktop');await page.setViewportSize({width:390,height:844});await shot(page,'members-default-narrow');await page.setViewportSize({width:1440,height:1000});
 await expect(page.locator('.member-table th[aria-sort]')).toHaveCount(0);
 const selected=page.getByLabel('选择成员：'+fixture.admin.account,{exact:true});await selected.check();
 const resize=page.getByRole('button',{name:'Resize account column',exact:true});const box=(await resize.boundingBox())!;await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.down();await page.mouse.move(box.x+box.width/2+50,box.y+box.height/2,{steps:8});await page.mouse.up();
 const before=(await page.locator('.member-table th').nth(1).boundingBox())!.width;
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByLabel('跳至页',{exact:true}).press('Enter');await expect(page.getByRole('button',{name:'Page 2',exact:true})).toHaveAttribute('aria-current','page');
 const entry=await newFilterEditor(page),panel=entry.editor;
 await panel.getByRole('button',{name:'或条件',exact:true}).click();await panel.getByLabel('条件 1.1 值',{exact:true}).fill(fixture.admin.account);await shot(page,'members-filter-expanded-desktop',true);
 await saveApplyFilter(page,entry);await expect(page.locator('.member-table tbody tr[data-row-id]')).toHaveCount(1);await shot(page,'members-filter-applied-desktop');
 await refresh(page);await expect(selected).not.toBeChecked();expect((await page.locator('.member-table th').nth(1).boundingBox())!.width).toBeCloseTo(before,0);await expect(page.getByRole('button',{name:'自定义筛选，已应用',exact:true})).toBeVisible();
 await page.setViewportSize({width:390,height:844});await shot(page,'members-filter-applied-narrow');await page.getByRole('button',{name:'自定义筛选，已应用',exact:true}).click();await shot(page,'members-filter-expanded-narrow',true);await page.keyboard.press('Escape');
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('Q36 actual events: backend safe display, absolute range, full-result time ordering and screenshots',async({page})=>{
 await enter(page);await page.setViewportSize({width:1440,height:1000});await page.getByRole('tab',{name:'操作记录',exact:true}).click();await expect(page.locator('.activity-table tbody tr[data-row-id]').first()).toBeVisible();await shot(page,'events-default-desktop');await page.setViewportSize({width:390,height:844});await shot(page,'events-default-narrow');await page.setViewportSize({width:1440,height:1000});
 await page.getByRole('button',{name:'排序 occurredAt',exact:true}).click();await page.getByRole('menuitem',{name:'升序',exact:true}).click();await expect(page.locator('.activity-table th[aria-sort]')).toHaveAttribute('aria-sort','ascending');
 const entry=await newFilterEditor(page),panel=entry.editor;await panel.getByRole('button',{name:'或条件',exact:true}).click();await panel.getByLabel('条件 1.1 值',{exact:true}).fill(fixture.admin.account);await shot(page,'events-filter-expanded-desktop',true);await saveApplyFilter(page,entry);await shot(page,'events-filter-applied-desktop');
 const result=await api<{items:{occurredAt:string;display:{object:string;detail:string}}[];range:{from:string;to:string}}>(page,'personnel/events/search','POST',{page:1,pageSize:100,sortBy:'occurredAt',sortDirection:'asc',filter:{operator:'and',children:[{field:'actorAccount',operator:'eq',value:fixture.admin.account}]}});
 expect(result.items.length).toBeGreaterThan(0);expect(result.items.every((event,i,all)=>i===0||Date.parse(all[i-1].occurredAt)<=Date.parse(event.occurredAt))).toBe(true);expect(result.items.every(event=>typeof event.display.detail==='string'&&typeof event.display.object==='string')).toBe(true);expect(Date.parse(result.range.to)).toBeGreaterThan(Date.parse(result.range.from));
 await page.setViewportSize({width:390,height:844});await shot(page,'events-filter-applied-narrow');await page.getByRole('button',{name:'自定义筛选，已应用',exact:true}).click();await shot(page,'events-filter-expanded-narrow',true);await page.keyboard.press('Escape');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('Q36 actual precision: unrelated option change proceeds; related member change explicitly refreshes',async({page})=>{
 await enter(page);let currentVersion='';page.on('response',async res=>{if(res.url().endsWith('/personnel/members/search')&&res.ok()){try{currentVersion=(await res.json()).data.queryVersion;}catch{/* An obsolete response aborted by refresh has no readable body. */}}});await refresh(page);await expect.poll(()=>currentVersion.length).toBeGreaterThan(0);
 await api(page,'personnel/templates','POST',{name:'Q36 无关模板 '+stamp,description:'不影响当前成员结果',permissionCodes:[]});
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByLabel('跳至页',{exact:true}).press('Enter');await expect(page.getByRole('button',{name:'Page 2',exact:true})).toHaveAttribute('aria-current','page');await expect(page.getByText('查询结果已变化，请刷新查询后继续',{exact:true})).toHaveCount(0);
 const target=await api<Member>(page,'personnel/members/'+fixture.userId);await api(page,'personnel/members/'+target.id+'/identities','PUT',{version:target.version,identityIds:[identity.id],queryVersion:currentVersion});
 await page.getByLabel('跳至页',{exact:true}).fill('1');await page.getByLabel('跳至页',{exact:true}).press('Enter');await expect(page.getByRole('alert')).toContainText('查询结果已变化');await shot(page,'members-refresh-required-desktop');await page.setViewportSize({width:390,height:844});await shot(page,'members-refresh-required-narrow');await refresh(page);await expect(page.getByRole('button',{name:'Page 1',exact:true})).toHaveAttribute('aria-current','page');
});

test('Q36 actual context LRU expiry is distinct from changed results',async({page})=>{
 await enter(page);for(let i=0;i<21;i++)await api(page,'personnel/members/search','POST',{page:1,pageSize:20,search:'unmatched-context-'+i});
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByLabel('跳至页',{exact:true}).press('Enter');await expect(page.getByRole('alert')).toContainText('查询上下文已过期');await shot(page,'members-context-expired');await refresh(page);
});

test('Q36 actual five draft kinds persist after fresh login; member/department inputs survive query refresh',async({page,browser})=>{
 await enter(page);await page.getByRole('button',{name:'配置身份',exact:true}).first().click();await page.getByLabel('身份：'+identity.name,{exact:true}).check();await saved(page);await shot(page,'draft-member-identities-saved');await closeSavedForm(page);
 await page.getByRole('button',{name:'调整分组',exact:true}).first().click();await saved(page);await shot(page,'draft-member-groups-saved');await closeSavedForm(page);
 await page.getByRole('button',{name:'新建部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('Q36 未提交部门');await saved(page);await shot(page,'draft-department-saved');await closeSavedForm(page);
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();await page.getByLabel('说明',{exact:true}).fill('Q36 已保存身份输入');await saved(page);await shot(page,'draft-identity-saved');await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'放弃修改',exact:true}).click();
 await page.getByRole('button',{name:template.name,exact:true}).click();await page.getByLabel('说明',{exact:true}).fill('Q36 已保存模板输入');await saved(page);await shot(page,'draft-template-saved');
 const stored=await api<{items:PersonnelDraftSummary[]}>(page,'personnel/drafts');expect(new Set(stored.items.map(d=>d.kind))).toEqual(new Set(['member-identities','member-groups','department','identity','template']));
 const context=await browser.newContext({ignoreHTTPSErrors:true});const other=await context.newPage();await enter(other);await restoreKind(other,'部门');await expect(other.getByLabel('部门名称',{exact:true})).toHaveValue('Q36 未提交部门');await shot(other,'draft-department-restored-desktop');await other.setViewportSize({width:390,height:844});await shot(other,'draft-department-restored-narrow');
 await other.getByRole('dialog').getByRole('button',{name:'刷新查询',exact:true}).click();await expect(other.getByLabel('部门名称',{exact:true})).toHaveValue('Q36 未提交部门');await context.close();
});

test('Q36 actual restoration compares latest target, preserves original base, and requires explicit resolution',async({page})=>{
 await enter(page);identity=await api<Definition>(page,'personnel/identities/'+identity.id,'PUT',{name:identity.name,description:'另一标签已提交最新说明',templateIds:[],permissionCodes:[],version:identity.version});
 await restoreKind(page,'身份');await expect(page.getByLabel('说明',{exact:true})).toHaveValue('Q36 已保存身份输入');await expect(page.getByRole('alert')).toContainText('原版本 1');await expect(page.getByRole('alert')).toContainText('最新版本 2');await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();await shot(page,'draft-target-conflict-desktop');await page.setViewportSize({width:390,height:844});await shot(page,'draft-target-conflict-narrow');
 const list=await api<{items:PersonnelDraftSummary[]}>(page,'personnel/drafts');const item=list.items.find(d=>d.kind==='identity'&&d.targetId===identity.id)!;const stored=await api<PersonnelDraft>(page,'personnel/drafts/'+item.id);expect(stored.baseVersion).toBe(1);
 await page.getByRole('button',{name:'保留当前输入，使用最新对象版本',exact:true}).click();await expect(page.getByLabel('说明',{exact:true})).toHaveValue('Q36 已保存身份输入');await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'已保存'})).toBeVisible();
 expect((await response(page,'personnel/drafts/'+item.id)).status()).toBe(404);const latest=await api<Definition>(page,'personnel/identities/'+identity.id);expect(latest.description).toBe('Q36 已保存身份输入');identity=latest;
});

test('Q36 actual draft CAS conflict preserves input; business success keeps a newer draft version',async({page})=>{
 await enter(page);await restoreKind(page,'权限模板');const items=(await api<{items:PersonnelDraftSummary[]}>(page,'personnel/drafts')).items;const original=await api<PersonnelDraft>(page,'personnel/drafts/'+items.find(d=>d.kind==='template')!.id);
 if(original.kind!=='template')throw Error('wrong isolated fixture kind');
 await api(page,'personnel/drafts/'+original.id,'PUT',{version:original.version,payload:{...original.payload,description:'其他标签草稿输入'}});
 await page.getByLabel('说明',{exact:true}).fill('本标签未保存输入');await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect(page.getByRole('alert')).toContainText('草稿已由其他标签页修改');await expect(page.getByLabel('说明',{exact:true})).toHaveValue('本标签未保存输入');await shot(page,'draft-cas-conflict-desktop');await page.setViewportSize({width:390,height:844});await shot(page,'draft-cas-conflict-narrow');
 await page.getByRole('button',{name:'保留当前输入，采用最新草稿版本',exact:true}).click();await saved(page);
 const adopted=await api<PersonnelDraft>(page,'personnel/drafts/'+original.id);expect(adopted.version).toBe(original.version+2);expect(adopted.baseVersion).toBe(original.baseVersion);
 if(adopted.kind!=='template')throw Error('wrong isolated fixture kind');const newer=await api<PersonnelDraft>(page,'personnel/drafts/'+original.id,'PUT',{version:adopted.version,payload:{...adopted.payload,description:'必须保留的新草稿'}});
 await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'已保存'})).toBeVisible();const preserved=await api<PersonnelDraft>(page,'personnel/drafts/'+original.id);expect(preserved.version).toBe(newer.version);expect(preserved.payload).toMatchObject({description:'必须保留的新草稿'});
});

test('Q36 actual drafts remain isolated between two currently authorized accounts',async({page,browser})=>{
 await enter(page);const foreign=(await api<{items:PersonnelDraftSummary[]}>(page,'personnel/drafts')).items[0];expect(foreign).toBeTruthy();
 const target=await api<Member>(page,'personnel/members/'+fixture.userId);
 const grant=await api<Definition>(page,'personnel/identities','POST',{name:'Q36 隔离验证授权 '+stamp,description:'仅测试账号',templateIds:[],permissionCodes:['personnel.manage']});
 const query=await api<MembersQueryPage<Member>>(page,'personnel/members/search','POST',{page:1,pageSize:20});
 const granted=await api<Member>(page,'personnel/members/'+target.id+'/identities','PUT',{identityIds:[...target.identityIds,grant.id],version:target.version,queryVersion:query.queryVersion});
 const context=await browser.newContext({ignoreHTTPSErrors:true});
 try{
  await login(context,fixture.user);const other=await context.newPage();await other.goto('/app/admin');await expect(other.getByRole('button',{name:'草稿箱',exact:true})).toBeVisible();
  const own=(await api<{items:PersonnelDraftSummary[]}>(other,'personnel/drafts')).items;expect(own.some(d=>d.id===foreign.id)).toBe(false);
  for(const method of ['GET','PUT','DELETE']){
   const denied=await response(other,'personnel/drafts/'+foreign.id+(method==='DELETE'?'?version='+foreign.version:''),method,method==='PUT'?{version:foreign.version,payload:{name:'禁止跨账户覆盖',parentId:null}}:undefined);
   expect(denied.status(),'foreign draft '+method).toBe(404);
  }
  await other.getByRole('tab',{name:'权限模板',exact:true}).click();await other.getByRole('button',{name:'新建权限模板',exact:true}).click();await other.getByLabel('模板名称',{exact:true}).fill('Q36 本账号草稿');await saved(other);
  await other.getByRole('button',{name:'草稿箱',exact:true}).click();await other.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'放弃修改',exact:true}).click();
  await expect(other.getByRole('dialog',{name:'草稿箱',exact:true}).locator('.personnel-draft-item')).toHaveCount(own.length+1);await shot(other,'draft-account-isolation');
  const unchanged=await api<PersonnelDraft>(page,'personnel/drafts/'+foreign.id);expect(unchanged.version).toBe(foreign.version);
 }finally{
  await context.close();const latest=await api<Member>(page,'personnel/members/'+granted.id);const current=await api<MembersQueryPage<Member>>(page,'personnel/members/search','POST',{page:1,pageSize:20});
  await api(page,'personnel/members/'+latest.id+'/identities','PUT',{identityIds:target.identityIds,version:latest.version,queryVersion:current.queryVersion});
 }
});

test('Q36 actual narrow finite AND OR groups validate twenty leaves and keep footer reachable',async({page})=>{
 await enter(page);await page.setViewportSize({width:1440,height:1000});const entry=await newFilterEditor(page),panel=entry.editor;
 await panel.getByRole('button',{name:'或条件',exact:true}).click();await panel.getByLabel('条件 1.1 字段',{exact:true}).selectOption('identityIds');await panel.getByRole('button',{name:'确定',exact:true}).click();await expect(panel.getByRole('alert')).toContainText('条件 1.1');await shot(page,'filter-invalid-desktop',true);
 await panel.getByLabel('条件 1.1 字段',{exact:true}).selectOption('account');await panel.getByLabel('条件 1.1 值',{exact:true}).fill(fixture.admin.account);
 for(let i=2;i<=20;i++){await panel.getByRole('button',{name:'组 1 且条件',exact:true}).click();await panel.getByLabel('条件 1.'+i+' 值',{exact:true}).fill(fixture.admin.account);}
 await expect(panel.getByRole('button',{name:'或条件',exact:true})).toBeDisabled();await expect(panel.getByRole('button',{name:'确定',exact:true})).toBeEnabled();await shot(page,'filter-long-groups-desktop',true);
 await page.setViewportSize({width:390,height:844});await shot(page,'filter-long-groups-narrow',true);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 const footer=(await panel.getByRole('button',{name:'确定',exact:true}).boundingBox())!;expect(footer.y+footer.height).toBeLessThanOrEqual(844);
 await saveApplyFilter(page,entry);await expect(page.locator('.member-table tbody tr[data-row-id]')).toHaveCount(1);
 await page.getByLabel('搜索成员',{exact:true}).fill('q36-no-match-'+stamp);await expect(page.locator('.member-table table')).toHaveAttribute('aria-busy','false');await expect(page.locator('.member-table')).toContainText('暂无成员');await shot(page,'members-empty-narrow');
});

test('Q36 actual page records open close stable trigger and reopen without a root blocker',async({browser})=>{
 const context=await browser.newContext({ignoreHTTPSErrors:true,viewport:{width:1440,height:1000},recordVideo:{dir:evidence,size:{width:1440,height:1000}}});
 await login(context);const page=await context.newPage();await page.goto('/app/admin');await expect(page.locator('.member-table table')).toHaveAttribute('aria-busy','false');await page.evaluate(()=>document.fonts.ready);
 const trigger=page.getByRole('button',{name:'自定义筛选',exact:true}),panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await page.waitForTimeout(1000);
 await trigger.click();await expect(panel).toBeVisible();await shot(page,'motion-open',true);await page.waitForTimeout(1000);
 await page.keyboard.press('Escape');await expect(panel).toBeHidden();await page.waitForFunction(()=>!document.documentElement.classList.contains('q36-filter-transition-active'));await page.waitForTimeout(1100);await shot(page,'motion-closed-stable');await expect(trigger.locator('svg')).toBeVisible();await expect(trigger).toContainText('自定义筛选');
 await trigger.click();await expect(panel).toBeVisible();await shot(page,'motion-reopened',true);await page.waitForTimeout(1000);await page.keyboard.press('Escape');await expect(panel).toBeHidden();await page.waitForTimeout(1100);await shot(page,'motion-final-stable');
 const video=page.video();await context.close();if(!video)throw Error('actual page video unavailable');await rename(await video.path(),resolve(evidence,test.info().project.name+'-actual-filter-open-close-reopen.webm'));
});

test('Q36 actual custom filtering clears current-page selection without resurrecting hidden members',async({page})=>{
 await enter(page);await page.setViewportSize({width:1440,height:1000});
 const hidden=page.getByLabel('选择成员：'+fixture.user.account,{exact:true});await hidden.check();await shot(page,'members-selection-before-filter');
 const entry=await newFilterEditor(page),panel=entry.editor;
 await panel.getByRole('button',{name:'或条件',exact:true}).click();await panel.getByLabel('条件 1.1 值',{exact:true}).fill(fixture.admin.account);await saveApplyFilter(page,entry);
 await expect(hidden).toHaveCount(0);await page.getByLabel('选择当前页成员',{exact:true}).check();await expect(page.getByLabel('选择成员：'+fixture.admin.account,{exact:true})).toBeChecked();await shot(page,'members-selection-filtered-current-page');
 await cancelFilter(page);
 await expect(hidden).not.toBeChecked();await expect(page.getByLabel('选择成员：'+fixture.admin.account,{exact:true})).not.toBeChecked();await expect(page.getByLabel('选择当前页成员',{exact:true})).not.toBeChecked();await shot(page,'members-selection-reset-cleared');
});
