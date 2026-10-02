import {test,expect,type Page,type Route} from '@playwright/test';

const id='00000000-0000-4000-8000-000000000001';
const identityId='00000000-0000-4000-8000-000000000002';
const draftId='00000000-0000-4000-8000-000000000009';
const user={id,account:'q36-admin'};
const member={...user,status:'active',bootstrapAdmin:true,version:0,departmentIds:[],identityIds:[],departments:[],identities:[],permissions:[]};
const identity={id:identityId,name:'测试身份',description:'原说明',version:1,templateIds:[],permissionCodes:[],affectedMembers:0,affectedIdentities:0};
const envelope=(data:unknown)=>({code:'OK',message:'success',data,meta:null});
const paging=(items:unknown[],input:{page?:number;pageSize?:number}={})=>({items,total:100,page:input.page??1,pageSize:input.pageSize??20,queryVersion:'context-'+(input.page??1),sort:null});
async function fixture(page:Page){
 let preset:Record<string,unknown>|null=null;
 await page.route('**/api/v1/**',async r=>{
  const path=new URL(r.request().url()).pathname.replace('/api/v1/','');
  if(path.startsWith('personnel/table-presets')){if(r.request().method()==='POST'){preset={...r.request().postDataJSON(),id:draftId,version:1,createdAt:'2026-10-02T03:00:00Z',updatedAt:'2026-10-02T03:00:00Z'};return r.fulfill({status:201,json:envelope(preset)});}return r.fulfill({json:envelope(path==='personnel/table-presets'?{items:preset?[preset]:[]}:preset)});}
  const data=path==='sessions/current'?user:path==='me/access'?{user,bootstrapAdmin:true,personnelManage:true,identities:[],permissions:[],applications:[]}:
   path==='personnel/departments'?{items:[]}:path==='personnel/permissions'?{items:[]}:
   path==='personnel/identities'?{items:[identity],total:1,page:1,pageSize:20}:path==='personnel/identities/'+identityId?identity:
   path==='personnel/templates'?{items:[],total:0,page:1,pageSize:20}:path==='personnel/members/'+id?member:
   path==='personnel/members'||path==='personnel/members/search'?paging([member]):
   path==='personnel/events'||path==='personnel/events/search'?{...paging([]),range:{from:'2026-09-24T00:00:00Z',to:'2026-10-02T00:00:00Z'}}:
   path==='personnel/drafts'?{items:[]}:{};
  await r.fulfill({json:envelope(data)});
 });
}
async function admin(page:Page){await page.goto('/app/admin');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();}
async function condition(page:Page,wait=true){
 await page.getByRole('button',{name:'自定义筛选',exact:true}).click();
 const panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await expect(panel).toBeVisible();
 await panel.getByRole('button',{name:'新增筛选',exact:true}).click();const editor=page.getByRole('dialog',{name:'新增自定义筛选',exact:true});
 await editor.getByLabel('自定义筛选名称',{exact:true}).fill('Q36 回归方案');await editor.getByRole('button',{name:'或条件',exact:true}).click();await editor.getByLabel('条件 1.1 值',{exact:true}).fill('q36-admin');await editor.getByRole('button',{name:'确定',exact:true}).click();
 await panel.getByRole('button',{name:'应用Q36 回归方案',exact:true}).click();
 if(wait){await expect(panel.getByRole('button',{name:'关闭筛选管理',exact:true})).toBeEnabled();await panel.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await expect(panel).toBeHidden();}
}
async function cancelCondition(page:Page){
 await page.getByRole('button',{name:'自定义筛选，已应用',exact:true}).click();const p=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await p.getByRole('button',{name:'取消应用',exact:true}).click();await page.getByRole('dialog',{name:'取消当前筛选',exact:true}).getByRole('button',{name:'确认取消',exact:true}).click();await expect(p.getByRole('button',{name:'关闭筛选管理',exact:true})).toBeEnabled();await p.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await expect(p).toBeHidden();
}

