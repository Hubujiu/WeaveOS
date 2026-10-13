import {test,expect,type Page} from '@playwright/test';
// V020 FLOW06, V075 P16/P17 and Figma488:6874. Only real HTTP boundary is isolated.
const id=(n:number)=>'d4d4d4d4-d4d4-4d4d-8d4d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),fieldId=id(5),base=`applications/${appId}/forms/${viewId}`,definitionPath=base+'/workflows/'+flowId+'/definition',route='/app/'+base+'/workflows/'+flowId+'/design';
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'approval',approval:{mode:'all',assigneeIds:[id(20),id(21)],editableFieldIds:[fieldId]}},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12)}]};
const triggers=[{event:'manual',condition:null},{event:'record.created',condition:{operator:'and',children:[{fieldId,operator:'eq',value:'原触发条件'}]}}];
type Options={candidateMode?:'normal'|'changed'|'denied'|'skew'|'late';write?:(body:any,n:number)=>{status:number;code:string;data?:unknown};lookupFails?:boolean;readAfterFails?:boolean};
async function setup(page:Page,options:Options={}){
 const calls:{path:string;method:string;body:any}[]=[],writes:any[]=[];let saved:any=null,identityChanged=false,refreshed=false,release:(()=>void)|undefined;
 const receipt=(operationId:string)=>({operationId,id:flowId,revision:4,currentVersion:1,candidateVersion:3,state:'enabled'});
 const app={id:appId,name:'流程保存测试',ownerUserId:actor.id,policyRevision:1};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=identityChanged?{id:id(99),account:'other'}:actor;
 else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
 else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime'){
  data={appId,viewId,tableId:id(8),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'说明',kind:'text',required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:['eq','neq'],sortable:false,quickSearchable:true}}],layout:[{id:id(9),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 }else if(path===definitionPath&&method==='GET'){
  if(saved&&options.readAfterFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  data={id:flowId,name:'部门审批',revision:saved?4:3,currentVersion:1,candidateVersion:saved?3:2,state:'enabled',schemaVersion:1,allowWithdraw:saved?saved.allowWithdraw:true,graph:saved?.graph??graph,triggers:saved?.triggers??triggers};
 }else if(path===base+'/workflows/approvers/search'){
  if(options.candidateMode==='late')await new Promise<void>(resolve=>{release=resolve;});
  if(options.candidateMode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  if(options.candidateMode==='changed'&&body.page===2){refreshed=true;return r.fulfill({status:409,json:{code:'COMMON_QUERY_CHANGED',data:null}});}
  data={items:body.page===2?[{id:id(22),label:'次页审批人',status:'active',departmentNames:[]}]:[{id:id(20),label:refreshed?'刷新后审批人':'原审批人甲',status:'active',departmentNames:[]},{id:id(21),label:'原审批人乙',status:'active',departmentNames:[]}],total:21,page:body.page,pageSize:body.pageSize,queryVersion:refreshed?'new-token':'original-token',schemaVersion:options.candidateMode==='skew'?2:1,policyRevision:1};
 }else if(path===definitionPath&&method==='PUT'){
  writes.push({body,method,path,headers:req.headers()});const reply=options.write?.(body,writes.length)??{status:200,code:'OK',data:receipt(body.operationId)};
  if(reply.status===200&&reply.code==='OK')saved=body;
  return r.fulfill({status:reply.status,json:{code:reply.code,data:reply.data??null,meta:null}});
 }else if(path.startsWith('application-operations/')){
  if(options.lookupFails)return r.fulfill({status:503,json:{code:'COMMON_SERVICE_UNAVAILABLE',data:null}});
  saved=writes[0].body;data={operationId:path.split('/').at(-1),status:'confirmed',httpStatus:200,location:null,result:receipt(path.split('/').at(-1)!)};
 }else if(path===base+'/workflows/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'manager'};
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes,receipt,release:()=>release?.(),changeIdentity:()=>{identityChanged=true;}};
}
const candidatePath=base+'/workflows/approvers/search';
async function open(page:Page){await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();await page.getByRole('button',{name:'选择审批人',exact:true}).click();return page.getByRole('dialog',{name:'选择审批人',exact:true});}
test('Root approver picker explicit open and confirmed cross-page selection only affect unsaved node IDs',async({page})=>{
 const f=await setup(page);await page.goto(route);await expect(page.getByRole('region',{name:'流程图',exact:true})).toBeVisible();expect(f.calls.filter(c=>c.path===candidatePath)).toEqual([]);
 await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();await page.getByRole('button',{name:'选择审批人',exact:true}).click();const dialog=page.getByRole('dialog',{name:'选择审批人',exact:true});
 await expect(dialog.getByRole('checkbox',{name:'原审批人甲',exact:true})).toBeChecked();await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await dialog.getByRole('button',{name:'下一页',exact:true}).click();await dialog.getByRole('checkbox',{name:'次页审批人',exact:true}).check();expect(f.writes).toEqual([]);await dialog.getByRole('button',{name:'确认',exact:true}).click();await expect(dialog).toHaveCount(0);expect(f.writes).toEqual([]);
 await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const g=structuredClone(graph);g.nodes[1].approval!.assigneeIds=[id(20),id(22)];expect(f.writes[0].body.graph).toEqual(g);expect(f.calls.filter(c=>c.path===candidatePath).map(c=>c.body)).toEqual([{page:1,pageSize:20},{page:2,pageSize:20,queryVersion:'original-token'}]);
});
test('Root approver picker cancel preserves original node and returns keyboard focus',async({page},testInfo)=>{
 const f=await setup(page);const dialog=await open(page);await expect(dialog.getByRole('checkbox',{name:'原审批人甲',exact:true})).toBeChecked();await expect.poll(async()=>{const box=await dialog.boundingBox();return box?{width:Math.round(box.width),height:Math.round(box.height)}:null;}).toEqual({width:640,height:560});await testInfo.attach('workflow-approver-picker',{body:await page.screenshot(),contentType:'image/png'});await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(dialog).toHaveCount(0);await expect(page.getByRole('button',{name:'选择审批人',exact:true})).toBeFocused();expect(f.writes).toEqual([]);await page.getByRole('button',{name:'选择审批人',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'原审批人乙',exact:true})).toBeChecked();
});
test('Root approver picker does not permit an empty approval configuration',async({page})=>{
 const f=await setup(page);const dialog=await open(page);await dialog.getByRole('checkbox',{name:'原审批人甲',exact:true}).uncheck();await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await expect(dialog.getByRole('button',{name:'确认',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
test('Root changed approver query clears candidates and explicitly refreshes first page without old token',async({page})=>{
 const f=await setup(page,{candidateMode:'changed'});const dialog=await open(page);await dialog.getByRole('button',{name:'下一页',exact:true}).click();await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('checkbox',{name:'原审批人甲',exact:true})).toHaveCount(0);await dialog.getByRole('button',{name:'刷新',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'刷新后审批人',exact:true})).toBeVisible();expect(f.calls.filter(c=>c.path===candidatePath).at(-1)!.body).toEqual({page:1,pageSize:20});expect(f.writes).toEqual([]);
});
for(const candidateMode of ['denied','skew'] as const)test('Root approver '+candidateMode+' never leaves selectable stale candidates',async({page})=>{
 const f=await setup(page,{candidateMode});const dialog=await open(page);await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('checkbox')).toHaveCount(0);await expect(dialog.getByRole('button',{name:'确认',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});
test('Root approver picker late response cannot reopen cancelled dialog',async({page})=>{
 const f=await setup(page,{candidateMode:'late'});const dialog=await open(page);await expect.poll(()=>f.calls.filter(c=>c.path===candidatePath).length).toBe(1);await dialog.getByRole('button',{name:'取消',exact:true}).click();f.release();await expect(dialog).toHaveCount(0);expect(f.writes).toEqual([]);
});

test('Root approver picker places each checkbox beside its label as the source candidate row',async({page})=>{
 await setup(page);const dialog=await open(page);await expect.poll(async()=>Math.round((await dialog.boundingBox())?.width??0)).toBe(640);const geometry=await dialog.getByRole('checkbox',{name:'原审批人甲',exact:true}).evaluate(input=>{const label=input.closest('label')!,walker=document.createTreeWalker(label,NodeFilter.SHOW_TEXT);let node:Node|null;while((node=walker.nextNode()))if(node.textContent?.trim()==='原审批人甲'){const range=document.createRange();range.selectNodeContents(node);const text=range.getBoundingClientRect(),box=input.getBoundingClientRect();return {gap:text.left-box.right,topDifference:Math.abs(text.top-box.top)};}throw Error('Visible label text not found');});expect(geometry.gap).toBeGreaterThanOrEqual(8);expect(geometry.gap).toBeLessThanOrEqual(16);expect(geometry.topDifference).toBeLessThanOrEqual(6);
});

test('Root approver search resets server paging and query context while keeping explicit choices',async({page},testInfo)=>{
 const f=await setup(page),searches:any[]=[];
 await page.route('**/api/v1/'+candidatePath,async r=>{
  const body=r.request().postDataJSON();searches.push(body);
  if(!body.q)return r.fallback();
  return r.fulfill({json:{code:'OK',data:{items:[{id:id(30),label:'财务搜索命中',status:'active',departmentNames:[]}],total:1,page:body.page,pageSize:body.pageSize,queryVersion:'search-token',schemaVersion:1,policyRevision:1},meta:null}});
 });
 const dialog=await open(page);await dialog.getByRole('checkbox',{name:'原审批人乙',exact:true}).uncheck();await dialog.getByRole('button',{name:'下一页',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'次页审批人',exact:true})).toBeVisible();
 const input=dialog.getByRole('textbox',{name:'搜索账号或部门',exact:true});await expect(input).toBeVisible();await input.fill('财务');
 await expect(dialog.getByRole('checkbox',{name:'财务搜索命中',exact:true})).toBeVisible();expect(searches.at(-1)).toEqual({page:1,pageSize:20,q:'财务'});await expect(dialog.getByRole('checkbox',{name:'次页审批人',exact:true})).toHaveCount(0);
 await dialog.getByRole('checkbox',{name:'财务搜索命中',exact:true}).check();await dialog.getByRole('button',{name:'刷新',exact:true}).click();await expect.poll(()=>searches.filter(x=>x.q==='财务').length).toBe(2);expect(searches.at(-1)).toEqual({page:1,pageSize:20,q:'财务'});
 await expect(dialog.getByRole('checkbox',{name:'财务搜索命中',exact:true})).toBeChecked();await testInfo.attach('approver-text-search',{body:await dialog.screenshot(),contentType:'image/png'});await dialog.getByRole('button',{name:'确认',exact:true}).click();expect(f.writes).toEqual([]);await page.getByRole('button',{name:'保存',exact:true}).click();await expect.poll(()=>f.writes.length).toBe(1);expect(f.writes[0].body.graph.nodes[1].approval.assigneeIds).toEqual([id(20),id(30)]);
});
test('Root approver search ignores superseded results and cancelling search never saves node changes',async({page})=>{
 const f=await setup(page);let release:(()=>void)|undefined,oldStarted=false;const searches:any[]=[];
 await page.route('**/api/v1/'+candidatePath,async r=>{
  const body=r.request().postDataJSON();searches.push(body);if(!body.q)return r.fallback();
  if(body.q==='旧'){oldStarted=true;await new Promise<void>(resolve=>{release=resolve;});}
  await r.fulfill({json:{code:'OK',data:{items:[{id:id(body.q==='旧'?30:31),label:body.q==='旧'?'迟到旧成员':'当前新成员',status:'active',departmentNames:[]}],total:1,page:body.page,pageSize:body.pageSize,queryVersion:'query-'+body.q,schemaVersion:1,policyRevision:1},meta:null}}).catch(()=>{});
 });
 const dialog=await open(page),input=dialog.getByRole('textbox',{name:'搜索账号或部门',exact:true});await input.fill('旧');await expect.poll(()=>oldStarted).toBe(true);await input.fill('新');await expect(dialog.getByRole('checkbox',{name:'当前新成员',exact:true})).toBeVisible();release!();await expect(dialog.getByRole('checkbox',{name:'迟到旧成员',exact:true})).toHaveCount(0);
 expect(searches.filter(x=>x.q).map(x=>x)).toEqual([{page:1,pageSize:20,q:'旧'},{page:1,pageSize:20,q:'新'}]);await dialog.getByRole('checkbox',{name:'当前新成员',exact:true}).check();await dialog.getByRole('button',{name:'取消',exact:true}).click();expect(f.writes).toEqual([]);await page.getByRole('button',{name:'选择审批人',exact:true}).click();await expect(dialog.getByRole('checkbox',{name:'原审批人乙',exact:true})).toBeChecked();await expect(dialog.getByRole('textbox',{name:'搜索账号或部门',exact:true})).toHaveValue('');expect(f.writes).toEqual([]);
});

test('Root approver displays every current department beside the account without changing its accessible checkbox label',async({page},testInfo)=>{
 await setup(page);await page.route('**/api/v1/'+candidatePath,async r=>{const body=r.request().postDataJSON();return r.fulfill({json:{code:'OK',data:{items:[{id:id(20),label:'多部门审批人',status:'active',departmentNames:['产品部','财务部']}],total:1,page:body.page,pageSize:body.pageSize,queryVersion:'department-query',schemaVersion:1,policyRevision:1},meta:null}});});
 const dialog=await open(page),checkbox=dialog.getByRole('checkbox',{name:'多部门审批人',exact:true});await expect(checkbox).toBeVisible();const row=checkbox.locator('..');await expect(row.getByText('产品部',{exact:true})).toBeVisible();await expect(row.getByText('财务部',{exact:true})).toBeVisible();
 const name=row.getByText('多部门审批人',{exact:true}),department=row.getByText('财务部',{exact:true});await expect(name).toHaveCSS('font-size','14px');await expect(department).toHaveCSS('font-size','12px');const a=await name.boundingBox(),b=await department.boundingBox();expect(b!.x).toBeGreaterThan(a!.x+a!.width);
 await testInfo.attach('approver-current-departments',{body:await dialog.screenshot(),contentType:'image/png'});
});
test('Root approver without a department is explicitly unassigned and remains selectable',async({page})=>{
 await setup(page);const dialog=await open(page),checkbox=dialog.getByRole('checkbox',{name:'原审批人甲',exact:true});await expect(checkbox).toBeVisible();await expect(checkbox.locator('..').getByText('未分配部门',{exact:true})).toBeVisible();await checkbox.uncheck();await checkbox.check();
});
test('Root malformed department projection cannot expose selectable stale approvers',async({page})=>{
 await setup(page);await page.route('**/api/v1/'+candidatePath,async r=>{const body=r.request().postDataJSON();return r.fulfill({json:{code:'OK',data:{items:[{id:id(20),label:'非法部门成员',status:'active',departmentNames:null}],total:1,page:body.page,pageSize:body.pageSize,queryVersion:'bad',schemaVersion:1,policyRevision:1},meta:null}});});
 const dialog=await open(page);await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog.getByRole('checkbox')).toHaveCount(0);await expect(dialog.getByRole('button',{name:'确认',exact:true})).toBeDisabled();
});
