import {fixtureRequestURL,q36FixtureEnvelope} from './personnel-query-fixtures';
import { test, expect, type Page } from '@playwright/test';
import { readFileSync } from 'node:fs';

// Independent oracle: personnel PRD R3 UI AC01–07, Q25 DTOs, Figma Home/Admin.
const user = {id:'00000000-0000-4000-8000-000000000001',account:'synthetic-admin'};
const identity = {id:'00000000-0000-4000-8000-000000000002',name:'企业管理员',description:'共享身份',version:1,templateIds:['00000000-0000-4000-8000-000000000003'],permissionCodes:[],affectedMembers:2,affectedIdentities:0};
const template = {id:'00000000-0000-4000-8000-000000000003',name:'企业管理',description:'共享模板',version:1,permissionCodes:['personnel.manage'],affectedMembers:2,affectedIdentities:1};
const department = {id:'00000000-0000-4000-8000-000000000004',name:'企业',parentId:null,isRoot:true,version:1,memberCount:0,childrenCount:0};
const catalog=[{code:'personnel.manage',name:'人员管理',category:'system',appId:null,enabled:true}];
const member={...user,status:'active',bootstrapAdmin:true,version:0,departmentIds:[],identityIds:[],departments:[],identities:[],permissions:catalog};
function list(items: unknown[]) {return {items,total:items.length,page:1,pageSize:20};}
async function fixture(page:Page,manage=true) {
 await page.route('**/api/v1/**',async route=>{
  const path=fixtureRequestURL(route.request()).pathname.replace('/api/v1/','');
  const data=path==='sessions/current'?user:path==='sessions'?user:path==='me/access'?{user,bootstrapAdmin:manage,personnelManage:manage,identities:[],permissions:manage?catalog:[],applications:[]}:
   path==='applications'?{items:[]}:path==='personnel/departments'?{items:[department]}:path==='personnel/permissions'?{items:catalog}:
   path==='personnel/identities'?list([identity]):path==='personnel/templates'?list([template]):path==='personnel/members'?list([member]):path==='personnel/members/'+user.id?member:path==='personnel/events'?list([]):{};
  await route.fulfill({status:route.request().method()==='DELETE'?204:200,contentType:'application/json',body:route.request().method()==='DELETE'?'':JSON.stringify(q36FixtureEnvelope(data,route.request()))});
 });
}
async function admin(page:Page){await fixture(page);await page.goto('/app/admin');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();}

// V030-021 Root oracle: user-approved monochrome full-bleed surface supersedes
// Q31/Q33 L-material and fixed 176/56 geometry; business assertions stay intact.
async function assertMonochromeLayout(page:Page) {
 const viewport=page.viewportSize()!;
 await expect(page.getByTestId('workspace-shell')).toHaveCount(1);
 const surface=page.getByTestId('workspace-surface'),rail=page.getByTestId('workspace-rail');
 await expect(surface).toHaveCSS('background-color','rgb(255, 255, 255)');
 const box=(await surface.boundingBox())!,railBox=(await rail.boundingBox())!;
 expect(box.y).toBeCloseTo(0,0);expect(box.x+box.width).toBeCloseTo(viewport.width,0);expect(box.y+box.height).toBeCloseTo(viewport.height,0);
 expect(box.x).toBeCloseTo(railBox.width,0);expect(railBox.x).toBe(0);
 if(viewport.width>=1000){expect(railBox.width).toBe(78);await expect(page.getByRole('navigation',{name:'应用导航'})).toHaveCSS('width','190px');await expect(page.locator('.mono-top')).toHaveCSS('height','64px');}
 const menu=page.getByRole('button',{name:'人员管理',exact:true});await menu.scrollIntoViewIfNeeded();await expect(menu).toBeVisible();await expect(menu).toBeInViewport();
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
}

async function chooseOption(page:Page,label:string,option:string){await page.getByRole('combobox',{name:label,exact:true}).click();await page.getByRole('option',{name:option,exact:true}).click();}

// Q35 supersedes identity/max360/native selection under synced R3 5.8; fixed
// Arca c0319d8 official default. Network fixtures exercise the real page controls.
for(const [tab,root] of [['成员与部门','.member-table'],['操作记录','.activity-table']] as const)test('Q34 Arca table geometry and readable rows on '+tab,async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await fixture(page);
 await page.route('**/api/v1/personnel/events/search',r=>r.fulfill({json:q36FixtureEnvelope(list([{id:user.id,occurredAt:'2026-10-01T02:00:00Z',actorAccount:user.account,action:'IDENTITY_UPDATED',objectType:'identity',objectId:identity.id,summary:{before:{name:'旧身份'},after:{name:identity.name}},outcome:'success'}]),r.request())}));
 await page.goto('/app/admin');if(tab!=='成员与部门')await page.getByRole('tab',{name:tab,exact:true}).click();
 const area=page.locator(root),table=area.getByRole('table');await expect(table).toHaveCount(1);
 await expect.soft(area).toHaveCSS('border-radius','14px');await expect.soft(area).toHaveCSS('border-top-width','1px');
 const header=table.locator('th').nth(tab==='成员与部门'?1:0),cell=table.locator('tbody tr[data-row-id] td').last();
 // Official c0319d8 measured header is 40.5px due to collapsed grid border;
 // nominal rowHeight stays 40. See q35 upstream official-default-computed.txt.
 expect.soft((await header.boundingBox())!.height).toBeCloseTo(40.5,1);
 expect.soft((await table.locator('tbody tr').first().boundingBox())!.height).toBeCloseTo(40,0);
 await expect.soft(header).toHaveCSS('font-size','14px');await expect.soft(header).toHaveCSS('line-height','20px');await expect.soft(header).toHaveCSS('font-weight','500');
 await expect.soft(header).toHaveCSS('text-align','center');
 await expect.soft(cell).toHaveCSS('line-height','20px');await expect.soft(table).toHaveCSS('table-layout','fixed');
 await expect(area.locator('.table-scroll')).toHaveCSS('max-height','none');
 await expect(table.locator('th[aria-sort]')).toHaveCount(tab==='成员与部门'?0:1);
 await page.screenshot({path:test.info().outputPath('q35-'+(tab==='成员与部门'?'members':'activity')+'.png'),fullPage:true});
});

test('Q34 page selection exposes mixed state without writing and shows selected rows',async({page})=>{
 await fixture(page);const another={...member,id:'00000000-0000-4000-8000-000000000099',account:'synthetic-second'};
 await page.route('**/api/v1/personnel/members/search',r=>r.fulfill({json:q36FixtureEnvelope(list([member,another]),r.request())}));await page.goto('/app/admin');
 let writes=0;page.on('request',r=>{if(r.url().includes('/api/v1/')&&!['GET','HEAD'].includes(r.method())&&!r.url().endsWith('/search'))writes++;});
 await page.getByLabel('选择成员：'+user.account,{exact:true}).check();
 const all=page.getByLabel('选择当前页成员',{exact:true});await expect.soft(all).toHaveAttribute('aria-checked','mixed');
 const row=page.getByRole('row').filter({hasText:user.account});await expect.soft(row).toHaveAttribute('aria-selected','true');
 await all.check();await expect(page.getByLabel('选择成员：synthetic-second',{exact:true})).toBeChecked();await all.uncheck();await expect(page.getByLabel('选择成员：'+user.account,{exact:true})).not.toBeChecked();expect(writes).toBe(0);
});

test('Q35 identity cards preserve edit entry, shared fields and unsaved switch protection',async({page})=>{
 await fixture(page);const another={...identity,id:'00000000-0000-4000-8000-000000000098',name:'财务身份',description:'独立示例说明'};
 await page.route('**/api/v1/personnel/identities?**',r=>r.fulfill({json:q36FixtureEnvelope(list([identity,another]),r.request())}));await page.goto('/app/admin');await page.getByRole('tab',{name:'身份',exact:true}).click();
 const table=page.locator('.definition-list');await expect(table.getByRole('table')).toHaveCount(0);await expect(table.locator('.definition-item')).toHaveCount(2);
 await table.getByRole('button',{name:identity.name,exact:true}).focus();await page.keyboard.press('Enter');await page.getByLabel('说明',{exact:true}).fill('未保存说明');
 await table.getByRole('button',{name:another.name,exact:true}).click();await expect(page.getByRole('dialog',{name:'有未保存的修改',exact:true})).toBeVisible();await page.getByRole('button',{name:'继续编辑',exact:true}).click();
 await expect(page.getByLabel('说明',{exact:true})).toHaveValue('未保存说明');await expect(table.getByRole('button',{name:identity.name,exact:true})).toHaveClass(/selected/);
 await expect(table.getByRole('button',{name:another.name,exact:true})).toContainText(another.description);
 await page.screenshot({path:test.info().outputPath('q34-identity-editor.png'),fullPage:true});
});

