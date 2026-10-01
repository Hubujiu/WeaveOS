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
 await page.route('**/api/v1/**',async r=>{
  const path=new URL(r.request().url()).pathname.replace('/api/v1/','');
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
async function condition(page:Page){
 await page.getByRole('button',{name:'自定义筛选',exact:true}).click();
 const panel=page.getByRole('dialog',{name:'自定义筛选',exact:true});await expect(panel).toBeVisible();
 await panel.getByRole('button',{name:'添加条件',exact:true}).first().click();
 await panel.locator('.q36-filter-condition input').first().fill('q36-admin');
 await panel.getByRole('button',{name:'应用筛选',exact:true}).click();
}

test('Q36 B2 real member page uses POST typed filter and old context without text sorting',async({page})=>{
 await fixture(page);const requests:Record<string,unknown>[]=[];
 await page.route('**/personnel/members/search',r=>{expect(r.request().method()).toBe('POST');const input=r.request().postDataJSON();requests.push(input);return r.fulfill({json:envelope(paging([member],input))});});
 await admin(page);await expect.poll(()=>requests.length).toBeGreaterThan(0);
 await expect(page.locator('.member-table th[aria-sort]')).toHaveCount(0);
 await condition(page);await expect.poll(()=>requests.at(-1)?.filter).toEqual({operator:'and',children:[{field:'account',operator:'eq',value:'q36-admin'}]});
 expect(requests.at(-1)?.queryVersion).toBe('context-1');
 await page.getByLabel('跳至页码').fill('4');await page.getByLabel('跳至页码').press('Enter');
 await expect.poll(()=>requests.at(-1)?.page).toBe(4);
});

test('Q36 B2 event page uses server display and only occurredAt server ordering',async({page})=>{
 await fixture(page);const requests:Record<string,unknown>[]=[];
 const event={id,occurredAt:'2026-10-01T00:00:00Z',actorAccount:'other-account',action:'MEMBER_IDENTITIES_UPDATED',outcome:'success',objectType:'member',objectId:id,summary:null,display:{action:'后端操作',object:'当前页之外的成员',detail:'后端安全摘要',outcome:'已完成'}};
 await page.route('**/personnel/events/search',r=>{const input=r.request().postDataJSON();requests.push(input);return r.fulfill({json:envelope({...paging([event],input),sort:{key:'occurredAt',direction:input.sortDirection??'desc'},range:{from:'2026-09-24T00:00:00Z',to:'2026-10-02T00:00:00Z'}})});});
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();
 await expect(page.locator('.activity-table')).toContainText('当前页之外的成员');await expect(page.locator('.activity-table')).toContainText('后端安全摘要');
 await page.locator('.activity-table th').filter({hasText:'时间'}).getByRole('button').first().click();
 await expect.poll(()=>requests.at(-1)?.sortBy).toBe('occurredAt');expect(requests.at(-1)?.queryVersion).toBeTruthy();
 expect(await page.locator('.activity-table th[aria-sort]').count()).toBe(1);
});

for(const code of ['COMMON_QUERY_CHANGED','COMMON_QUERY_CONTEXT_EXPIRED'])test('Q36 B2 '+code+' requires explicit page-one refresh and clears selection',async({page})=>{
 await fixture(page);let failed=false;const requests:Record<string,unknown>[]=[];
 await page.route('**/personnel/members/search',r=>{const input=r.request().postDataJSON();requests.push(input);if(input.page===2&&!failed){failed=true;return r.fulfill({status:409,json:{code,message:'conflict',data:null,meta:null}});}return r.fulfill({json:envelope(paging([member],input))});});
 await admin(page);await page.getByLabel('选择成员：q36-admin',{exact:true}).check();
 await page.getByLabel('跳至页码').fill('2');await page.getByLabel('跳至页码').press('Enter');
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
 await fixture(page);await admin(page);await page.getByRole('button',{name:'自定义筛选',exact:true}).click();await expect(page.getByRole('dialog',{name:'自定义筛选',exact:true})).toBeVisible();
 await page.keyboard.press('Escape');await page.getByRole('button',{name:'自定义筛选',exact:true}).click();await page.keyboard.press('Escape');
 await page.getByRole('tab',{name:'操作记录',exact:true}).click();await page.waitForTimeout(1000);
 await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);
 await expect(page.getByRole('button',{name:'自定义筛选',exact:true}).locator('svg')).toBeVisible();
});
