import {test,expect,type Page,type Locator} from '@playwright/test';

const memberId='00000000-0000-4000-8000-000000000001';
const presetA='00000000-0000-4000-8000-000000000011';
const presetB='00000000-0000-4000-8000-000000000012';
const user={id:memberId,account:'preset-admin'};
const member={...user,status:'active',bootstrapAdmin:true,version:1,departmentIds:[],identityIds:[],departments:[],identities:[],permissions:[]};
const eq={operator:'and',children:[{field:'account',operator:'eq',value:'preset-admin'}]};
type Preset={id:string;view:string;name:string;filter:unknown;hiddenColumnIds:string[];schemaVersion:number;version:number;createdAt:string;updatedAt:string};
const saved=(id=presetA,name='方案甲',hiddenColumnIds:string[]=['departments'],filter:unknown=eq):Preset=>({id,view:'members',name,filter,hiddenColumnIds,schemaVersion:1,version:1,createdAt:'2026-10-02T03:00:00Z',updatedAt:'2026-10-02T03:00:00Z'});
const envelope=(data:unknown)=>({code:'OK',message:'success',data,meta:null});
async function fixture(page:Page,initial:Preset[]=[]){
 const state={presets:structuredClone(initial),queries:[] as Record<string,unknown>[],writes:[] as {method:string;body:Record<string,unknown>}[],conflict:'',queryError:'',reads:[] as string[]};
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(),url=new URL(req.url()),path=url.pathname.replace('/api/v1/',''),method=req.method();
  if(path.startsWith('personnel/table-presets')){
   state.reads.push(path+url.search);
   if(method==='GET')return route.fulfill({json:envelope(path==='personnel/table-presets'?{items:state.presets.filter(p=>p.view===url.searchParams.get('view'))}:state.presets.find(p=>path.endsWith(p.id)))});
   const body=method==='DELETE'?{}:req.postDataJSON();state.writes.push({method,body});
   if(state.conflict)return route.fulfill({status:409,json:{code:state.conflict,message:'conflict',data:null,meta:null}});
   if(method==='DELETE'){state.presets=state.presets.filter(p=>!path.endsWith(p.id));return route.fulfill({status:204});}
   const prior=state.presets.find(p=>path.endsWith(p.id));
   const next={...saved(prior?.id??presetB),...body,view:prior?.view??body.view,version:prior?prior.version+1:1};
   state.presets=state.presets.filter(p=>p.id!==next.id);state.presets.unshift(next);return route.fulfill({status:method==='POST'?201:200,json:envelope(next)});
  }
  if(path.endsWith('/search')){
   const input=req.postDataJSON();state.queries.push(input);
   if(state.queryError&&input.filter)return route.fulfill({status:409,json:{code:state.queryError,message:'conflict',data:null,meta:null}});
   return route.fulfill({json:envelope({items:path.includes('members')?[member]:[],total:40,page:input.page,pageSize:input.pageSize,queryVersion:'cloud-context',sort:null,...(path.includes('events')?{range:{from:'2026-09-24T00:00:00Z',to:'2026-10-02T00:00:00Z'}}:{})})});
  }
  const data=path==='sessions/current'?user:path==='me/access'?{user,bootstrapAdmin:true,personnelManage:true,identities:[],permissions:[],applications:[]}:
   path==='personnel/departments'||path==='personnel/permissions'||path==='personnel/drafts'?{items:[]}:
   path==='personnel/identities'||path==='personnel/templates'?{items:[],total:0,page:1,pageSize:20}:{};
  return route.fulfill({json:envelope(data)});
 });
 await page.goto('/app/admin');await expect(page.locator('.member-table table')).toHaveAttribute('aria-busy','false');return state;
}
async function manager(page:Page){await page.getByRole('button',{name:/^自定义筛选/}).click();const dialog=page.getByRole('dialog',{name:'管理自定义筛选',exact:true});await expect(dialog).toBeVisible();return dialog;}
async function editor(page:Page,panel:Locator){await panel.getByRole('button',{name:'新增筛选',exact:true}).click();const dialog=page.getByRole('dialog',{name:'新增自定义筛选',exact:true});await expect(dialog).toBeVisible();return dialog;}
async function addRow(dialog:Locator,value='preset-admin'){await dialog.getByRole('button',{name:'或条件',exact:true}).click();await dialog.getByLabel('条件 1.1 值',{exact:true}).fill(value);}