for(const width of [1920,320])test('Q34 dense table keeps sticky header and both scroll axes at '+width,async({page})=>{
 await page.setViewportSize({width,height:844});await fixture(page);
 const rows=Array.from({length:20},(_,i)=>({...member,id:'row-'+i,account:'synthetic-row-'+i}));
 await page.route('**/api/v1/personnel/members/search',r=>r.fulfill({json:q36FixtureEnvelope(list(rows),r.request())}));await page.goto('/app/admin');
 const scroll=page.locator('.member-table .table-scroll');await scroll.scrollIntoViewIfNeeded();expect(await scroll.evaluate(n=>n.scrollHeight>n.clientHeight)).toBe(true);
 const th=scroll.locator('th').first(),start=(await th.boundingBox())!.y;await scroll.evaluate(n=>{n.scrollTop=200;});expect((await th.boundingBox())!.y).toBeCloseTo(start,0);await expect(th).toHaveCSS('position','sticky');
 if(width===320){
  await scroll.evaluate(n=>{n.scrollTop=0;n.scrollLeft=n.scrollWidth;});expect(await scroll.evaluate(n=>n.scrollLeft)).toBeGreaterThan(0);
  for(const name of ['配置身份','调整分组']){const action=page.getByRole('button',{name,exact:true}).first();await action.scrollIntoViewIfNeeded();await expect(action).toBeInViewport();expect(await scroll.evaluate(n=>n.scrollLeft)).toBeGreaterThan(0);}
 }
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.screenshot({path:test.info().outputPath('q34-scroll-'+width+'.png'),fullPage:true});
});

test('Q35 identity empty list and reduced motion preserve accessible states',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await fixture(page);await page.route('**/api/v1/personnel/identities?**',r=>r.fulfill({json:q36FixtureEnvelope(list([]),r.request())}));await page.goto('/app/admin');await page.getByRole('tab',{name:'身份',exact:true}).click();
 const table=page.locator('.definition-list');await expect(table.getByRole('table')).toHaveCount(0);await expect(table).toContainText('暂无身份');
 await page.getByRole('tab',{name:'成员与部门',exact:true}).click();await expect(page.locator('.member-table tbody tr').first()).toHaveCSS('transition-duration','0s');
});

test('Q34 keyboard focus ring stays visible inside truncated table cells',async({page})=>{
 await admin(page);
 await page.keyboard.press('Tab');const name=page.locator('.member-table').getByRole('button',{name:'配置身份',exact:true});await name.focus();await expect(name).toBeFocused();
 const ring=await name.evaluate(n=>{const b=n.getBoundingClientRect(),clip=n.parentElement!.getBoundingClientRect(),style=getComputedStyle(n),outset=parseFloat(style.outlineOffset)+parseFloat(style.outlineWidth);return {style:style.outlineStyle,width:parseFloat(style.outlineWidth),left:b.left-outset,right:b.right+outset,top:b.top-outset,bottom:b.bottom+outset,clip:{left:clip.left,right:clip.right,top:clip.top,bottom:clip.bottom}};});
 expect(ring.style).not.toBe('none');expect(ring.width).toBeGreaterThan(0);
 expect.soft(ring.left).toBeGreaterThanOrEqual(ring.clip.left);expect.soft(ring.right).toBeLessThanOrEqual(ring.clip.right);expect.soft(ring.top).toBeGreaterThanOrEqual(ring.clip.top);expect.soft(ring.bottom).toBeLessThanOrEqual(ring.clip.bottom);
 await page.screenshot({path:test.info().outputPath('q34-focus.png'),fullPage:true});
});

test('Q34 reverse keyboard navigation keeps row actions below the sticky header',async({page})=>{
 await page.setViewportSize({width:1920,height:844});await fixture(page);const rows=Array.from({length:20},(_,i)=>({...member,id:'keyboard-'+i,account:'synthetic-keyboard-'+i}));
 await page.route('**/api/v1/personnel/members/search',r=>r.fulfill({json:q36FixtureEnvelope(list(rows),r.request())}));await page.goto('/app/admin');
 const actions=page.getByRole('button',{name:'配置身份',exact:true});await actions.last().focus();await page.keyboard.press('Tab');
 const bottom=(await page.locator('.member-table th').first().boundingBox())!.y+40;
 for(let i=0;i<15;i++){await page.keyboard.press('Shift+Tab');const focused=page.locator('.personnel-data-table :focus');await expect(focused).toHaveCount(1);expect((await focused.boundingBox())!.y).toBeGreaterThanOrEqual(bottom);}
});

// Q33 oracle: user removal + synced R3 5.3–5.5 and Figma Admin 108:151.
// Superseded by V030-021: shared full-bleed desktop surface, reachable compact navigation.
for(const width of [2504,1920,900,390,320])test('Root Mono personnel keeps reachable navigation and full-bleed surface at '+width,async({page})=>{
 await page.setViewportSize({width,height:844});await admin(page);await assertMonochromeLayout(page);
 await expect(page.getByRole('button',{name:/收起侧栏|展开侧栏/})).toHaveCount(0);
});

test('Q33 crossing responsive breakpoints preserves navigation, dirty input and explicit leave protection',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('固定侧栏保留草稿');
 let writes=0;page.on('request',r=>{if(r.url().includes('/api/v1/')&&!['GET','HEAD'].includes(r.method())&&!r.url().endsWith('/search'))writes++;});
 for(const width of [390,320,900,1920]){
  await page.setViewportSize({width,height:844});await assertMonochromeLayout(page);
  await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('固定侧栏保留草稿');await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');
  await expect(page.getByRole('button',{name:'保存',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'退出',exact:true})).toBeVisible();
 }
 expect(writes).toBe(0);await page.getByRole('button',{name:'退出',exact:true}).click();await expect(page.getByRole('dialog',{name:'有未保存的修改',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('固定侧栏保留草稿');expect(writes).toBe(0);
});

for(const width of [390,320])test('Q33 fixed sidebar retains narrow keyboard tabs and real content scrolling at '+width,async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await page.setViewportSize({width,height:600});await admin(page);
 await assertMonochromeLayout(page);
 const tabs=page.getByRole('tablist'),first=page.getByRole('tab',{name:'成员与部门',exact:true});await first.focus();await first.press('End');await page.keyboard.press('Enter');
 const last=page.getByRole('tab',{name:'操作记录',exact:true});await expect(last).toHaveAttribute('aria-selected','true');
 const bar=(await tabs.boundingBox())!,button=(await last.boundingBox())!;expect(button.x).toBeGreaterThanOrEqual(bar.x);expect(button.x+button.width).toBeLessThanOrEqual(bar.x+bar.width+1);
 expect(await tabs.evaluate(n=>n.scrollHeight===n.clientHeight)).toBe(true);
 await first.focus();await first.press('Enter');const search=page.getByLabel('搜索成员',{exact:true});await search.scrollIntoViewIfNeeded();await search.fill('固定布局');await expect(search).toHaveValue('固定布局');
 await expect(search).toBeInViewport();
 const table=page.locator('.table-scroll');await table.evaluate(n=>{n.scrollLeft=n.scrollWidth;});expect(await table.evaluate(n=>n.scrollLeft)).toBeGreaterThan(0);
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

// Q32 oracle: user-selected Arca Select Menu / default Pill, R3 section 5.6.
// These tests exercise the real page; only the network boundary uses synthetic data.
test('Q32 identity select has the separated rounded menu and preserves server filtering',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);
 const combo=page.getByRole('combobox',{name:'筛选身份',exact:true});
 await expect.soft(combo).toHaveCSS('height','38px');await expect.soft(combo).toHaveCSS('border-radius','12px');
 expect.soft(await combo.evaluate(n=>n.tagName)).toBe('BUTTON');await combo.click();
 const menu=page.getByRole('listbox',{name:'筛选身份',exact:true});await expect(menu).toBeVisible();
 await expect(menu).toHaveCSS('border-radius','12px');await expect(page.getByRole('option',{name:'全部身份',exact:true})).toHaveCSS('height','32px');
 await expect.poll(async()=>{const a=(await combo.boundingBox())!,b=(await menu.boundingBox())!;return Math.round(b.y-a.y-a.height);}).toBe(8);
 const queries:string[]=[];await page.route('**/api/v1/personnel/members/search',r=>{queries.push(fixtureRequestURL(r.request()).search);return r.fulfill({status:200,json:q36FixtureEnvelope(list([member]),r.request())});});
 await page.getByRole('option',{name:'企业管理员',exact:true}).click();await expect(combo).toContainText('企业管理员');await expect(combo).toBeFocused();
 await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('identityId')===identity.id)).toBe(true);
 await combo.press('ArrowDown');await combo.press('Home');await combo.press('Enter');await expect(combo).toContainText('全部身份');
 await expect.poll(()=>queries.some(q=>!new URLSearchParams(q).has('identityId'))).toBe(true);
});

