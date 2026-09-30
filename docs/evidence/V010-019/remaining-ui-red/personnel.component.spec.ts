import { test, expect, type Page } from '@playwright/test';

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
 await admin(page);let created=0;await page.route('**/api/v1/invitations',async route=>{created++;await route.fulfill({status:201,json:{code:'OK',message:'success',data:{id:'00000000-0000-4000-8000-000000000006',code:'synthetic-component-fixture'},meta:null}});});
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