test('Q36 B2 real member page uses POST typed filter and old context without text sorting',async({page})=>{
 await fixture(page);const requests:Record<string,unknown>[]=[];
 await page.route('**/personnel/members/search',r=>{expect(r.request().method()).toBe('POST');const input=r.request().postDataJSON();requests.push(input);return r.fulfill({json:envelope(paging([member],input))});});
 await admin(page);await expect.poll(()=>requests.length).toBeGreaterThan(0);
 await expect(page.locator('.member-table th[aria-sort]')).toHaveCount(0);
 await condition(page);await expect.poll(()=>requests.at(-1)?.filter).toEqual({operator:'and',children:[{field:'account',operator:'eq',value:'q36-admin'}]});
 expect(requests.at(-1)?.queryVersion).toBe('context-1');
 await page.getByLabel('跳至页',{exact:true}).fill('4');await page.getByLabel('跳至页',{exact:true}).press('Enter');
 await expect.poll(()=>requests.at(-1)?.page).toBe(4);
});

test('Q36 B2 event page uses server display and only occurredAt server ordering',async({page})=>{
 await fixture(page);const requests:Record<string,unknown>[]=[];
 const event={id,occurredAt:'2026-10-01T00:00:00Z',actorAccount:'other-account',action:'MEMBER_IDENTITIES_UPDATED',outcome:'success',objectType:'member',objectId:id,summary:null,display:{action:'后端操作',object:'当前页之外的成员',detail:'后端安全摘要',outcome:'已完成'}};
 await page.route('**/personnel/events/search',r=>{const input=r.request().postDataJSON();requests.push(input);return r.fulfill({json:envelope({...paging([event],input),sort:{key:'occurredAt',direction:input.sortDirection??'desc'},range:{from:'2026-09-24T00:00:00Z',to:'2026-10-02T00:00:00Z'}})});});
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();
 await expect(page.locator('.activity-table')).toContainText('当前页之外的成员');await expect(page.locator('.activity-table')).toContainText('后端安全摘要');
 await page.getByRole('button',{name:'排序 occurredAt',exact:true}).click();await page.getByRole('menuitem',{name:'升序',exact:true}).click();
 await expect.poll(()=>requests.at(-1)?.sortDirection).toBe('asc');expect(requests.at(-1)?.sortBy).toBe('occurredAt');expect(requests.at(-1)?.queryVersion).toBeTruthy();
 expect(await page.locator('.activity-table th[aria-sort]').count()).toBe(1);
 const prior=requests.length;
 await page.getByRole('button',{name:'排序 occurredAt',exact:true}).click();await page.getByRole('menuitem',{name:'升序',exact:true}).click();
 await expect.poll(()=>requests.length).toBeGreaterThan(prior);
 expect(requests.at(-1)?.sortBy).toBeUndefined();expect(requests.at(-1)?.sortDirection).toBeUndefined();
 expect(requests.at(-1)?.queryVersion).toBeTruthy();expect(requests.at(-1)?.from).toBe('2026-09-24T00:00:00Z');expect(requests.at(-1)?.to).toBe('2026-10-02T00:00:00Z');
 await expect(page.locator('.activity-table th[aria-sort]')).toHaveCount(0);
});