test('Q32 grouped selectors keep modal focus and Escape first dismisses the menu',async({page})=>{
 await admin(page);await page.getByRole('button',{name:'调整分组',exact:true}).click();
 const operation=page.getByRole('combobox',{name:'分组操作',exact:true});expect(await operation.evaluate(n=>n.tagName)).toBe('BUTTON');
 await operation.press('ArrowDown');await operation.press('End');await operation.press('Enter');await expect(operation).toContainText('移动');
 for(const label of ['来源部门','目标部门']){const combo=page.getByRole('combobox',{name:label,exact:true});expect(await combo.evaluate(n=>n.tagName)).toBe('BUTTON');await expect(combo).toHaveCSS('height','38px');}
 const target=page.getByRole('combobox',{name:'目标部门',exact:true});await target.click();await expect(page.getByRole('listbox',{name:'目标部门',exact:true})).toBeVisible();
 await target.press('Escape');await expect(target).toHaveAttribute('aria-expanded','false');await expect(page.getByRole('dialog',{name:'调整成员分组',exact:true})).toBeVisible();await expect(target).toBeFocused();
 await target.press('Escape');await expect(page.getByRole('dialog',{name:'有未保存的修改',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(operation).toContainText('移动');
});

test('Q32 parent department chooser uses options without closing the native modal',async({page})=>{
 await admin(page);await page.getByRole('button',{name:'新建部门',exact:true}).click();
 const combo=page.getByRole('combobox',{name:'父部门',exact:true});expect(await combo.evaluate(n=>n.tagName)).toBe('BUTTON');
 await combo.click();await page.getByRole('option',{name:'企业',exact:true}).click();await expect(combo).toBeFocused();await expect(page.getByRole('dialog',{name:'新建部门',exact:true})).toBeVisible();
 let sent:unknown;await page.route('**/api/v1/personnel/departments',r=>{if(r.request().method()!=='POST')return r.fallback();sent=r.request().postDataJSON();return r.fulfill({status:201,json:q36FixtureEnvelope(department,r.request())});});
 await page.getByLabel('部门名称',{exact:true}).fill('设计组');await page.getByRole('button',{name:'确认创建',exact:true}).click();await expect.poll(()=>sent).toEqual({name:'设计组',parentId:department.id,queryVersion:'synthetic-q36-context'});
});

test('Q32 long action menu stays in the viewport and keyboard chooses the final real action',async({page})=>{
 await page.setViewportSize({width:390,height:600});await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();
 const combo=page.getByRole('combobox',{name:'操作类型',exact:true});expect(await combo.evaluate(n=>n.tagName)).toBe('BUTTON');await combo.press('ArrowDown');
 const menu=page.getByRole('listbox',{name:'操作类型',exact:true});await expect(menu).toBeVisible();
 await expect.poll(async()=>{const b=(await menu.boundingBox())!;return b.y>=7&&b.y+b.height<=593;}).toBe(true);
 expect(await menu.evaluate(n=>n.scrollHeight>n.clientHeight)).toBe(true);
 const queries:string[]=[];await page.route('**/api/v1/personnel/events/search',r=>{queries.push(fixtureRequestURL(r.request()).search);return r.fulfill({status:200,json:q36FixtureEnvelope(list([]),r.request())});});
 await combo.press('End');await combo.press('Enter');await expect(combo).toContainText('邀请成员');await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('action')==='INVITATION_CREATED')).toBe(true);
 await combo.click();await page.getByRole('heading',{name:'人员管理',exact:true}).click();await expect(combo).toHaveAttribute('aria-expanded','false');
});

test('Q32 pill indicator slides between tabs only after unsaved changes are resolved',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);
 const indicator=page.locator('.personnel-tab-indicator');await expect(indicator).toHaveCount(1);await expect(indicator).toHaveCSS('background-color','rgb(239, 239, 239)');await expect(indicator).toHaveCSS('height','32px');
 await expect(page.getByRole('tablist')).toHaveCSS('height','48px');for(const tab of await page.getByRole('tab').all())await expect(tab).toHaveCSS('height','32px');
 const initial=(await indicator.boundingBox())!.x;
 const positions=await page.evaluate(async()=>{const el=document.querySelector('.personnel-tab-indicator')!;const samples:number[]=[];const done=new Promise<number[]>(resolve=>{const start=performance.now();function sample(){samples.push(el.getBoundingClientRect().x);if(performance.now()-start<900)requestAnimationFrame(sample);else resolve(samples);}requestAnimationFrame(sample);});(document.querySelectorAll('[role=tab]')[1] as HTMLElement).click();return done;});
 await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');const end=(await indicator.boundingBox())!.x;expect(end).toBeGreaterThan(initial);expect(positions.some(x=>x>initial+1&&x<end-1)).toBe(true);
 await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('说明',{exact:true}).fill('待保存修改');const before=(await indicator.boundingBox())!.x;
 await page.getByRole('tab',{name:'权限模板',exact:true}).click();await expect(page.getByRole('dialog',{name:'有未保存的修改',exact:true})).toBeVisible();await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');expect((await indicator.boundingBox())!.x).toBeCloseTo(before,0);
 await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('说明',{exact:true})).toHaveValue('待保存修改');
});

test('Q32 reduced motion removes displacement and option stagger and keeps compact tabs reachable',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await page.setViewportSize({width:390,height:844});await admin(page);
 const first=page.getByRole('tab',{name:'成员与部门',exact:true});await first.focus();await first.press('End');await page.keyboard.press('Enter');await expect(page.getByRole('tab',{name:'操作记录',exact:true})).toHaveAttribute('aria-selected','true');
 const last=(await page.getByRole('tab',{name:'操作记录',exact:true}).boundingBox())!,bar=(await page.getByRole('tablist').boundingBox())!;expect(last.x).toBeGreaterThanOrEqual(bar.x);expect(last.x+last.width).toBeLessThanOrEqual(bar.x+bar.width+1);
 expect(await page.getByRole('tablist').evaluate(n=>n.scrollHeight===n.clientHeight)).toBe(true);
 const combo=page.getByRole('combobox',{name:'操作类型',exact:true});await combo.click();const menu=page.getByRole('listbox',{name:'操作类型',exact:true});await expect(menu).toBeVisible();await expect(menu).toHaveCSS('transform','none');
 await expect(page.getByRole('option').first()).toHaveCSS('filter','none');await expect(page.getByRole('option').first()).toHaveCSS('animation-name','none');
 await combo.press('Tab');await expect(combo).toHaveAttribute('aria-expanded','false');
});

// Q31: current Figma75:2 /106:112 /113:425 fixes top56, removes collapse,
test('Q32 large directories expose the final option promptly after End',async({page})=>{
 await fixture(page);const choices=Array.from({length:100},(_,index)=>({...identity,id:'00000000-0000-4000-8000-'+String(index+200).padStart(12,'0'),name:'身份-'+String(index).padStart(3,'0')}));
 await page.route('**/api/v1/personnel/identities?**',r=>r.fulfill({status:200,json:q36FixtureEnvelope({...list(choices),pageSize:100},r.request())}));await page.goto('/app/admin');
 const combo=page.getByRole('combobox',{name:'筛选身份',exact:true});await combo.press('ArrowDown');await combo.press('End');const last=page.getByRole('option',{name:'身份-099',exact:true});await expect(last).toHaveCSS('opacity','1',{timeout:1000});await combo.press('Enter');await expect(combo).toContainText('身份-099');
});

