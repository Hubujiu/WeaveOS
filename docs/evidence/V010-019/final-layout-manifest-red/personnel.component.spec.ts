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
  const path=new URL(route.request().url()).pathname.replace('/api/v1/','');
  const data=path==='sessions/current'?user:path==='sessions'?user:path==='me/access'?{user,bootstrapAdmin:manage,personnelManage:manage,identities:[],permissions:manage?catalog:[],applications:[]}:
   path==='personnel/departments'?{items:[department]}:path==='personnel/permissions'?{items:catalog}:
   path==='personnel/identities'?list([identity]):path==='personnel/templates'?list([template]):path==='personnel/members'?list([member]):path==='personnel/members/'+user.id?member:path==='personnel/events'?list([]):{};
  await route.fulfill({status:route.request().method()==='DELETE'?204:200,contentType:'application/json',body:route.request().method()==='DELETE'?'':JSON.stringify({code:'OK',message:'success',data,meta:null})});
 });
}
async function admin(page:Page){await fixture(page);await page.goto('/app/admin');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();}
test('R3 login enters Home with settings and account navigation; admin exit retains Session',async({page})=>{
 await fixture(page);let logouts=0;
 await page.route('**/api/v1/sessions/current',async route=>{if(route.request().method()==='DELETE') logouts++;await route.fulfill({status:200,json:{code:'OK',message:'success',data:user,meta:null}});});
 await page.goto('/login');await page.getByLabel('账号',{exact:true}).fill(user.account);await page.getByLabel('密码',{exact:true}).fill('Synthetic@123');await page.getByRole('button',{name:'登录',exact:true}).click();
 await expect(page).toHaveURL(/\/app$/);await expect(page.getByRole('button',{name:'设置',exact:true})).toBeVisible();
 await expect(page.locator('.home-header')).toHaveCSS('height','72px');
 await page.getByRole('button',{name:'设置',exact:true}).click();await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'退出',exact:true}).click();await expect(page).toHaveURL(/\/app$/);expect(logouts).toBe(0);
 await page.getByRole('button',{name:'账号',exact:true}).click();await expect(page.getByText(user.account,{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'退出登录',exact:true}).click();expect(logouts).toBe(1);await expect(page).toHaveURL(/\/login$/);
});
test('R2 AC02 ordinary member Home has own identity/empty app state; direct admin denies',async({page})=>{
 await fixture(page,false);await page.goto('/app');await expect(page.getByText('暂无可用应用',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'设置',exact:true})).toBeHidden();
 await page.goto('/app/admin');await expect(page.getByRole('alert')).toContainText('没有人员管理权限');await expect(page.getByRole('tab',{name:'成员与部门',exact:true})).toBeHidden();
});
test('R3 four independent shell layouts retain selected tab and unsaved input; icons aligned',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('尚未保存');
 for(const [side,top] of [[320,72],[72,72],[72,40],[320,40],[320,72]]){
  if(side===72 && (await page.locator('.admin-sidebar').boundingBox())!.width===320)await page.getByRole('button',{name:'收起侧栏'}).click();
  if(side===320 && (await page.locator('.admin-sidebar').boundingBox())!.width===72)await page.getByRole('button',{name:'展开侧栏'}).click();
  if(top===40 && (await page.locator('.admin-header').boundingBox())!.height===72)await page.getByRole('button',{name:'收起顶栏'}).click();
  if(top===72 && (await page.locator('.admin-header').boundingBox())!.height===40)await page.getByRole('button',{name:'展开顶栏'}).click();
  await expect(page.locator('.admin-sidebar')).toHaveCSS('width',side+'px');await expect(page.locator('.admin-header')).toHaveCSS('height',top+'px');
  await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('尚未保存');await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');
  if(side===72){for(const sel of ['.admin-settings-icon','.personnel-nav-icon']){const b=await page.locator(sel).boundingBox();expect(b!.x+b!.width/2).toBeCloseTo(36,0);}}
 }
});
test('R3 identity separates direct/template sources and confirms impact before versioned save',async({page})=>{
 await admin(page);const sent:unknown[]=[];
 await page.route('**/api/v1/personnel/identities/'+identity.id,async route=>{sent.push(route.request().postDataJSON());await route.fulfill({status:200,json:{code:'OK',message:'success',data:{...identity,name:'新名称',version:2},meta:null}});});
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
 await page.route('**/api/v1/personnel/members/'+user.id+'/identities',async route=>{requests.push(route.request().postDataJSON());await route.fulfill({status:200,json:{code:'OK',message:'success',data:{...member,version:1,identityIds:[identity.id],identities:[identity]},meta:null}});});
 await page.getByRole('button',{name:'配置身份',exact:true}).click();await expect(page.getByRole('dialog')).toContainText(user.account);await page.getByLabel('身份：企业管理员',{exact:true}).check();await page.getByRole('button',{name:'确认分配',exact:true}).click();await expect.poll(()=>requests).toEqual([{identityIds:[identity.id],version:0}]);
});
test('R3 departments create uses selected parent; enterprise root is protected',async({page})=>{
 await admin(page);await page.getByRole('button',{name:'新建部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('研发');let input:unknown;
 await page.route('**/api/v1/personnel/departments',async route=>{if(route.request().method()==='POST'){input=route.request().postDataJSON();await route.fulfill({status:201,json:{code:'OK',message:'success',data:{...department,id:'00000000-0000-4000-8000-000000000005',name:'研发',isRoot:false,parentId:department.id},meta:null}});}else await route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[department]},meta:null}});});
 await page.getByRole('button',{name:'确认创建',exact:true}).click();await expect.poll(()=>input).toEqual({name:'研发',parentId:department.id});
});
test('R3 invitation is deliberate one-time result; activity has true empty state',async({page})=>{
 await admin(page);let created=0;await page.route('**/api/v1/invitations',async route=>{created++;await route.fulfill({status:201,json:{code:'OK',message:'success',data:{id:'00000000-0000-4000-8000-000000000006',invitationCode:'synthetic-component-fixture'},meta:null}});});
 await page.getByRole('button',{name:'邀请成员',exact:true}).click();expect(created).toBe(0);await page.getByRole('button',{name:'生成邀请码',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).toHaveValue('synthetic-component-fixture');expect(created).toBe(1);await page.getByRole('button',{name:'关闭',exact:true}).click();await page.getByRole('tab',{name:'操作记录',exact:true}).click();await expect(page.getByText('暂无操作记录',{exact:true})).toBeVisible();await expect(page.getByText('周涵',{exact:true})).toBeHidden();
});
test('R3 reduced motion and compact layouts retain keyboard-reachable navigation',async({page})=>{
 await page.emulateMedia({reducedMotion:'reduce'});await page.setViewportSize({width:390,height:844});await admin(page);await expect(page.locator('.admin-sidebar')).toHaveCSS('transition-duration','0s');await page.getByRole('tab',{name:'身份',exact:true}).focus();await page.keyboard.press('Enter');await expect(page.getByRole('tab',{name:'身份',exact:true})).toHaveAttribute('aria-selected','true');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('R3 browser back with dirty configuration prompts, cancel preserves inputs',async({page})=>{
 await fixture(page);await page.goto('/app');await page.getByRole('button',{name:'设置',exact:true}).click();
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'企业管理员',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('浏览器返回保护');
 await page.goBack();await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByLabel('身份名称',{exact:true})).toHaveValue('浏览器返回保护');
 await page.goBack();await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page).toHaveURL(/\/app$/);
});
test('R3 definition creation has no fake seed; validates and sends explicit config',async({page})=>{
 await admin(page);let created:unknown;await page.route('**/api/v1/personnel/identities?**',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:list([]),meta:null}}));
 await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:'新建身份',exact:true}).click();await page.getByLabel('身份名称',{exact:true}).fill('普通员工');
 await page.route('**/api/v1/personnel/identities',async route=>{created=route.request().postDataJSON();await route.fulfill({status:201,json:{code:'OK',message:'success',data:{...identity,name:'普通员工',description:'',templateIds:[],permissionCodes:[]},meta:null}});});
 await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();await expect.poll(()=>created).toEqual({name:'普通员工',description:'',templateIds:[],permissionCodes:[]});
});
test('R3 member explicit move preserves other groups and identity configuration',async({page})=>{
 await admin(page);const other={...department,id:'00000000-0000-4000-8000-000000000010',parentId:department.id,name:'研发部',isRoot:false};
 await page.route('**/api/v1/personnel/departments',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[department,other]},meta:null}}));
 await page.route('**/api/v1/personnel/members**',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:list([{...member,departmentIds:[department.id],departments:[department]}]),meta:null}}));
 await page.reload();await page.getByRole('button',{name:'调整分组',exact:true}).click();await page.getByLabel('分组操作',{exact:true}).selectOption('move');await page.getByLabel('来源部门',{exact:true}).selectOption(department.id);await page.getByLabel('目标部门',{exact:true}).selectOption(other.id);
 const sent:unknown[]=[];await page.route('**/api/v1/personnel/members/'+user.id+'/groups',async route=>{sent.push(route.request().postDataJSON());await route.fulfill({status:200,json:{code:'OK',message:'success',data:{...member,version:1},meta:null}});});
 await page.getByRole('button',{name:'确认调整',exact:true}).click();await expect.poll(()=>sent).toEqual([{operation:'move',departmentId:other.id,sourceDepartmentId:department.id,version:0}]);
});
test('R3 member search and pagination send approved filters, no local page-only filtering',async({page})=>{
 await admin(page);const queries:string[]=[];await page.route('**/api/v1/personnel/members?**',async route=>{queries.push(new URL(route.request().url()).search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:{...list([member]),total:21},meta:null}});});
 await page.getByLabel('搜索成员',{exact:true}).fill('目标');await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('search')==='目标')).toBe(true);
 await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
});
test('R3 activity search and action filter are server-side, not cosmetic controls',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/events?**',async route=>{queries.push(new URL(route.request().url()).search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:list([]),meta:null}});});
 await page.getByLabel('搜索操作记录',{exact:true}).fill('研发');await page.getByLabel('操作类型',{exact:true}).selectOption('DEPARTMENT_CREATED');await expect.poll(()=>queries.some(q=>{const p=new URLSearchParams(q);return p.get('search')==='研发'&&p.get('action')==='DEPARTMENT_CREATED';})).toBe(true);
});
test('R3 deleting unreferenced template confirms and sends version; 409 keeps selection',async({page})=>{
 await admin(page);await page.route('**/api/v1/personnel/templates?**',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:list([{...template,affectedIdentities:0,affectedMembers:0}]),meta:null}}));
 await page.reload();await page.getByRole('tab',{name:'权限模板',exact:true}).click();await page.getByRole('button',{name:'企业管理',exact:true}).click();await page.getByRole('button',{name:'删除模板',exact:true}).click();
 let query='';await page.route('**/api/v1/personnel/templates/'+template.id+'?**',async route=>{query=new URL(route.request().url()).search;await route.fulfill({status:409,json:{code:'PERSONNEL_CONFLICT',message:'conflict',data:null,meta:null}});});
 await page.getByRole('button',{name:'确认删除',exact:true}).click();expect(new URLSearchParams(query).get('version')).toBe('1');await expect(page.getByRole('alert')).toContainText('配置已变更');await expect(page.getByLabel('模板名称',{exact:true})).toHaveValue('企业管理');
});