test('Q36 B2 restored invalid identity reference remains editable until explicitly resolved',async({page})=>{
 await fixture(page);const missing='00000000-0000-4000-8000-000000000088';
 const stored={id:draftId,kind:'member-identities',targetId:id,baseVersion:0,version:1,createdAt:'2026-10-01T00:00:00Z',updatedAt:'2026-10-01T00:00:00Z',payload:{identityIds:[missing]}};
 await page.route('**/personnel/drafts',r=>r.fulfill({json:envelope({items:[stored]})}));await page.route('**/personnel/drafts/'+draftId,r=>r.fulfill({json:envelope(stored)}));
 await admin(page);await page.getByRole('button',{name:'草稿箱',exact:true}).click();await page.getByRole('button',{name:'恢复草稿',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('已失效引用');
 const invalid=page.getByLabel('已失效身份：'+missing,{exact:true});await expect(invalid).toBeChecked();await invalid.click();await expect(invalid).toHaveCount(0);
 await page.getByRole('button',{name:'保留当前输入，使用最新对象版本',exact:true}).click();await expect(page.getByRole('alert')).toHaveCount(0);
});

test('Q36 B2 modal query refresh preserves unsaved member input and original object version',async({page})=>{
 await fixture(page);await admin(page);await page.getByRole('button',{name:'配置身份',exact:true}).click();const dialog=page.getByRole('dialog',{name:'配置成员身份',exact:true});
 await dialog.getByLabel('身份：测试身份',{exact:true}).check();await dialog.getByRole('button',{name:'刷新查询',exact:true}).click();await expect(dialog.getByLabel('身份：测试身份',{exact:true})).toBeChecked();
});

for(const code of ['COMMON_QUERY_CHANGED','COMMON_QUERY_CONTEXT_EXPIRED'])test('Q36 B2 '+code+' requires explicit page-one refresh and clears selection',async({page})=>{
 await fixture(page);let failed=false;const requests:Record<string,unknown>[]=[];
 await page.route('**/personnel/members/search',r=>{const input=r.request().postDataJSON();requests.push(input);if(input.page===2&&!failed){failed=true;return r.fulfill({status:409,json:{code,message:'conflict',data:null,meta:null}});}return r.fulfill({json:envelope(paging([member],input))});});
 await admin(page);await page.getByLabel('选择成员：q36-admin',{exact:true}).check();
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByLabel('跳至页',{exact:true}).press('Enter');
 await expect(page.getByRole('alert')).toContainText(code==='COMMON_QUERY_CHANGED'?'查询结果已变化':'查询上下文已过期');
 await page.getByRole('button',{name:'刷新查询',exact:true}).click();
 await expect.poll(()=>requests.at(-1)?.page).toBe(1);expect(requests.at(-1)?.queryVersion).toBeUndefined();
 await expect(page.getByLabel('选择成员：q36-admin',{exact:true})).not.toBeChecked();
});

test('Q36 B2 late responses cannot overwrite the newest search',async({page})=>{
 await fixture(page);let release:((r:Route)=>Promise<void>)|undefined;let oldRoute:Route|undefined;
 await page.route('**/personnel/members/search',async r=>{const input=r.request().postDataJSON();if(input.search==='older'){oldRoute=r;release=async route=>{await route.fulfill({json:envelope(paging([{...member,account:'late-old-response'}],input))}).catch(()=>{});};return;}await r.fulfill({json:envelope(paging([{...member,account:input.search==='newer'?'newest-response':member.account}],input))});});
 await admin(page);await page.getByLabel('搜索成员',{exact:true}).fill('older');await expect.poll(()=>!!oldRoute).toBe(true);
 await page.getByLabel('搜索成员',{exact:true}).fill('newer');await expect(page.locator('.member-table')).toContainText('newest-response');
 if(release&&oldRoute)await release(oldRoute);await page.waitForTimeout(200);await expect(page.locator('.member-table')).not.toContainText('late-old-response');
});

test('Q36 B2 unused template card save keeps member page and validates its existing context on return',async({page})=>{
 await fixture(page);const queries:Record<string,unknown>[]=[];
 await page.route('**/personnel/members/search',r=>{const input=r.request().postDataJSON();queries.push(input);return r.fulfill({json:envelope(paging([member],input))});});
 await page.route('**/personnel/templates',r=>r.fulfill({json:envelope({...identity,id:draftId,name:'不影响成员结果的新模板',templateIds:[]})}));
 await admin(page);await page.getByLabel('跳至页',{exact:true}).fill('4');await page.getByLabel('跳至页',{exact:true}).press('Enter');await expect(page.getByRole('button',{name:'Page 4',exact:true})).toHaveAttribute('aria-current','page');
 await expect(page.locator('.member-table table')).toHaveAttribute('aria-busy','false');const beforeSave=queries.length;
 await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'新建权限模板',exact:true}).click();await page.getByLabel('模板名称',{exact:true}).fill('不影响成员结果的新模板');
 await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect(page.getByRole('status').filter({hasText:'已保存'})).toBeVisible();
 await page.getByRole('tab',{name:'成员与部门',exact:true}).click();await expect(page.locator('.member-table table')).toHaveAttribute('aria-busy','false');
 await expect.poll(()=>queries.length).toBeGreaterThan(beforeSave);await expect(page.getByRole('button',{name:'Page 4',exact:true})).toHaveAttribute('aria-current','page');expect(queries.at(-1)?.queryVersion).toBe('context-4');expect(queries.slice(beforeSave).every(q=>!!q.queryVersion)).toBe(true);
});