test('Q32 quick reversal preserves the current menu height rather than jumping to endpoints',async({page})=>{
 await admin(page);
 const button=page.getByRole('combobox',{name:'筛选身份',exact:true});await button.click();
 // Two independent fixture choices: 2*32px + 8px padding + 2px border = 74px.
 await page.waitForFunction(()=>{const h=document.querySelector('[role=listbox]')!.getBoundingClientRect().height;return h>14&&h<64;});
 async function clickedFrame(){
  const sample=page.evaluate(()=>new Promise<{before:number;after:number;trusted:boolean}>(resolve=>{
   document.querySelector('[role=combobox]')!.addEventListener('click',event=>{
    const menu=document.querySelector('[role=listbox]')!;const before=menu.getBoundingClientRect().height;
    requestAnimationFrame(()=>resolve({before,after:menu.getBoundingClientRect().height,trusted:event.isTrusted}));
   },{capture:true,once:true});
  }));
  await button.click();return sample;
 }
 const closing=await clickedFrame(),reopening=await clickedFrame();
 expect(closing.trusted&&reopening.trusted).toBe(true);expect(closing.before).toBeGreaterThan(10);expect(closing.before).toBeLessThan(74);
 expect(Math.abs(closing.after-closing.before)).toBeLessThan(16);expect(Math.abs(reopening.after-reopening.before)).toBeLessThan(16);await expect(button).toHaveAttribute('aria-expanded','true');
});

test('Q32 searching below the tabs preserves the narrow-screen content scroll position',async({page})=>{
 await page.setViewportSize({width:390,height:600});await admin(page);const input=page.getByLabel('搜索成员',{exact:true});await input.scrollIntoViewIfNeeded();
 const content=page.locator('.personnel-content');await input.focus();const before=await content.evaluate(n=>n.scrollTop);expect(before).toBeGreaterThan(80);
 await input.fill('连续输入');await expect(input).toHaveValue('连续输入');await expect.poll(()=>content.evaluate(n=>n.scrollTop)).toBe(before);
});

test('Q32 upward expansion and closing stay attached to the trigger edge on every frame',async({page})=>{
 await page.setViewportSize({width:390,height:320});await fixture(page);const choices=Array.from({length:20},(_,index)=>({...department,id:'00000000-0000-4000-8000-'+String(index+400).padStart(12,'0'),name:'部门-'+index}));
 await page.route('**/api/v1/personnel/departments',r=>r.fulfill({status:200,json:q36FixtureEnvelope({items:choices},r.request())}));await page.goto('/app/admin');await page.getByRole('button',{name:'新建部门',exact:true}).click();
 const button=page.getByRole('combobox',{name:'父部门',exact:true});
 const sample=()=>page.evaluate(async()=>{
  const button=document.querySelector<HTMLButtonElement>('[aria-label="父部门"]')!;
  const points:{gap:number;height:number;side:string|undefined}[]=[];const start=performance.now();while(performance.now()-start<300){await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()));const menu=document.querySelector<HTMLElement>('.personnel-select-menu[role=listbox]')!;const b=menu.getBoundingClientRect();points.push({gap:button.getBoundingClientRect().top-b.bottom,height:b.height,side:menu.dataset.side});}return points;
 });
 await button.click();const opening=await sample();await button.click();const closing=await sample();const samples={opening,closing};
 expect(samples.opening.every(point=>point.side==='above')).toBe(true);
 for(const point of [...samples.opening,...samples.closing]){expect(point.gap).toBeGreaterThanOrEqual(-1);expect(point.gap).toBeLessThanOrEqual(12);}
 expect(samples.opening.some(point=>point.height>10&&point.height<samples.opening.at(-1)!.height-10),JSON.stringify(samples)).toBe(true);
});

test('Q32 upward menus animate the corners adjacent to their trigger',async({page})=>{
 await page.setViewportSize({width:390,height:320});await fixture(page);const choices=Array.from({length:20},(_,index)=>({...department,id:'00000000-0000-4000-8000-'+String(index+400).padStart(12,'0'),name:'部门-'+index}));
 await page.route('**/api/v1/personnel/departments',r=>r.fulfill({status:200,json:q36FixtureEnvelope({items:choices},r.request())}));await page.goto('/app/admin');await page.getByRole('button',{name:'新建部门',exact:true}).click();
 const corners=await page.evaluate(async()=>{document.querySelector<HTMLButtonElement>('[aria-label="父部门"]')!.click();await new Promise<void>(resolve=>requestAnimationFrame(()=>resolve()));const menu=document.querySelector<HTMLElement>('.personnel-select-menu[role=listbox]')!;const s=getComputedStyle(menu);return {side:menu.dataset.side,top:s.borderTopLeftRadius,bottom:s.borderBottomLeftRadius};});
 expect(corners.side).toBe('above');
 expect(parseFloat(corners.top)).toBe(12);expect(parseFloat(corners.bottom)).toBeLessThan(12);
});

// and moves both sidebar menus to y116; supersedes Q29/Q30 old top/menu geometry. Network fixtures isolate the real rendered components.
test('Root Mono Home keeps account control in the rail and a stable top bar',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await fixture(page);await page.goto('/app');await assertMonochromeLayout(page);
 const account=page.getByRole('button',{name:'账号',exact:true});await expect(account).toHaveCount(1);await expect(account).toBeInViewport();
 await account.click();await expect(page.locator('.account-menu')).toBeVisible();await expect(page.locator('.account-menu')).toBeInViewport();
 await expect(page.locator('.mono-top')).toHaveCSS('height','64px');
});

test('Root Mono admin retains actions within shared full-bleed chrome across resize',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await admin(page);
 for(const width of [1920,1440,390]){await page.setViewportSize({width,height:1080});await assertMonochromeLayout(page);await expect(page.getByRole('button',{name:'退出',exact:true})).toBeVisible();await expect(page.getByRole('heading',{name:'人员管理',exact:true})).toBeVisible();}
});

// Q33 supersedes the historical folding oracle. The visible L material still
// fills the viewport; navigation geometry remains fixed during resize/hover.
test('Q31 tabs have no vertical overflow and compact layouts retain keyboard access to the last tab',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);
 const tabs=page.getByRole('tablist');await expect.soft(tabs).toHaveCSS('height','48px');await expect.soft(tabs).toHaveCSS('overflow-y','hidden');
 const desktop=await tabs.evaluate(n=>({height:n.clientHeight,scrollHeight:n.scrollHeight,buttons:[...n.querySelectorAll('button')].map(b=>b.getBoundingClientRect().height)}));
 // Q32 replaces the old 47px underline buttons with the confirmed 32px Pill.
 expect.soft(desktop.scrollHeight).toBe(desktop.height);expect.soft(desktop.buttons).toEqual([32,32,32,32]);
 expect.soft(await tabs.evaluate(n=>{n.scrollTop=10;return n.scrollTop;})).toBe(0);
 await page.setViewportSize({width:390,height:844});await page.goto('/app/admin');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();
 const last=page.getByRole('tab',{name:'操作记录',exact:true});await last.focus();await last.press('Enter');await expect(last).toHaveAttribute('aria-selected','true');
 const bounds=(await tabs.boundingBox())!,button=(await last.boundingBox())!;expect(button.x).toBeGreaterThanOrEqual(bounds.x);expect(button.x+button.width).toBeLessThanOrEqual(bounds.x+bounds.width+1);
 expect(await tabs.evaluate(n=>n.scrollLeft)).toBeGreaterThan(0);expect(await tabs.evaluate(n=>n.scrollTop)).toBe(0);
});

test('Q31 real member overflow remains scrollable while the tab strip has no vertical scrollbar',async({page})=>{
 await page.setViewportSize({width:1440,height:600});await fixture(page);
 const members=Array.from({length:20},(_,i)=>({...member,id:'00000000-0000-4000-8000-'+String(i+100).padStart(12,'0'),account:'synthetic-row-'+String(i).padStart(2,'0')}));
 await page.route('**/api/v1/personnel/members/search',r=>r.fulfill({status:200,json:q36FixtureEnvelope(list(members),r.request())}));await page.goto('/app/admin');
 const last=page.getByRole('row').filter({hasText:'synthetic-row-19'});
 const table=page.locator('.table-scroll');expect(await table.evaluate(n=>n.scrollHeight>n.clientHeight)).toBe(true);
 await table.evaluate(n=>{n.scrollTop=n.scrollHeight;});expect(await table.evaluate(n=>n.scrollTop)).toBeGreaterThan(0);await expect(last).toHaveCount(1);
 const box=(await table.boundingBox())!,row=(await last.boundingBox())!;expect(row.y).toBeGreaterThanOrEqual(box.y);expect(row.y+row.height).toBeLessThanOrEqual(box.y+box.height+1);
 await page.setViewportSize({width:390,height:844});await table.evaluate(n=>{n.scrollLeft=n.scrollWidth;});expect(await table.evaluate(n=>n.scrollLeft)).toBeGreaterThan(0);
});