test('R3 creates a permission template with explicit empty configuration',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'权限模板',exact:true}).click();
 await expect(page.getByRole('button',{name:'新建权限模板',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'新建权限模板',exact:true}).click();await page.getByLabel('模板名称',{exact:true}).fill('空模板');
 let body:unknown;await page.route('**/api/v1/personnel/templates',async route=>{body=route.request().postDataJSON();await route.fulfill({status:201,json:{code:'OK',message:'success',data:{...template,id:'00000000-0000-4000-8000-000000000012',name:'空模板',permissionCodes:[],affectedIdentities:0,affectedMembers:0},meta:null}});});
 await page.getByRole('button',{name:'保存',exact:true}).click();await page.getByRole('button',{name:'确认保存',exact:true}).click();
 await expect.poll(()=>body).toEqual({name:'空模板',description:'',permissionCodes:[]});
});
test('R3 identity deletion protects references and explicitly confirms an unreferenced object',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();
 await expect(page.getByRole('button',{name:'删除身份',exact:true})).toBeDisabled();
 await page.route('**/api/v1/personnel/identities?**',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:list([{...identity,affectedMembers:0}]),meta:null}}));await page.reload();await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();
 let version='';await page.route('**/api/v1/personnel/identities/'+identity.id+'?**',async route=>{version=new URL(route.request().url()).searchParams.get('version')||'';await route.fulfill({status:204});});
 await page.getByRole('button',{name:'删除身份',exact:true}).click();await page.getByRole('button',{name:'确认删除',exact:true}).click();await expect.poll(()=>version).toBe('1');await expect(page.getByText('请选择身份查看配置',{exact:true})).toBeVisible();
});
test('R3 department selected node filters members; rename and delete use actual version; root protected',async({page})=>{
 await fixture(page);const d={...department,id:'00000000-0000-4000-8000-000000000010',name:'研发部',parentId:department.id,isRoot:false,memberCount:0,childrenCount:0};
 await page.route('**/api/v1/personnel/departments',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[department,d]},meta:null}}));await page.goto('/app/admin');
 await page.getByRole('button',{name:/^企业/}).click();await expect(page.getByRole('button',{name:'删除部门',exact:true})).toBeDisabled();
 const queries:string[]=[];await page.route('**/api/v1/personnel/members?**',async route=>{queries.push(new URL(route.request().url()).search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:list([member]),meta:null}});});
 await page.getByRole('button',{name:/^研发部/}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('departmentId')===d.id)).toBe(true);
 await page.getByRole('button',{name:'重命名部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('研发新名');
 let renamed:unknown;await page.route('**/api/v1/personnel/departments/'+d.id,async route=>{renamed=route.request().postDataJSON();await route.fulfill({status:200,json:{code:'OK',message:'success',data:{...d,name:'研发新名',version:2},meta:null}});});
 await page.getByRole('button',{name:'确认重命名',exact:true}).click();await expect.poll(()=>renamed).toEqual({name:'研发新名',version:1});
});
test('R3 complete assignment choices load beyond the first 100 templates',async({page})=>{
 await fixture(page);const choices=Array.from({length:100},(_,i)=>({...template,id:'00000000-0000-4000-8000-'+String(200+i).padStart(12,'0'),name:'模板'+i,affectedIdentities:0}));
 const last={...template,id:'00000000-0000-4000-8000-000000000399',name:'第101个模板',affectedIdentities:0};
 await page.route('**/api/v1/personnel/templates?**',async route=>{const pageNo=Number(new URL(route.request().url()).searchParams.get('page')||1);await route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:pageNo===2?[last]:choices,total:101,page:pageNo,pageSize:100},meta:null}});});
 await page.goto('/app/admin');await page.getByRole('tab',{name:'身份',exact:true}).click();await page.getByRole('button',{name:identity.name,exact:true}).click();
 await expect(page.getByRole('checkbox',{name:'模板：第101个模板',exact:true})).toBeVisible();
});
test('R3 identity search and next page query the backend while keeping explicit selection',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'身份',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/identities?**',async route=>{const url=new URL(route.request().url());queries.push(url.search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[identity],total:21,page:Number(url.searchParams.get('page')||1),pageSize:20},meta:null}});});
 await page.getByLabel('搜索身份',{exact:true}).fill('共享');await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('search')==='共享')).toBe(true);
 await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
});
test('R3 member identity filter and activity pagination are server controls',async({page})=>{
 await admin(page);const memberQueries:string[]=[];await page.route('**/api/v1/personnel/members?**',async route=>{memberQueries.push(new URL(route.request().url()).search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:list([member]),meta:null}});});
 await expect(page.getByLabel('筛选身份',{exact:true})).toBeVisible();await page.getByLabel('筛选身份',{exact:true}).selectOption(identity.id);await expect.poll(()=>memberQueries.some(q=>new URLSearchParams(q).get('identityId')===identity.id)).toBe(true);
 await page.getByRole('tab',{name:'操作记录',exact:true}).click();const eventQueries:string[]=[];await page.route('**/api/v1/personnel/events?**',async route=>{const url=new URL(route.request().url());eventQueries.push(url.search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[],total:21,page:Number(url.searchParams.get('page')||1),pageSize:20},meta:null}});});
 await page.getByLabel('搜索操作记录',{exact:true}).fill('活动');await expect(page.getByRole('button',{name:'下一页',exact:true})).toBeEnabled();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>eventQueries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
});
test('R3 invitation uses the existing invitationCode response and clears it on close',async({page})=>{
 await admin(page);await page.route('**/api/v1/invitations',route=>route.fulfill({status:201,json:{code:'OK',message:'success',data:{id:'00000000-0000-4000-8000-000000000006',invitationCode:'synthetic-existing-contract'},meta:null}}));
 await page.getByRole('button',{name:'邀请成员',exact:true}).click();await page.getByRole('button',{name:'生成邀请码',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).toHaveValue('synthetic-existing-contract');
 await page.getByRole('button',{name:'关闭',exact:true}).click();await page.getByRole('button',{name:'邀请成员',exact:true}).click();await expect(page.getByLabel('邀请码',{exact:true})).toBeHidden();await expect(page.getByRole('button',{name:'生成邀请码',exact:true})).toBeVisible();
});

test('R3 activity pagination and explicit time range reach the server independently',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/events?**',async route=>{const url=new URL(route.request().url());queries.push(url.search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[],total:21,page:Number(url.searchParams.get('page')||1),pageSize:20},meta:null}});});
 await page.getByLabel('搜索操作记录',{exact:true}).fill('分页');await expect(page.getByRole('button',{name:'下一页',exact:true})).toBeEnabled();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect.poll(()=>queries.some(q=>new URLSearchParams(q).get('page')==='2')).toBe(true);
 await page.getByText('最近 7 天',{exact:true}).click();await page.getByLabel('开始时间',{exact:true}).fill('2026-09-01T00:00');await page.getByLabel('结束时间',{exact:true}).fill('2026-09-02T00:00');await page.getByRole('button',{name:'应用时间范围',exact:true}).click();
 await expect.poll(()=>queries.some(q=>{const p=new URLSearchParams(q);return p.get('from')===new Date('2026-09-01T00:00').toISOString()&&p.get('to')===new Date('2026-09-02T00:00').toISOString()&&p.get('page')==='1';})).toBe(true);
});
test('R3 explicit activity time filter sends RFC3339 and resets the page',async({page})=>{
 await admin(page);await page.getByRole('tab',{name:'操作记录',exact:true}).click();const queries:string[]=[];
 await page.route('**/api/v1/personnel/events?**',async route=>{queries.push(new URL(route.request().url()).search);await route.fulfill({status:200,json:{code:'OK',message:'success',data:list([]),meta:null}});});
 await page.getByText('最近 7 天',{exact:true}).click();await page.getByLabel('开始时间',{exact:true}).fill('2026-09-01T00:00');await page.getByLabel('结束时间',{exact:true}).fill('2026-09-02T00:00');await page.getByRole('button',{name:'应用时间范围',exact:true}).click();
 await expect.poll(()=>queries.some(q=>{const p=new URLSearchParams(q);return p.get('from')===new Date('2026-09-01T00:00').toISOString()&&p.get('to')===new Date('2026-09-02T00:00').toISOString();})).toBe(true);
});
test('R3 empty department deletion confirms version and preserves conflict feedback',async({page})=>{
 await fixture(page);const d={...department,id:'00000000-0000-4000-8000-000000000010',name:'空部门',parentId:department.id,isRoot:false};
 await page.route('**/api/v1/personnel/departments',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:{items:[department,d]},meta:null}}));await page.goto('/app/admin');await page.getByRole('button',{name:/^空部门/}).click();
 let version='';await page.route('**/api/v1/personnel/departments/'+d.id+'?**',async route=>{version=new URL(route.request().url()).searchParams.get('version')||'';await route.fulfill({status:409,json:{code:'PERSONNEL_CONFLICT',message:'conflict',data:null,meta:null}});});
 await page.getByRole('button',{name:'删除部门',exact:true}).click();await page.getByRole('button',{name:'确认删除',exact:true}).click();await expect.poll(()=>version).toBe('1');await expect(page.getByRole('alert')).toContainText('配置已变更');
});
for(const mode of ['department','member','groups'] as const)test('R3 unsaved '+mode+' form survives close and Escape until explicit discard',async({page})=>{
 await admin(page);
 if(mode==='department'){await page.getByRole('button',{name:'新建部门',exact:true}).click();await page.getByLabel('部门名称',{exact:true}).fill('保留部门');}
 else if(mode==='member'){await page.getByRole('button',{name:'配置身份',exact:true}).click();await page.getByLabel('身份：企业管理员',{exact:true}).check();}
 else{await page.getByRole('button',{name:'调整分组',exact:true}).click();await page.getByLabel('目标部门',{exact:true}).selectOption(department.id);}
 await page.getByRole('button',{name:'关闭',exact:true}).click();await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'继续编辑',exact:true}).click();
 if(mode==='department')await expect(page.getByLabel('部门名称',{exact:true})).toHaveValue('保留部门');else if(mode==='member')await expect(page.getByLabel('身份：企业管理员',{exact:true})).toBeChecked();else await expect(page.getByLabel('目标部门',{exact:true})).toHaveValue(department.id);
 await page.keyboard.press('Escape');await expect(page.getByRole('dialog')).toContainText('未保存');await page.getByRole('button',{name:'放弃修改',exact:true}).click();await expect(page.getByRole('dialog')).toBeHidden();
});