test('Q36 B2 definition explicitly saves durable draft and restores original version with conflict',async({page})=>{
 await fixture(page);let saved:Record<string,unknown>|undefined;let current=identity;
 await page.route('**/personnel/identities/'+identityId,r=>r.fulfill({json:envelope(current)}));
 await page.route('**/personnel/drafts',r=>{if(r.request().method()==='POST'){const input=r.request().postDataJSON();saved={...input,id:draftId,version:1,createdAt:'2026-10-01T00:00:00Z',updatedAt:'2026-10-01T00:00:00Z'};return r.fulfill({status:201,json:envelope(saved)});}return r.fulfill({json:envelope({items:saved?[saved]:[]})});});
 await page.route('**/personnel/drafts/'+draftId,r=>r.fulfill({json:envelope(saved)}));
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'测试身份',exact:true}).click();
 await page.getByLabel('说明',{exact:true}).fill('尚未提交的输入');expect(saved).toBeUndefined();
 await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect.poll(()=>saved?.baseVersion).toBe(1);
 expect(saved?.payload).toMatchObject({description:'尚未提交的输入'});current={...identity,version:2,description:'其他标签已更新'};
 await page.reload();await page.getByRole('button',{name:'草稿箱',exact:true}).click();await page.getByRole('button',{name:'恢复草稿',exact:true}).first().click();
 await expect(page.getByLabel('说明',{exact:true})).toHaveValue('尚未提交的输入');await expect(page.getByRole('alert')).toContainText('原版本 1');await expect(page.getByRole('alert')).toContainText('最新版本 2');
 await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();expect(saved?.baseVersion).toBe(1);
 await page.setViewportSize({width:390,height:844});
 const resolve=page.getByRole('button',{name:'保留当前输入，使用最新对象版本',exact:true});await resolve.scrollIntoViewIfNeeded();
 const box=(await resolve.boundingBox())!;
 expect(box.x).toBeGreaterThanOrEqual(176);expect(box.x+box.width).toBeLessThanOrEqual(390);
});

test('Q36 B2 explicit member draft does not save automatically or bypass query guard',async({page})=>{
 await fixture(page);let saved:Record<string,unknown>|undefined;let write:Record<string,unknown>|undefined;
 await page.route('**/personnel/drafts',r=>{if(r.request().method()==='POST'){saved={...r.request().postDataJSON(),id:draftId,version:3};return r.fulfill({status:201,json:envelope(saved)});}return r.fulfill({json:envelope({items:[]})});});
 await page.route('**/personnel/members/'+id+'/identities',r=>{write=r.request().postDataJSON();return r.fulfill({json:envelope(member)});});
 await admin(page);await page.getByRole('button',{name:'配置身份',exact:true}).click();await page.getByLabel('身份：测试身份',{exact:true}).check();expect(saved).toBeUndefined();
 await page.getByRole('button',{name:'保存草稿',exact:true}).click();await expect.poll(()=>saved?.kind).toBe('member-identities');
 await page.getByRole('button',{name:'确认分配',exact:true}).click();await expect.poll(()=>write?.queryVersion).toBe('context-1');expect(write?.draftRef).toEqual({id:draftId,version:3});
});

test('Q36 B2 real-page animation closes on navigation and leaves no root blocker',async({page})=>{
 await fixture(page);await admin(page);await page.getByRole('button',{name:'自定义筛选',exact:true}).click();await expect(page.getByRole('dialog',{name:'管理自定义筛选',exact:true})).toBeVisible();
 await page.keyboard.press('Escape');await page.getByRole('button',{name:'自定义筛选',exact:true}).click();await page.keyboard.press('Escape');
 await page.getByRole('tab',{name:'操作记录',exact:true}).click();await page.waitForTimeout(1000);
 await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);
 await expect(page.getByRole('button',{name:'自定义筛选',exact:true}).locator('svg')).toBeVisible();
});