for(const viewport of [{width:2504,height:1355},{width:1440,height:900},{width:390,height:844}])test('Root Mono white surface fills viewport '+viewport.width+'x'+viewport.height+' after resize',async({page})=>{
 await page.setViewportSize(viewport);await admin(page);await page.emulateMedia({reducedMotion:'reduce'});await assertMonochromeLayout(page);
 await page.setViewportSize({width:viewport.width+173,height:viewport.height+129});await assertMonochromeLayout(page);
});

test('Root Mono hover preserves navigation geometry without moving the label',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);
 const menu=page.getByRole('button',{name:'人员管理',exact:true});const before=await menu.boundingBox();await menu.hover();
 for(let i=0;i<8;i++){await page.evaluate(()=>new Promise<void>(r=>requestAnimationFrame(()=>r())));expect(await menu.boundingBox()).toEqual(before);}
 await expect(menu).toBeVisible();await assertMonochromeLayout(page);
});
test('Q33 reduced motion and repeated resize keep navigation geometry and unsaved edit',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('保留输入');
 for(const reducedMotion of ['reduce','no-preference'] as const){
  await page.emulateMedia({reducedMotion});
  for(const width of [390,1000,320,1920]){await page.setViewportSize({width,height:844});await assertMonochromeLayout(page);await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('保留输入');}
 }
});
test('R3 login enters Home with settings and account navigation; admin exit retains Session',async({page})=>{
 await fixture(page);let logouts=0;
 await page.route('**/api/v1/sessions/current',async route=>{if(route.request().method()==='DELETE') logouts++;await route.fulfill({status:200,json:q36FixtureEnvelope(user,route.request())});});
 await page.goto('/login');await page.getByLabel('账号',{exact:true}).fill(user.account);await page.getByLabel('密码',{exact:true}).fill('Synthetic@123');await page.getByRole('button',{name:'登录',exact:true}).click();
 await expect(page).toHaveURL(/\/app$/);await expect(page.getByRole('button',{name:'设置',exact:true})).toBeVisible();
 await expect(page.locator('.mono-top')).toHaveCSS('height','64px');
 await page.getByRole('button',{name:'设置',exact:true}).click();await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'退出',exact:true}).click();await expect(page).toHaveURL(/\/app$/);expect(logouts).toBe(0);
 await page.getByRole('button',{name:'账号',exact:true}).click();await expect(page.getByText(user.account,{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'退出登录',exact:true}).click();expect(logouts).toBe(1);await expect(page).toHaveURL(/\/login$/);
});
test('R2 AC02 ordinary member Home has own identity/empty app state; direct admin denies',async({page})=>{
 await fixture(page,false);await page.goto('/app');await expect(page.getByText('暂无可用应用',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'设置',exact:true})).toBeHidden();
 await page.goto('/app/admin');await expect(page.getByRole('alert')).toContainText('没有人员管理权限');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeHidden();
});
test('Q33 fixed sidebar retains selected tab and unsaved input across viewports; icons aligned',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('尚未保存');
 for(const width of [1920,1440,390]){
  await page.setViewportSize({width,height:1080});await expect(page.getByRole('button',{name:'保存',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'退出',exact:true})).toBeVisible();
  await assertMonochromeLayout(page);
  await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('尚未保存');await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');
  const rail=(await page.getByTestId('workspace-rail').boundingBox())!;for(const icon of await page.getByTestId('workspace-rail').locator('img').all()){const b=(await icon.boundingBox())!;expect(b.x+b.width/2).toBeCloseTo(rail.x+rail.width/2,0);}
 }
});
test('R3 identity separates direct/template sources and confirms impact before versioned save',async({page})=>{
 await admin(page);const sent:unknown[]=[];
 await page.route('**/api/v1/personnel/identities/'+identity.id,async route=>{sent.push(route.request().postDataJSON());await route.fulfill({status:200,json:q36FixtureEnvelope({...identity,name:'新名称',version:2},route.request())});});
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();
 await expect(page.getByLabel('直接权限：人员管理',{exact:true})).not.toBeChecked();await expect(page.getByLabel('模板：企业管理',{exact:true})).toBeChecked();await expect(page.getByText('来自模板：企业管理',{exact:true})).toBeVisible();
 await page.getByLabel('身份名称',{exact:true}).fill('新名称');await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('2 位成员');expect(sent).toHaveLength(0);
 await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect.poll(()=>sent).toEqual([{name:'新名称',description:'共享身份',version:1,templateIds:[template.id],permissionCodes:[]}]);await expect(page.getByRole('status')).toContainText('已保存');
});
for(const [status,message] of [[409,'配置已变更'],[403,'没有人员管理权限'],[503,'服务暂时不可用']] as const){
 test('R3 save failure '+status+' preserves input and exposes specific outcome',async({page})=>{
  await admin(page);await page.route('**/api/v1/personnel/identities/'+identity.id,route=>route.fulfill({status,json:{code:status===409?'PERSONNEL_CONFLICT':status===403?'COMMON_PERMISSION_DENIED':'COMMON_UNAVAILABLE',message:'safe',data:null,meta:null}}));
  await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('保留修改');await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect(page.getByRole('alert')).toContainText(message);await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('保留修改');
 });
}
test('R3 dirty tab/object/admin leave requires explicit choice and never saves',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('保留');
 await page.getByRole('tab',{name:'权限模板',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('保留');
 await page.getByRole('button',{name:'退出',exact:true}).click();await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(/\/app$/);
});
test('R3 template UI has genuine no-application empty state and referenced delete protection',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'企业管理',exact:true}).click();await expect(page.getByText('尚未接入业务应用',{exact:true})).toBeVisible();await expect(page.getByLabel('中央权限：人员管理',{exact:true})).toBeChecked();await expect(page.getByRole('button',{name:'删除模板',exact:true})).toBeDisabled();await expect(page.getByText('1 个身份正在引用',{exact:true})).toBeVisible();
});
test('R3 members expose multi-identity editor and group operation without changing permissions',async({page})=>{
 await admin(page);const requests:unknown[]=[];
 await page.route('**/api/v1/personnel/members/'+user.id+'/identities',async route=>{requests.push(route.request().postDataJSON());await route.fulfill({status:200,json:q36FixtureEnvelope({...member,version:1,identityIds:[identity.id],identities:[identity]},route.request())});});
 await page.getByRole('button',{name:'配置身份',exact:true}).click();await expect(page.getByRole('dialog')).toContainText(user.account);await page.getByLabel('身份：企业管理员',{exact:true}).check();await page.getByRole('button',{name:'确认分配',exact:true}).click();await expect.poll(()=>requests).toEqual([{identityIds:[identity.id],version:0,queryVersion:'synthetic-q36-context'}]);
});
test('R3 departments create uses selected parent; enterprise root is protected',async({page})=>{
 await admin(page);await page.getByRole('button',{name:'新建部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('研发');let input:unknown;
 await page.route('**/api/v1/personnel/departments',async route=>{if(route.request().method()==='POST'){input=route.request().postDataJSON();await route.fulfill({status:201,json:q36FixtureEnvelope({...department,id:'00000000-0000-4000-8000-000000000005',name:'研发',isRoot:false,parentId:department.id},route.request())});}else await route.fulfill({status:200,json:q36FixtureEnvelope({items:[department]},route.request())});});
 await page.getByRole('button',{name:'确认创建',exact:true}).click();await expect.poll(()=>input).toEqual({name:'研发',parentId:department.id,queryVersion:'synthetic-q36-context'});
});
test('R3 invitation is deliberate one-time result; activity has true empty state',async({page})=>{
 await admin(page);let created=0;await page.route('**/api/v1/invitations',async route=>{created++;await route.fulfill({status:201,json:q36FixtureEnvelope({id:'00000000-0000-4000-8000-000000000006',invitationCode:'synthetic-component-fixture'},route.request())});});
 await page.getByRole('button',{name:'邀请成员',exact:true}).click();expect(created).toBe(0);await page.getByRole('button',{name:'生成邀请码',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).toHaveValue('synthetic-component-fixture');expect(created).toBe(1);await page.getByRole('button',{name:'关闭',exact:true}).click();await page.getByRole('tab',{name:'操作记录',exact:true}).click();await expect(page.getByText('暂无操作记录',{exact:true})).toBeVisible();await expect(page.getByText('周涵',{exact:true})).toBeHidden();
});
test('R3 reduced motion and compact layouts retain keyboard-reachable navigation',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await page.setViewportSize({width:390,height:844});await admin(page);await expect(page.getByRole('navigation',{name:'应用导航'})).toHaveCSS('transition-duration','0s');await page.getByRole('tab',{name:'身份',exact:true}).focus();await page.keyboard.press('Enter');await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('R3 browser back with dirty configuration prompts, cancel preserves inputs',async({page})=>{
 await fixture(page);await page.goto('/app');await page.getByRole('button',{name:'设置',exact:true}).click();
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('浏览器返回保护');
 await page.goBack();await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('浏览器返回保护');
 await page.goBack();await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(/\/app$/);
});
test('V030-012 retained app operation does not hide the current personnel discard consequence',async({page})=>{
 await fixture(page);
 let writes=0;
 await page.route('**/api/v1/applications',route=>{
  if(route.request().method()==='GET')return route.fulfill({json:q36FixtureEnvelope({items:[]},route.request())});
  writes++;return route.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:null}});
 });
 await page.goto('/app/applications');
 await page.getByRole('button',{name:'新建应用',exact:true}).click();
 const create=page.getByRole('dialog',{name:'新建应用',exact:true});
 await create.getByRole('textbox',{name:'应用名称',exact:true}).fill('待核查应用');
 await create.getByRole('button',{name:'创建应用',exact:true}).click();
 await expect(create.getByRole('alert')).toContainText('尚未确认');
 await create.getByRole('button',{name:'关闭',exact:true}).click();
 await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'关闭并保留待核查操作',exact:true}).click();
 await expect(create).toHaveCount(0);
 await page.getByRole('button',{name:'设置',exact:true}).click();
 await page.getByRole('tab',{name:'身份',exact:true}).click();
 await page.getByRole('button',{name:'企业管理员',exact:true}).click();
 await page.getByLabel('身份名称',{exact:true}).fill('未保存人员更改');
 await page.goBack();
 const guard=page.getByRole('dialog',{name:'有未保存的修改',exact:true});
 await expect(guard).toContainText('人员更改将丢失');
 await expect(guard).toContainText('应用创建结果仍未确认');
 await guard.getByRole('button',{name:'继续编辑',exact:true}).click();
 await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('未保存人员更改');
 await page.goBack();
 await guard.getByRole('button',{name:/放弃人员更改.*保留待核查操作/}).click();
 await expect(page).toHaveURL(/\/app\/applications$/);
 await page.getByRole('button',{name:'新建应用',exact:true}).click();
 const restored=page.getByRole('dialog',{name:'新建应用',exact:true});
 await expect(restored.getByRole('textbox',{name:'应用名称',exact:true})).toHaveValue('待核查应用');
 await expect(restored.getByRole('alert')).toContainText('尚未确认');
 expect(writes).toBe(1);
});
test('R3 definition creation has no fake seed; validates and sends explicit config',async({page})=>{
 await admin(page);let created:unknown;await page.route('**/api/v1/personnel/identities?**',route=>route.fulfill({status:200,json:q36FixtureEnvelope(list([]),route.request())}));
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'新建身份',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('普通员工');
 await page.route('**/api/v1/personnel/identities',async route=>{created=route.request().postDataJSON();await route.fulfill({status:201,json:q36FixtureEnvelope({...identity,name:'普通员工',description:'',templateIds:[],permissionCodes:[]},route.request())});});
 await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect.poll(()=>created).toEqual({name:'普通员工',description:'',templateIds:[],permissionCodes:[]});
});
test('R3 member explicit move preserves other groups and identity configuration',async({page})=>{
 await admin(page);const other={...department,id:'00000000-0000-4000-8000-000000000010',parentId:department.id,name:'研发部',isRoot:false};
 await page.route('**/api/v1/personnel/departments',route=>route.fulfill({status:200,json:q36FixtureEnvelope({items:[department,other]},route.request())}));
 await page.route('**/api/v1/personnel/members**',route=>route.fulfill({status:200,json:q36FixtureEnvelope(list([{...member,departmentIds:[department.id],departments:[department]}]),route.request())}));
 await page.reload();await page.getByRole('button',{name:'调整分组',exact:true}).click();await chooseOption(page,'分组操作','移动');await chooseOption(page,'来源部门','企业');await chooseOption(page,'目标部门','研发部');
 const sent:unknown[]=[];await page.route('**/api/v1/personnel/members/'+user.id+'/groups',async route=>{sent.push(route.request().postDataJSON());await route.fulfill({status:200,json:q36FixtureEnvelope({...member,version:1},route.request())});});
 await page.getByRole('button',{name:'确认调整',exact:true}).click();await expect.poll(()=>sent).toEqual([{operation:'move',departmentId:other.id,sourceDepartmentId:department.id,version:0,queryVersion:'synthetic-q36-context'}]);
});
test('R3 member search and pagination send approved filters, no local page-only filtering',async({page})=>{
 await admin(page);const queries:string[]=[];await page.route('**/api/v1/personnel/members/search',async route=>{queries.push(fixtureRequestURL(route.request()).search);await route.fulfill({status:200,json:q36FixtureEnvelope({...list([member]),total:21},route.request())});});
 await page.getByLabel('搜索成员',{exact:true}).fill('目标');await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('search')==='目标')).toBe(true);
 await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
});
test('R3 activity search and action filter are server-side, not cosmetic controls',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/events/search',async route=>{queries.push(fixtureRequestURL(route.request()).search);await route.fulfill({status:200,json:q36FixtureEnvelope(list([]),route.request())});});
 await page.getByLabel('搜索操作记录',{exact:true}).fill('研发');await chooseOption(page,'操作类型','新建部门');await expect.poll(()=>queries.some(q=>{const p=new URLSearchParams(q);return p.get('search')==='研发'&&p.get('action')==='DEPARTMENT_CREATED';})).toBe(true);
});
test('R3 deleting unreferenced template confirms and sends version; 409 keeps selection',async({page})=>{
 await admin(page);await page.route('**/api/v1/personnel/templates?**',route=>route.fulfill({status:200,json:q36FixtureEnvelope(list([{...template,affectedIdentities:0,affectedMembers:0}]),route.request())}));
 await page.reload();await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'企业管理',exact:true}).click();await page.getByRole('button',{name:'删除模板',exact:true}).click();
 let query='';await page.route('**/api/v1/personnel/templates/'+template.id+'?**',async route=>{query=fixtureRequestURL(route.request()).search;await route.fulfill({status:409,json:{code:'PERSONNEL_CONFLICT',message:'conflict',data:null,meta:null}});});
 await page.getByRole('button',{name:'确认删除',exact:true}).click();expect(new URLSearchParams(query).get('version')).toBe('1');await expect(page.getByRole('alert')).toContainText('配置已变更');await expect(page.getByLabel('模板名称',{exact:true})).toHaveValue('企业管理');
});