test('Figma shell retains each original L material asset and original SVG dimensions',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);
 for(const state of ['ee','ec','cc','ce'] as const){
  const material=page.locator('.admin-material');await expect(material).toHaveAttribute('src','data:image/svg+xml;base64,'+readFileSync(new URL('./assets/admin-background-'+state+'.svg',import.meta.url)).toString('base64'));
  await expect(material).toHaveCSS('top','-22px');await expect(material).toHaveCSS('left','-22px');
  await expect(page.locator('.admin-sidebar')).toHaveCSS('width',state[1]==='e'?'320px':'72px');await expect(page.locator('.admin-header')).toHaveCSS('height',state[0]==='e'?'72px':'40px');
  const dimensions=await material.evaluate((node:HTMLImageElement)=>({width:node.naturalWidth,height:node.naturalHeight}));expect(dimensions).toEqual({width:1980,height:1140});
  await page.screenshot({path:'../../docs/evidence/V010-019/visual-admin-'+state+'.png'});
  if(state==='ee')await page.getByRole('button',{name:'收起侧栏',exact:true}).click();else if(state==='ec')await page.getByRole('button',{name:'收起顶栏',exact:true}).click();else if(state==='cc')await page.getByRole('button',{name:'展开侧栏',exact:true}).click();
 }
});