test('Q36 B2 applied preset manager close then trigger press honors reopen intent',async({page})=>{
 await fixture(page);await admin(page);await condition(page);await page.getByRole('button',{name:'自定义筛选，已应用',exact:true}).click();const panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await expect(panel).toBeVisible();
 await page.evaluate(()=>{(document.querySelector('.preset-close') as HTMLButtonElement).click();(document.querySelector('.q36-filter-trigger') as HTMLButtonElement).click();});
 await expect(page.locator('.q36-filter-content')).toBeVisible();await expect(panel).toBeVisible();await page.waitForTimeout(1000);await expect(panel).toBeVisible();
 await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);
});

test('Q36 B2 twenty AND conditions keep the save footer inside the narrow viewport',async({page})=>{
 await fixture(page);await page.setViewportSize({width:1440,height:1000});await admin(page);await page.getByRole('button',{name:'自定义筛选',exact:true}).click();const manager=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await manager.getByRole('button',{name:'新增筛选',exact:true}).click();const panel=page.getByRole('dialog',{name:'新增自定义筛选',exact:true});await panel.getByLabel('自定义筛选名称',{exact:true}).fill('二十条件');
 await panel.getByRole('button',{name:'或条件',exact:true}).click();
 for(let i=1;i<=20;i++){if(i>1)await panel.getByRole('button',{name:'组 1 且条件',exact:true}).click();await panel.getByLabel('条件 1.'+i+' 值',{exact:true}).fill('SyntheticLongAccountOutsidePageScope');}
 await page.setViewportSize({width:390,height:844});const apply=panel.getByRole('button',{name:'确定',exact:true});await expect(apply).toBeEnabled();
 await expect.poll(async()=>{const box=(await apply.boundingBox())!;return box.y+box.height;}).toBeLessThanOrEqual(844);
 await expect(apply).toBeInViewport();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('Q36 B2 select-current-page cannot resurrect a member hidden by custom filtering',async({page})=>{
 await fixture(page);const other={...member,id:identityId,account:'other-visible-member'};
 await page.route('**/personnel/members/search',r=>{const input=r.request().postDataJSON();return r.fulfill({json:envelope({...paging(input.filter?[member]:[member,other],input),total:input.filter?1:2})});});
 await admin(page);await page.getByLabel('选择成员：other-visible-member',{exact:true}).check();await condition(page);
 await expect(page.getByLabel('选择成员：other-visible-member',{exact:true})).toHaveCount(0);
 await expect(page.getByLabel('选择成员：q36-admin',{exact:true})).toBeVisible();
 await expect(page.locator('.member-table table')).toHaveAttribute('aria-busy','false');
 await page.getByLabel('选择当前页成员',{exact:true}).check();await expect(page.getByLabel('选择成员：q36-admin',{exact:true})).toBeChecked();
 await cancelCondition(page);
 await expect(page.getByLabel('选择成员：other-visible-member',{exact:true})).not.toBeChecked();await expect(page.getByLabel('选择当前页成员',{exact:true})).not.toBeChecked();
});

test('Q36 B2 a trusted press spanning the exiting selection mark still selects current rows',async({page})=>{
 await page.addInitScript(()=>{const events:{type:string;trusted:boolean;target:string}[]=[];Object.defineProperty(window,'__q36SelectionPress',{value:events});for(const type of ['pointerdown','pointerup','click'])document.addEventListener(type,event=>{const target=event.target as Element;if(target.closest?.('[aria-label="选择当前页成员"]')){if(type==='pointerdown')events.length=0;events.push({type,trusted:event.isTrusted,target:target.tagName,time:event.timeStamp,detail:event instanceof MouseEvent?event.detail:0,active:document.activeElement?.getAttribute('aria-label'),selected:target.closest('[aria-label="选择当前页成员"]')?.getAttribute('aria-checked')});}},true);});
 await fixture(page);const other={...member,id:identityId,account:'other-visible-member'};
 await page.route('**/personnel/members/search',r=>{const input=r.request().postDataJSON();return r.fulfill({json:envelope({...paging(input.filter?[member]:[member,other],input),total:input.filter?1:2})});});
 await admin(page);await page.getByLabel('选择成员：other-visible-member',{exact:true}).check();
 await condition(page);const panel=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await expect(panel).toBeHidden();
 const all=page.getByLabel('选择当前页成员',{exact:true});await all.check();await expect(page.getByLabel('选择成员：q36-admin',{exact:true})).toBeChecked();
 const box=(await all.boundingBox())!;await all.focus();await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.keyboard.press('Space');
 // Drive a real press during the existing exit, then release after its actual
 // DOM removal. No sleep, timeout change or click retry determines the result.
 await page.waitForFunction(()=>{const table=document.querySelector('.member-table table'),all=document.querySelector('[aria-label="选择当前页成员"]');return table?.getAttribute('aria-busy')==='false'&&all?.getAttribute('aria-checked')==='false'&&!!document.querySelector('[aria-label="选择成员：q36-admin"]')&&!document.querySelector('[aria-label="选择成员：other-visible-member"]')&&!!all?.querySelector('svg');});
 await page.mouse.down();await expect(all.locator('svg')).toHaveCount(0);await page.mouse.up();
 await test.info().attach('trusted-selection-press.json',{body:JSON.stringify(await page.evaluate(()=>({events:(window as unknown as {__q36SelectionPress:unknown}).__q36SelectionPress,active:document.activeElement?.getAttribute('aria-label'),button:document.querySelector('[aria-label="选择当前页成员"]')?.outerHTML,transitions:document.documentElement.className}))),contentType:'application/json'});
 await expect(page.getByLabel('选择成员：q36-admin',{exact:true})).toBeChecked();await expect(all).toBeChecked();
 const events=await page.evaluate(()=>(window as unknown as {__q36SelectionPress:{type:string;trusted:boolean;target:string}[]}).__q36SelectionPress);
 expect(events.filter(event=>event.type==='click'&&event.trusted)).toHaveLength(1);
 await cancelCondition(page);
 await expect(page.getByLabel('选择成员：other-visible-member',{exact:true})).not.toBeChecked();await expect(all).not.toBeChecked();
});

test('Q36 B2 trusted select-all clicks while loading or empty cannot persist into restored or next-page rows',async({page})=>{
 await fixture(page);const other={...member,id:identityId,account:'other-visible-member'},next={...member,id:draftId,account:'next-page-member'};
 let release!:()=>void;const gate=new Promise<void>(resolve=>{release=resolve;});
 await page.route('**/personnel/members/search',async r=>{const input=r.request().postDataJSON();if(input.filter)await gate;return r.fulfill({json:envelope({...paging(input.filter?[]:input.page===2?[next]:[member,other],input),total:input.filter?0:40})});});
 await admin(page);await page.getByLabel('选择成员：other-visible-member',{exact:true}).check();await condition(page,false);
 const all=page.getByLabel('选择当前页成员',{exact:true}),table=page.locator('.member-table table');
 await expect(table).toHaveAttribute('aria-busy','true');await expect(all).toBeDisabled();
 async function trustedClick(){const box=(await all.boundingBox())!;await page.mouse.click(box.x+box.width/2,box.y+box.height/2);}
 await trustedClick();await expect(page.getByLabel('选择成员：other-visible-member',{exact:true})).toBeChecked();
 release();await expect(table).toHaveAttribute('aria-busy','false');await expect(table.locator('tbody tr[data-row-id]')).toHaveCount(0);await expect(all).toBeDisabled();await trustedClick();await expect(all).not.toBeChecked();
 const manager=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await manager.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await expect(manager).toBeHidden();
 await cancelCondition(page);
 await expect(table).toHaveAttribute('aria-busy','false');await expect(page.getByLabel('选择成员：q36-admin',{exact:true})).not.toBeChecked();await expect(page.getByLabel('选择成员：other-visible-member',{exact:true})).not.toBeChecked();await expect(all).not.toBeChecked();
 await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByLabel('跳至页',{exact:true}).press('Enter');
 await expect(page.getByLabel('选择成员：next-page-member',{exact:true})).toBeVisible();await expect(table).toHaveAttribute('aria-busy','false');await expect(page.getByLabel('选择成员：next-page-member',{exact:true})).not.toBeChecked();await expect(all).not.toBeChecked();
});