test('R3 creates a permission template with explicit empty configuration',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'权限模板',exact:true}).click();
 await expect(page.getByRole('button',{name:'新建权限模板',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'新建权限模板',exact:true}).click();await page.getByLabel('模板名称',{exact:true}).fill('空模板');
 let body:unknown;await page.route('**/api/v1/personnel/templates',async route=>{body=route.request().postDataJSON();await route.fulfill({status:201,json:q36FixtureEnvelope({...template,id:'00000000-0000-4000-8000-000000000012',name:'空模板',permissionCodes:[],affectedIdentities:0,affectedMembers:0},route.request())});});
 await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();
 await expect.poll(()=>body).toEqual({name:'空模板',description:'',permissionCodes:[]});
});
test('R3 identity deletion protects references and explicitly confirms an unreferenced object',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();
 await expect(page.getByRole('button',{name:'删除身份',exact:true})).toBeDisabled();
 await page.route('**/api/v1/personnel/identities?**',route=>route.fulfill({status:200,json:q36FixtureEnvelope(list([{...identity,affectedMembers:0}]),route.request())}));await page.reload();await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();
 let version='';await page.route('**/api/v1/personnel/identities/'+identity.id+'?**',async route=>{version=fixtureRequestURL(route.request()).searchParams.get('version')||'';await route.fulfill({status:204});});
 await page.getByRole('button',{name:'删除身份',exact:true}).click();await page.getByRole('button',{name:'确认删除',exact:true}).click();await expect.poll(()=>version).toBe('1');await expect(page.getByText('请选择身份查看配置',{exact:true})).toBeVisible();
});
test('R3 department selected node filters members; rename and delete use actual version; root protected',async({page})=>{
 await fixture(page);const d={...department,id:'00000000-0000-4000-8000-000000000010',name:'研发部',parentId:department.id,isRoot:false,memberCount:0,childrenCount:0};
 await page.route('**/api/v1/personnel/departments',route=>route.fulfill({status:200,json:q36FixtureEnvelope({items:[department,d]},route.request())}));await page.goto('/app/admin');
 await page.getByRole('button',{name:/^企业/}).click();await expect(page.getByRole('button',{name:'删除部门',exact:true})).toBeDisabled();
 const queries:string[]=[];await page.route('**/api/v1/personnel/members/search',async route=>{queries.push(fixtureRequestURL(route.request()).search);await route.fulfill({status:200,json:q36FixtureEnvelope(list([member]),route.request())});});
 await page.getByRole('button',{name:/^研发部/}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('departmentId')===d.id)).toBe(true);
 await page.getByRole('button',{name:'重命名部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('研发新名');
 let renamed:unknown;await page.route('**/api/v1/personnel/departments/'+d.id,async route=>{renamed=route.request().postDataJSON();await route.fulfill({status:200,json:q36FixtureEnvelope({...d,name:'研发新名',version:2},route.request())});});
 await page.getByRole('button',{name:'确认重命名',exact:true}).click();await expect.poll(()=>renamed).toEqual({name:'研发新名',version:1,queryVersion:'synthetic-q36-context'});
});
test('R3 complete assignment choices load beyond the first 100 templates',async({page})=>{
 await fixture(page);const choices=Array.from({length:100},(_,i)=>({...template,id:'00000000-0000-4000-8000-'+String(200+i).padStart(12,'0'),name:'模板'+i,affectedIdentities:0}));
 const last={...template,id:'00000000-0000-4000-8000-000000000399',name:'第101个模板',affectedIdentities:0};
 await page.route('**/api/v1/personnel/templates?**',async route=>{const pageNo=Number(fixtureRequestURL(route.request()).searchParams.get('page')||1);await route.fulfill({status:200,json:q36FixtureEnvelope({items:pageNo===2?[last]:choices,total:101,page:pageNo,pageSize:100},route.request())});});
 await page.goto('/app/admin');await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();
 await expect(page.getByRole('checkbox',{name:'模板：第101个模板',exact:true})).toBeVisible();
});
test('R3 identity search and next page query the backend while keeping explicit selection',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/identities?**',async route=>{const url=fixtureRequestURL(route.request());queries.push(url.search);await route.fulfill({status:200,json:q36FixtureEnvelope({items:[identity],total:21,page:Number(url.searchParams.get('page')||1),pageSize:20},route.request())});});
 await page.getByLabel('搜索身份',{exact:true}).fill('共享');await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('search')==='共享')).toBe(true);
 await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
});
test('R3 member identity filter and activity pagination are server controls',async({page})=>{
 await admin(page);const memberQueries:string[]=[];await page.route('**/api/v1/personnel/members/search',async route=>{memberQueries.push(fixtureRequestURL(route.request()).search);await route.fulfill({status:200,json:q36FixtureEnvelope(list([member]),route.request())});});
 await expect(page.getByLabel('筛选身份',{exact:true})).toBeVisible();await chooseOption(page,'筛选身份','企业管理员');await expect.poll(()=>memberQueries.some(q=>new URLSearchParams(q).get('identityId')===identity.id)).toBe(true);
 await page.getByRole('tab',{name:'操作记录',exact:true}).click();const eventQueries:string[]=[];await page.route('**/api/v1/personnel/events/search',async route=>{const url=fixtureRequestURL(route.request());eventQueries.push(url.search);await route.fulfill({status:200,json:q36FixtureEnvelope({items:[],total:21,page:Number(url.searchParams.get('page')||1),pageSize:20},route.request())});});
 await page.getByLabel('搜索操作记录',{exact:true}).fill('活动');await expect(page.getByRole('button',{name:'下一页',exact:true})).toBeEnabled();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>eventQueries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
});
test('R3 invitation uses the existing invitationCode response and clears it on close',async({page})=>{
 await admin(page);await page.route('**/api/v1/invitations',route=>route.fulfill({status:201,json:q36FixtureEnvelope({id:'00000000-0000-4000-8000-000000000006',invitationCode:'synthetic-existing-contract'},route.request())}));
 await page.getByRole('button',{name:'邀请成员',exact:true}).click();await page.getByRole('button',{name:'生成邀请码',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).toHaveValue('synthetic-existing-contract');
 await page.getByRole('button',{name:'关闭',exact:true}).click();await page.getByRole('button',{name:'邀请成员',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).toBeHidden();await expect(page.getByRole('button',{name:'生成邀请码',exact:true})).toBeVisible();
});

test('R3 activity pagination and explicit time range reach the server independently',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/events/search',async route=>{const url=fixtureRequestURL(route.request());queries.push(url.search);await route.fulfill({status:200,json:q36FixtureEnvelope({items:[],total:21,page:Number(url.searchParams.get('page')||1),pageSize:20},route.request())});});
 await page.getByLabel('搜索操作记录',{exact:true}).fill('分页');await expect(page.getByRole('button',{name:'下一页',exact:true})).toBeEnabled();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
 await page.getByText('最近 7 天',{exact:true}).click();await page.getByLabel('开始时间',{exact:true}).fill('2026-09-01T00:00');await page.getByLabel('结束时间',{exact:true}).fill('2026-09-02T00:00');await page.getByRole('button',{name:'应用时间范围',exact:true}).click();
 await expect.poll(()=>queries.some(q=>{const p=new URLSearchParams(q);return p.get('from')===new Date('2026-09-01T00:00').toISOString()&&p.get('to')===new Date('2026-09-02T00:00').toISOString()&&p.get('page')==='1';})).toBe(true);
});
test('R3 explicit activity time filter sends RFC3339 and resets the page',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/events/search',async route=>{queries.push(fixtureRequestURL(route.request()).search);await route.fulfill({status:200,json:q36FixtureEnvelope(list([]),route.request())});});
 await page.getByText('最近 7 天',{exact:true}).click();await page.getByLabel('开始时间',{exact:true}).fill('2026-09-01T00:00');await page.getByLabel('结束时间',{exact:true}).fill('2026-09-02T00:00');await page.getByRole('button',{name:'应用时间范围',exact:true}).click();
 await expect.poll(()=>queries.some(q=>{const p=new URLSearchParams(q);return p.get('from')===new Date('2026-09-01T00:00').toISOString()&&p.get('to')===new Date('2026-09-02T00:00').toISOString();})).toBe(true);
});
test('R3 empty department deletion confirms version and preserves conflict feedback',async({page})=>{
 await fixture(page);const d={...department,id:'00000000-0000-4000-8000-000000000010',name:'空部门',parentId:department.id,isRoot:false};
 await page.route('**/api/v1/personnel/departments',route=>route.fulfill({status:200,json:q36FixtureEnvelope({items:[department,d]},route.request())}));await page.goto('/app/admin');await page.getByRole('button',{name:/^空部门/}).click();
 let version='';await page.route('**/api/v1/personnel/departments/'+d.id+'?**',async route=>{version=fixtureRequestURL(route.request()).searchParams.get('version')||'';await route.fulfill({status:409,json:{code:'PERSONNEL_CONFLICT',message:'conflict',data:null,meta:null}});});
 await page.getByRole('button',{name:'删除部门',exact:true}).click();await page.getByRole('button',{name:'确认删除',exact:true}).click();await expect.poll(()=>version).toBe('1');await expect(page.getByRole('alert')).toContainText('配置已变更');
});
for(const mode of ['department','member','groups'] as const)test('R3 unsaved '+mode+' form survives close and Escape until explicit discard',async({page})=>{
 await admin(page);
 if(mode==='department'){await page.getByRole('button',{name:'新建部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('保留部门');}
 else if(mode==='member'){await page.getByRole('button',{name:'配置身份',exact:true}).click();await page.getByLabel('身份：企业管理员',{exact:true}).check();}
 else{await page.getByRole('button',{name:'调整分组',exact:true}).click();await chooseOption(page,'目标部门','企业');}
 await page.getByRole('button',{name:'关闭',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'继续编辑',exact:true}).click();
 if(mode==='department')await expect(page.getByLabel('部门名称',{exact:true})).toHaveValue('保留部门');else if(mode==='member')await expect(page.getByLabel('身份：企业管理员',{exact:true})).toBeChecked();else await expect(page.getByRole('combobox',{name:'目标部门',exact:true})).toContainText('企业');
 await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('dialog')).toBeHidden();
});

test('Root Mono shell retains white surface and inset content well at large sizes',async({page})=>{
 await admin(page);
 for(const viewport of [{width:1920,height:1080},{width:2504,height:1355}]){
  await page.setViewportSize(viewport);await assertMonochromeLayout(page);
  await expect(page.getByTestId('workspace-content-well')).toHaveCSS('background-color','rgb(245, 245, 245)');
  await page.screenshot({path:test.info().outputPath('visual-admin-monochrome-'+viewport.width+'.png')});
 }
});

test('Q25 default activity range omits optional empty timestamps in every initial request',async({page})=>{
 await fixture(page);const queries:URLSearchParams[]=[];
 await page.route('**/api/v1/personnel/events/search',async route=>{queries.push(fixtureRequestURL(route.request()).searchParams);await route.fulfill({status:200,json:q36FixtureEnvelope(list([]),route.request())});});
 await page.goto('/app/admin');await expect(page.getByLabel('搜索成员',{exact:true})).toBeVisible();
 expect(queries.length).toBeGreaterThan(0);for(const query of queries){expect(query.has('from')).toBe(false);expect(query.has('to')).toBe(false);}
});
test('Figma original checkbox uses the original check SVG, 20px surface and 6px corners',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'企业管理',exact:true}).click();
 const box=page.getByLabel('中央权限：人员管理',{exact:true});await expect(box).toHaveCSS('appearance','none');await expect(box).toHaveCSS('border-radius','6px');await expect(box).toHaveCSS('width','20px');await expect(box).toHaveCSS('background-size','14px 14px');
 const original=readFileSync(new URL('./assets/admin-check.svg',import.meta.url),'utf8');
 expect(await box.evaluate((node,source)=>{const url=getComputedStyle(node).backgroundImage.slice(5,-2);const raw=url.startsWith('data:image/svg+xml;base64,')?atob(url.split(',')[1]):decodeURIComponent(url.slice(url.indexOf(',')+1));const canonical=(svg:string)=>new XMLSerializer().serializeToString(new DOMParser().parseFromString(svg.replace(/>\s+</g,'><').trim(),'image/svg+xml'));return canonical(raw)===canonical(source);},original)).toBe(true);
 await box.uncheck();await expect(box).toHaveCSS('background-image','none');
});
test('Root Mono shell has the approved white foreground and pale neutral background',async({page})=>{
 await admin(page);await expect(page.getByTestId('workspace-surface')).toHaveCSS('background-color','rgb(255, 255, 255)');
 await expect(page.getByTestId('workspace-shell')).toHaveCSS('background-color','rgb(250, 250, 250)');
});
test('R3 activity presents readable action, target and safe changes rather than raw protocol JSON',async({page})=>{
 await fixture(page);const event={id:'00000000-0000-4000-8000-000000000020',occurredAt:'2026-09-30T00:00:00Z',actorAccount:'synthetic-operator',action:'DEPARTMENT_UPDATED',objectType:'department',objectId:department.id,summary:{before:{name:'研发部',parentId:department.id},after:{name:'研发中心',parentId:department.id}},outcome:'success'};
 await page.route('**/api/v1/personnel/events/search',route=>route.fulfill({status:200,json:q36FixtureEnvelope(list([event]),route.request())}));await page.goto('/app/admin');await page.getByRole('tab',{name:'操作记录',exact:true}).click();
 const row=page.getByRole('row').filter({hasText:'synthetic-operator'});await expect(row).toContainText('重命名部门');await expect(row).toContainText('研发中心');await expect(row).toContainText('研发部 → 研发中心');await expect(row).not.toContainText('DEPARTMENT_UPDATED');await expect(row).not.toContainText('parentId');
 const filtered=page.waitForRequest(request=>fixtureRequestURL(request).searchParams.get('action')==='MEMBER_IDENTITIES_UPDATED');await chooseOption(page,'操作类型','分配身份');expect(fixtureRequestURL(await filtered).searchParams.get('action')).toBe('MEMBER_IDENTITIES_UPDATED');
});
test('Root Mono personnel navigation stays reachable at viewport edges after resize',async({page})=>{
 await admin(page);for(const width of [390,1920]){await page.setViewportSize({width,height:1080});await assertMonochromeLayout(page);}
});
test('Q34 member table keeps confirmed column geometry and page selection never sends a write',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);const th=page.locator('.member-table th');
 for(const [index,width] of [[0,48],[2,176],[3,216],[4,136],[5,192]])expect((await th.nth(index).boundingBox())!.width).toBeCloseTo(width,0);
 let writes=0;page.on('request',request=>{if(request.url().includes('/api/v1/')&&!['GET','HEAD'].includes(request.method())&&!request.url().endsWith('/search'))writes++;});
 const select=page.getByLabel('选择成员：'+user.account,{exact:true});await select.check();await expect(page.getByLabel('选择当前页成员',{exact:true})).toBeChecked();await page.getByLabel('选择当前页成员',{exact:true}).uncheck();await expect(select).not.toBeChecked();expect(writes).toBe(0);
});
test('Root Mono surface retains viewport edges after narrow re-entry',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.setViewportSize({width:390,height:844});await page.reload();await assertMonochromeLayout(page);
});