test('approved empty manager opens at viewport center with explicit new action',async({page})=>{
 await fixture(page);const panel=await manager(page);await expect(panel.getByRole('button',{name:'新增筛选',exact:true})).toBeVisible();await expect(panel.getByLabel('自定义筛选名称')).toHaveCount(0);
 const box=(await panel.boundingBox())!;expect(Math.abs(box.x+box.width/2-720)).toBeLessThan(2);expect(Math.abs(box.y+box.height/2-500)).toBeLessThan(2);expect(box.width).toBe(440);
});
test('save finite OR of AND blocks and typed comparison without applying',async({page})=>{
 const state=await fixture(page);const d=await editor(page,await manager(page));await d.getByLabel('自定义筛选名称',{exact:true}).fill('  团队筛选  ');await addRow(d);
 await d.getByRole('button',{name:'组 1 且条件',exact:true}).click();await d.getByLabel('条件 1.2 字段',{exact:true}).selectOption('status');await d.getByLabel('条件 1.2 值',{exact:true}).selectOption('active');
 await d.getByRole('button',{name:'或条件',exact:true}).click();await d.getByLabel('条件 2.1 值',{exact:true}).fill('另一个成员');await d.getByLabel('条件 2.1 比较',{exact:true}).selectOption('neq');
 await d.getByRole('button',{name:'确定',exact:true}).click();await expect.poll(()=>state.writes.length).toBe(1);
 expect(state.writes[0].body).toMatchObject({name:'团队筛选',filter:{operator:'or',children:[{operator:'and',children:[{field:'account',operator:'eq',value:'preset-admin'},{field:'status',operator:'eq',value:'active'}]},{operator:'and',children:[{field:'account',operator:'neq',value:'另一个成员'}]}]},hiddenColumnIds:[],schemaVersion:1,view:'members'});
 expect(state.queries.every(q=>!q.filter)).toBe(true);await expect(page.getByRole('dialog',{name:'管理自定义筛选',exact:true})).toBeVisible();
});
test('visibility-only save has independent searches and keeps at least one business field',async({page})=>{
 const state=await fixture(page);const d=await editor(page,await manager(page));await d.getByLabel('自定义筛选名称',{exact:true}).fill('显隐配置');
 await d.getByLabel('显示字段搜索',{exact:true}).fill('部门');await expect(d.getByRole('button',{name:'隐藏部门',exact:true})).toBeVisible();await expect(d.getByRole('button',{name:'隐藏身份',exact:true})).toHaveCount(0);await d.getByRole('button',{name:'隐藏部门',exact:true}).click();
 await d.getByLabel('隐藏字段搜索',{exact:true}).fill('身份');await expect(d.getByRole('region',{name:'隐藏字段',exact:true})).toContainText('暂无字段');await d.getByLabel('隐藏字段搜索',{exact:true}).fill('');await expect(d.getByRole('button',{name:'显示部门',exact:true})).toBeVisible();
 await d.getByLabel('显示字段搜索',{exact:true}).fill('');for(const name of ['成员','身份'])await d.getByRole('button',{name:'隐藏'+name,exact:true}).click();await expect(d.getByRole('button',{name:'隐藏人员管理',exact:true})).toBeDisabled();
 await expect(d.getByRole('button',{name:'隐藏操作',exact:true})).toHaveCount(0);await d.getByRole('button',{name:'确定',exact:true}).click();await expect.poll(()=>state.writes.length).toBe(1);expect(state.writes[0].body.filter).toBeNull();expect(state.queries.every(q=>q.hiddenColumnIds===undefined)).toBe(true);
});
test('apply latest resource resets page selection while preserving search quick filters and queryVersion',async({page})=>{
 const state=await fixture(page,[saved()]);await page.getByLabel('搜索成员',{exact:true}).fill('preset');await expect.poll(()=>state.queries.at(-1)?.search).toBe('preset');await page.getByLabel('跳至页',{exact:true}).fill('2');await page.getByLabel('跳至页',{exact:true}).press('Enter');await expect.poll(()=>state.queries.at(-1)?.page).toBe(2);await page.getByLabel('选择成员：preset-admin',{exact:true}).check();
 const panel=await manager(page);await panel.getByRole('button',{name:'应用方案甲',exact:true}).click();await expect.poll(()=>state.queries.at(-1)?.filter).toEqual(eq);expect(state.queries.at(-1)).toMatchObject({page:1,search:'preset',queryVersion:'cloud-context'});expect(state.reads).toContain('personnel/table-presets/'+presetA);
 await expect(page.getByLabel('选择成员：preset-admin',{exact:true})).not.toBeChecked();await expect(page.locator('.member-table thead')).not.toContainText('部门');await expect(page.locator('.member-table thead')).toContainText('操作');
 await expect(panel.getByText('已应用',{exact:true})).toHaveCount(1);
});
test('editing applied preset saves new version but preserves applied snapshot until explicit apply',async({page})=>{
 const state=await fixture(page,[saved()]);const panel=await manager(page);await panel.getByRole('button',{name:'应用方案甲',exact:true}).click();await expect.poll(()=>state.queries.at(-1)?.filter).toEqual(eq);const before=state.queries.length;
 await panel.getByRole('button',{name:'编辑方案甲',exact:true}).click();const d=page.getByRole('dialog',{name:'编辑自定义筛选',exact:true});await d.getByLabel('条件 1.1 值',{exact:true}).fill('另一个成员');await d.getByRole('button',{name:'显示部门',exact:true}).click();await d.getByRole('button',{name:'确定',exact:true}).click();await expect(panel).toContainText('已修改，待应用');expect(state.queries.length).toBe(before);await expect(page.locator('.member-table thead')).not.toContainText('部门');
});
test('switch A to B then cancel restores original visibility and does not restore A baseline',async({page})=>{
 const state=await fixture(page,[saved(),saved(presetB,'方案乙',['identities'])]);const panel=await manager(page);await panel.getByRole('button',{name:'应用方案甲',exact:true}).click();await expect(page.locator('.member-table thead')).not.toContainText('部门');await panel.getByRole('button',{name:'应用方案乙',exact:true}).click();await expect(page.locator('.member-table thead')).not.toContainText('身份');await expect(page.locator('.member-table thead')).toContainText('部门');await expect(panel.getByText('已应用',{exact:true})).toHaveCount(1);
 await panel.getByRole('button',{name:'取消应用',exact:true}).click();const confirm=page.getByRole('dialog',{name:'取消当前筛选',exact:true});await expect(confirm).toBeVisible();await confirm.getByRole('button',{name:'确认取消',exact:true}).click();await expect(page.locator('.member-table thead')).toContainText('部门');await expect(page.locator('.member-table thead')).toContainText('身份');await expect.poll(()=>state.queries.at(-1)?.filter).toBeUndefined();
});
test('delete active requires confirmation then clears filter and restores columns',async({page})=>{
 const state=await fixture(page,[saved()]);const panel=await manager(page);await panel.getByRole('button',{name:'应用方案甲',exact:true}).click();await expect(page.locator('.member-table thead')).not.toContainText('部门');await panel.getByRole('button',{name:'删除方案甲',exact:true}).click();const d=page.getByRole('dialog',{name:'删除筛选方案',exact:true});await expect(d).toContainText('取消当前应用');await d.getByRole('button',{name:'确认删除',exact:true}).click();await expect.poll(()=>state.presets.length).toBe(0);await expect(page.locator('.member-table thead')).toContainText('部门');
});
for(const code of ['PERSONNEL_PRESET_NAME_CONFLICT','PERSONNEL_PRESET_LIMIT_REACHED','PERSONNEL_PRESET_CONFLICT'])test(code+' preserves editor input with specific recovery',async({page})=>{
 const state=await fixture(page);state.conflict=code;const d=await editor(page,await manager(page));await d.getByLabel('自定义筛选名称',{exact:true}).fill('保留我');await d.getByRole('button',{name:'确定',exact:true}).click();await expect(d.getByRole('alert')).toContainText(code==='PERSONNEL_PRESET_NAME_CONFLICT'?'名称已存在':code==='PERSONNEL_PRESET_LIMIT_REACHED'?'20':'其他标签页');await expect(d.getByLabel('自定义筛选名称',{exact:true})).toHaveValue('保留我');
});
for(const code of ['COMMON_QUERY_CHANGED','COMMON_QUERY_CONTEXT_EXPIRED'])test('failed '+code+' apply cannot change active filter visibility or refresh automatically',async({page})=>{
 const state=await fixture(page,[saved()]);state.queryError=code;const panel=await manager(page);await panel.getByRole('button',{name:'应用方案甲',exact:true}).click();await expect(panel.getByRole('alert')).toContainText(code==='COMMON_QUERY_CHANGED'?'查询结果已变化':'查询上下文已过期');await expect(panel.getByText('已应用',{exact:true})).toHaveCount(0);await expect(page.locator('.member-table thead')).toContainText('部门');expect(state.queries.at(-1)?.queryVersion).toBe('cloud-context');await expect(page.getByRole('button',{name:'刷新查询',exact:true})).toBeVisible();
});
test('reopening and reloading do not autoapply persisted presets',async({page})=>{
 const state=await fixture(page,[saved()]);const p=await manager(page);await p.getByRole('button',{name:'关闭筛选管理',exact:true}).click();await expect(p).toHaveCount(0);await manager(page);expect(state.queries.every(q=>!q.filter)).toBe(true);await page.reload();await expect(page.locator('.member-table thead')).toContainText('部门');expect(state.queries.every(q=>!q.filter)).toBe(true);
});
test('invalid empty intermediate row stays editable and cannot be saved',async({page})=>{
 const state=await fixture(page);const d=await editor(page,await manager(page));await d.getByLabel('自定义筛选名称',{exact:true}).fill('未完成条件');await d.getByRole('button',{name:'或条件',exact:true}).click();await d.getByLabel('条件 1.1 字段',{exact:true}).selectOption('identityIds');await d.getByRole('button',{name:'确定',exact:true}).click();await expect(d.getByRole('alert')).toContainText('1.1');expect(state.writes).toHaveLength(0);await d.getByRole('button',{name:'删除条件 1.1',exact:true}).click();await d.getByRole('button',{name:'确定',exact:true}).click();await expect.poll(()=>state.writes.length).toBe(1);
});
test('narrow centered editor scrolls body with fixed footer and keyboard move controls',async({page})=>{
 await fixture(page);await page.setViewportSize({width:390,height:844});const d=await editor(page,await manager(page));await d.getByLabel('自定义筛选名称',{exact:true}).fill('窄屏');await addRow(d);for(let i=2;i<=20;i++){await d.getByRole('button',{name:'组 1 且条件',exact:true}).click();await d.getByLabel(`条件 1.${i} 值`,{exact:true}).fill('成员'+i);}
 const box=(await d.boundingBox())!;expect(box.x).toBeGreaterThanOrEqual(12);expect(box.x+box.width).toBeLessThanOrEqual(378);expect(box.y).toBeGreaterThanOrEqual(12);expect(box.y+box.height).toBeLessThanOrEqual(832);await expect(d.getByRole('button',{name:'确定',exact:true})).toBeInViewport();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
test('open close reopen last intent survives transition and reduced motion remains operable',async({page})=>{
 await fixture(page);const p=await manager(page);await page.evaluate(()=>{(document.querySelector('[aria-label="关闭筛选管理"]') as HTMLButtonElement).click();(document.querySelector('.q36-filter-trigger') as HTMLButtonElement).click();});await expect(p).toBeVisible();await expect.poll(()=>page.evaluate(()=>document.documentElement.classList.contains('q36-filter-transition-active'))).toBe(false);await page.keyboard.press('Escape');await expect(p).toHaveCount(0);await page.emulateMedia({reducedMotion:'reduce'});await manager(page);await page.keyboard.press('Escape');await expect(p).toHaveCount(0);await expect(page.getByRole('button',{name:/^自定义筛选/})).toBeFocused();
});
test('single Arca Table hides by stable ID and preserves hidden width/order through visible resize',async({page})=>{
 await page.goto('/src/filter-manager-table-fixture.html');const table=page.getByRole('table',{name:'显隐保序',exact:true});
 await expect(table.locator('thead th')).toHaveText(['丙','乙','甲']);await page.getByRole('button',{name:'切换乙列',exact:true}).click();await expect(table.locator('thead th')).toHaveText(['丙','甲']);
 await expect(table.locator('tbody')).not.toContainText('乙数据');
 const grip=table.locator('thead th').filter({hasText:'丙'}).locator('[data-resize-handle]');const box=(await grip.boundingBox())!;await page.mouse.move(box.x+box.width/2,box.y+box.height/2);await page.mouse.down();await page.mouse.move(box.x+box.width/2+30,box.y+box.height/2);await page.mouse.up();
 expect(JSON.parse(await page.locator('output').innerText()).widths.b).toBe(200);await page.getByRole('button',{name:'切换乙列',exact:true}).click();await expect(table.locator('thead th')).toHaveText(['丙','乙','甲']);expect(JSON.parse(await page.locator('output').innerText()).order).toEqual(['c','b','a']);
});