test('Q25 default activity range omits optional empty timestamps in every initial request',async({page})=>{
 await fixture(page);const queries:URLSearchParams[]=[];
 await page.route('**/api/v1/personnel/events?**',async route=>{queries.push(new URL(route.request().url()).searchParams);await route.fulfill({status:200,json:{code:'OK',message:'success',data:list([]),meta:null}});});
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
test('Figma shell original translucent material has no extra white foreground layers',async({page})=>{
 await admin(page);await expect(page.locator('.admin-header')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');await expect(page.locator('.admin-sidebar')).toHaveCSS('background-color','rgba(0, 0, 0, 0)');
});
test('R3 activity presents readable action, target and safe changes rather than raw protocol JSON',async({page})=>{
 await fixture(page);const event={id:'00000000-0000-4000-8000-000000000020',occurredAt:'2026-09-30T00:00:00Z',actorAccount:'synthetic-operator',action:'DEPARTMENT_UPDATED',objectType:'department',objectId:department.id,summary:{before:{name:'研发部',parentId:department.id},after:{name:'研发中心',parentId:department.id}},outcome:'success'};
 await page.route('**/api/v1/personnel/events?**',route=>route.fulfill({status:200,json:{code:'OK',message:'success',data:list([event]),meta:null}}));await page.goto('/app/admin');await page.getByRole('tab',{name:'操作记录',exact:true}).click();
 const row=page.getByRole('row').filter({hasText:'synthetic-operator'});await expect(row).toContainText('重命名部门');await expect(row).toContainText('研发中心');await expect(row).toContainText('研发部 → 研发中心');await expect(row).not.toContainText('DEPARTMENT_UPDATED');await expect(row).not.toContainText('parentId');
 await expect(page.getByRole('option',{name:'分配身份',exact:true})).toHaveAttribute('value','MEMBER_IDENTITIES_UPDATED');
});
test('Figma each shell variant retains its own material offset and collapsed 48px menu target',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);
 await page.getByRole('button',{name:'收起侧栏',exact:true}).click();await expect(page.locator('.admin-material')).toHaveCSS('left','-30px');
 const menu=page.getByRole('button',{name:'人员管理',exact:true});await expect(menu).toHaveCSS('width','48px');await expect(menu).toHaveCSS('height','48px');expect((await menu.boundingBox())!.y).toBe(144);
 await page.getByRole('button',{name:'收起顶栏',exact:true}).click();await expect(page.locator('.admin-material')).toHaveCSS('left','-22px');expect((await menu.boundingBox())!.y).toBe(112);
 await page.getByRole('button',{name:'展开侧栏',exact:true}).click();await expect(page.locator('.admin-material')).toHaveCSS('top','-30px');
});
test('Figma member table keeps fixed column geometry and page selection never sends a write',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);const th=page.locator('.member-table th');
 for(const [index,width] of [[0,52],[2,176],[3,216],[4,136],[5,144]])expect((await th.nth(index).boundingBox())!.width).toBeCloseTo(width,0);
 let writes=0;page.on('request',request=>{if(request.url().includes('/api/v1/')&&!['GET','HEAD'].includes(request.method()))writes++;});
 const select=page.getByLabel('选择成员：'+user.account,{exact:true});await select.check();await expect(page.getByLabel('选择当前页成员',{exact:true})).toBeChecked();await page.getByLabel('选择当前页成员',{exact:true}).uncheck();await expect(select).not.toBeChecked();expect(writes).toBe(0);
});
test('Figma collapsed menu independently has a 48px target at the original sidebar inset',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('button',{name:'收起侧栏',exact:true}).click();const menu=page.getByRole('button',{name:'人员管理',exact:true});await expect(menu).toHaveCSS('width','48px');await expect(menu).toHaveCSS('height','48px');expect((await menu.boundingBox())!.y).toBe(144);
});
test('Figma top-collapsed side-expanded material independently uses the original minus30 top',async({page})=>{
 await page.setViewportSize({width:1920,height:1080});await admin(page);await page.getByRole('button',{name:'收起顶栏',exact:true}).click();await expect(page.locator('.admin-material')).toHaveCSS('top','-30px');
});