for(const [tab,label] of [['身份','新建身份'],['权限模板','新建权限模板']] as const){
 test('Figma final view original white plus asset on '+tab,async({page})=>{
  await admin(page);await page.getByRole('tab',{name:tab,exact:true}).click();
  const icon=page.getByRole('button',{name:label,exact:true}).locator('img');await expect(icon).toHaveCount(1);await expect(icon).toHaveAttribute('width','16');await expect(icon).toHaveAttribute('height','16');
 });
}
test('Figma final template permission retains the actual controlled code',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'企业管理',exact:true}).click();await expect(page.locator('.permission-item').getByText('personnel.manage',{exact:true})).toBeVisible();
});
test('Figma final identity distinguishes assignable management from trusted Root',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await expect(page.getByText('人员管理可授予普通成员；它不是 Root 身份。',{exact:true})).toBeVisible();
});
test('Figma final activity action filter belongs in the section heading',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();await expect(page.locator('.section-heading').getByLabel('操作类型',{exact:true})).toBeVisible();await expect(page.getByLabel('操作类型',{exact:true})).toHaveCSS('height','38px');
});
test('Figma final activity time range retains the right toolbar inset',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const b=await page.locator('.event-time-filter summary').boundingBox();const toolbar=(await page.locator('.activity-table .table-toolbar').boundingBox())!;expect(b!.x).toBeGreaterThanOrEqual(toolbar.x);expect(b!.x+b!.width).toBeLessThanOrEqual(toolbar.x+toolbar.width);
});
test('Figma final definition footer retains its source separation line',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'企业管理',exact:true}).click();await expect(page.locator('.definition-footer')).toHaveCSS('border-top-width','1px');
});

test('Figma final identity without applications keeps its footer in the original view',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await expect(page.locator('.definition-details .no-applications')).toHaveCount(0);await expect(page.locator('.definition-footer')).toBeInViewport();
});
test('Q34 successful activity uses Arca body typography and retains the result meaning',async({page})=>{
 await admin(page);await page.route('**/api/v1/personnel/events/search',route=>route.fulfill({json:q36FixtureEnvelope(list([{id:user.id,occurredAt:'2026-09-30T05:00:00Z',actorAccount:user.account,action:'TEMPLATE_UPDATED',objectType:'template',objectId:template.id,summary:{before:{name:'旧名称'},after:{name:template.name}},outcome:'success'}]),route.request())}));await page.getByRole('tab',{name:'操作记录',exact:true}).click();await page.getByLabel('搜索操作记录').fill('模板');const result=page.locator('.activity-table tbody tr[data-row-id] td').last();await expect(result).toHaveText('已完成');await expect(result).toHaveCSS('color','oklch(0.145 0 0)');await expect(result).toHaveCSS('font-size','14px');await expect(result).toHaveCSS('line-height','20px');
});


// Frozen Figma 485:3621: title/tabs and one content toolbar, not stacked legacy chrome.
test('Root Mono personnel desktop keeps the primary table in the first working area',async({page})=>{
 await page.setViewportSize({width:1440,height:1000});await admin(page);
 const title=page.getByRole('heading',{name:'人员管理',exact:true});
 await expect(title).toBeVisible();expect((await title.boundingBox())!.y).toBeLessThanOrEqual(150);
 const header=page.locator('.member-table').getByRole('table').locator('thead');
 await expect(header).toBeVisible();expect((await header.boundingBox())!.y).toBeLessThanOrEqual(350);
 for(const name of ['新建部门','邀请成员','刷新查询','退出'])await expect(page.getByRole('button',{name,exact:true})).toBeVisible();
 await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toHaveAttribute('aria-selected','true');
 await page.screenshot({path:test.info().outputPath('monochrome-admin-primary-density.png'),fullPage:true});
});
