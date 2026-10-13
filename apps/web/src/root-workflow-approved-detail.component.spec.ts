import {test,expect,type Page} from '@playwright/test';
// V075 personal approved detail; V062/V064 and Figma493:8258 / 484:3324.
const id=(n:number)=>'33333333-3333-4333-8333-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'reviewer'},loc={appId:id(2),viewId:id(3),recordId:id(4),eventId:id(5)};
const event={id:loc.eventId,instanceId:id(6),flowId:id(7),flowName:'差旅审批',flowNameSource:'captured',nodeId:id(8),targetNodeId:null,actorId:actor.id,action:'agree',outcome:'success',sequence:2,schemaVersion:1,recordVersion:3,occurredAt:'2026-10-13T12:00:00Z'};
const detail=()=>({event:{...event},basis:{status:'available',fields:[{fieldId:id(9),fieldName:'历史金额',fieldKind:'money',value:'123456789012345678901234567890.12',valueLabels:{}},{fieldId:id(10),fieldName:'原部门',fieldKind:'department',value:id(11),valueLabels:{[id(11)]:{label:'原财务部',deleted:true}}}]}});
const api=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}/workflow-events/${loc.eventId}`;
const path='/app/approvals/completed/'+Object.values(loc).join('/');
async function setup(page:Page,reply:()=>Promise<{status?:number;code?:string;data?:unknown}>|{status?:number;code?:string;data?:unknown}=()=>({data:detail()})){
 const calls:{path:string;method:string;headers:Record<string,string>}[]=[];await page.route('**/api/v1/**',async r=>{
  const request=r.request(),path=new URL(request.url()).pathname;calls.push({path,method:request.method(),headers:request.headers()});
  if(path===api){const v=await reply();return r.fulfill({status:v.status??200,json:{code:v.code??'OK',data:v.data??null,meta:null}}).catch(()=>{});}
  let data:unknown;if(path.endsWith('/sessions/current'))data=actor;else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};else if(path==='/api/v1/applications')data={items:[]};else if(path==='/api/v1/workflow-approvals/search'){const body=request.postDataJSON();data={items:[{appId:loc.appId,viewId:loc.viewId,recordId:loc.recordId,event}],total:1,page:body.page,pageSize:body.pageSize,queryVersion:'approved'}}else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null}});
 });return calls;
}
test('Root approved list opens original event without requiring an active task and retains the shared shell',async({page},testInfo)=>{
 const calls=await setup(page);await page.setViewportSize({width:1440,height:1000});await page.goto('/app/approvals/completed');await expect(page.getByText('差旅审批',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===api)).toHaveLength(0);const shell=await page.getByTestId('workspace-shell').elementHandle();
 await page.getByRole('button',{name:'查看已审批：差旅审批',exact:true}).click();await expect(page).toHaveURL(path);await expect(page.getByRole('heading',{name:'历史审批依据',exact:true})).toBeVisible();await expect(page.getByText('123456789012345678901234567890.12',{exact:true})).toBeVisible();await expect(page.getByText('原财务部（已删除）',{exact:true})).toBeVisible();expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);expect(calls.filter(c=>c.path===api)).toHaveLength(1);expect(calls.find(c=>c.path===api)!.headers['x-expected-actor-id']).toBe(actor.id);
 expect(calls.filter(c=>c.method!=='GET'&&c.path!=='/api/v1/workflow-approvals/search')).toEqual([]);expect(calls.filter(c=>c.path.includes('/tasks/')||c.path.endsWith('/preview'))).toEqual([]);await expect(page.getByRole('button',{name:'同意',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'保存修改',exact:true})).toHaveCount(0);
 await testInfo.attach('personal-approved-event-desktop',{body:await page.screenshot(),contentType:'image/png'});await page.getByRole('button',{name:'返回我已审批',exact:true}).click();await expect(page).toHaveURL('/app/approvals/completed');
});
test('Root approved detail direct navigation and reload validate the original event without trusting list state',async({page})=>{
 const calls=await setup(page,()=>({data:{...detail(),event:{...event,flowNameSource:'legacy_last_known'}}}));await page.goto(path);await expect(page.getByText('最后已知名称',{exact:true})).toBeVisible();await expect(page.getByText('历史金额',{exact:true})).toBeVisible();await page.reload();await expect(page.getByText('历史金额',{exact:true})).toBeVisible();expect(calls.filter(c=>c.path===api)).toHaveLength(2);
});
for(const [name,patch]of [['event',{id:id(90)}],['actor',{actorId:id(91)}]] as const)test('Root approved detail rejects mismatched '+name+' without rendering private basis',async({page})=>{
 await setup(page,()=>({data:{...detail(),event:{...event,...patch}}}));await page.goto(path);await expect(page.getByRole('alert')).toContainText('审批依据响应格式不正确');await expect(page.getByText('历史金额',{exact:true})).toHaveCount(0);
});
test('Root approved detail refresh rechecks permissions and discards formerly readable evidence',async({page})=>{
 let n=0;await setup(page,()=>++n===1?{data:detail()}:{status:403,code:'APPLICATION_FORBIDDEN'});await page.goto(path);await expect(page.getByText('历史金额',{exact:true})).toBeVisible();await page.getByRole('button',{name:'刷新依据',exact:true}).click();await expect(page.getByRole('alert')).toContainText('没有');await expect(page.getByText('历史金额',{exact:true})).toHaveCount(0);
});
for(const status of ['available','unavailable'])test('Root approved detail accurately presents '+status+' empty original basis',async({page})=>{
 await setup(page,()=>({data:{event,basis:{status,fields:[]}}}));await page.goto(path);await expect(page.getByText(status==='available'?'当时可见的审批依据为空':'当时审批依据不可用',{exact:true})).toBeVisible();await expect(page.getByText('历史金额',{exact:true})).toHaveCount(0);
});
test('Root approved detail leaving aborts late original values and returning rechecks',async({page})=>{
 let release:(()=>void)|undefined,n=0;await setup(page,async()=>{if(++n===1)await new Promise<void>(resolve=>{release=resolve});return {data:detail()}});await page.goto(path);await expect.poll(()=>n).toBe(1);await page.getByRole('button',{name:'首页',exact:true}).first().click();await expect(page).toHaveURL('/app');await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();release!();await expect(page.getByText('历史金额',{exact:true})).toHaveCount(0);await page.goBack();await expect(page.getByText('历史金额',{exact:true})).toBeVisible();expect(n).toBe(2);
});
test('Root approved detail narrow screen retains readable original values without outer overflow',async({page},testInfo)=>{
 await setup(page);await page.setViewportSize({width:390,height:844});await page.goto(path);const value=page.getByText('123456789012345678901234567890.12',{exact:true});await expect(value).toBeVisible();await expect(value).toHaveCSS('font-size','14px');expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await testInfo.attach('personal-approved-event-small',{body:await page.screenshot(),contentType:'image/png'});
});

const recordPath=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}`,runtimePath=`/api/v1/applications/${loc.appId}/forms/${loc.viewId}/runtime`;
const currentRecord=()=>({id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(20),createdBy:actor.id,createdAt:'2026-10-13T11:00:00Z',updatedAt:'2026-10-13T13:00:00Z',schemaVersion:1,recordVersion:5,values:{[id(9)]:'987654321098765432109876543210.34'},referenceDisplays:{}});
const currentView=()=>({appId:loc.appId,viewId:loc.viewId,tableId:id(20),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:id(9),name:'当前金额',kind:'money',required:false,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:false,edit:'none',history:'all'},query:{operators:['eq'],sortable:false,quickSearchable:false}}],layout:[],capabilities:{create:false,read:'all',edit:'none',history:'all',search:true,draftCreate:false,draftEdit:false}});
async function currentReads(page:Page,reply:()=>Promise<{status?:number;code?:string;data?:unknown}>|{status?:number;code?:string;data?:unknown}=()=>({data:currentRecord()})){
 const calls:{path:string;method:string;actor:string|undefined}[]=[];
 for(const path of [runtimePath,recordPath])await page.route('**'+path,async r=>{calls.push({path,method:r.request().method(),actor:r.request().headers()['x-expected-actor-id']});const v=path===runtimePath?{data:currentView()}:await reply();await r.fulfill({status:v.status??200,json:{code:v.code??'OK',data:v.data??null,meta:null}}).catch(()=>{});});return calls;
}
async function openComparison(page:Page){await page.goto(path);await expect(page.getByText('历史金额',{exact:true})).toBeVisible();await page.getByRole('button',{name:'查看版本差异',exact:true}).click();}
test('Root approved comparison explicitly rechecks original history and ordinary current record without an active task',async({page},testInfo)=>{
 const calls=await setup(page),current=await currentReads(page);await page.goto(path);await expect(page.getByText('历史金额',{exact:true})).toBeVisible();expect(current).toEqual([]);const shell=await page.getByTestId('workspace-shell').elementHandle();await page.getByRole('button',{name:'查看版本差异',exact:true}).click();const region=page.getByRole('region',{name:'版本对照',exact:true});await expect(region).toContainText('123456789012345678901234567890.12');await expect(region).toContainText('987654321098765432109876543210.34');await expect(region).toContainText('原财务部（已删除）');await expect(region).toContainText('当前不可比较');
 expect(calls.filter(c=>c.path===api)).toHaveLength(2);expect(current).toEqual([{path:runtimePath,method:'GET',actor:actor.id},{path:recordPath,method:'GET',actor:actor.id}]);expect(calls.filter(c=>c.path.includes('/tasks/')||c.method!=='GET')).toEqual([]);expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);
 await testInfo.attach('approved-current-comparison',{body:await page.screenshot(),contentType:'image/png'});await page.getByRole('button',{name:'返回历史审批依据',exact:true}).click();await expect(page.getByRole('button',{name:'查看版本差异',exact:true})).toBeFocused();await expect(page.getByText('历史金额',{exact:true})).toBeVisible();
});
test('Root approved comparison rechecks revoked history before reading any current values',async({page})=>{
 let n=0;await setup(page,()=>++n===1?{data:detail()}:{status:403,code:'APPLICATION_FORBIDDEN'});const current=await currentReads(page);await openComparison(page);await expect(page.getByRole('alert')).toBeVisible();expect(current).toEqual([]);await expect(page.getByText('历史金额',{exact:true})).toHaveCount(0);await expect(page.getByText('987654321098765432109876543210.34',{exact:true})).toHaveCount(0);
});
for(const mode of ['revoked','wrong-record'] as const)test('Root approved comparison refuses '+mode+' current data without displaying a stale snapshot',async({page})=>{
 await setup(page);await currentReads(page,()=>mode==='revoked'?{status:403,code:'APPLICATION_FORBIDDEN'}:{data:{...currentRecord(),id:id(99)}});await openComparison(page);await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText('987654321098765432109876543210.34',{exact:true})).toHaveCount(0);await expect(page.getByRole('table',{name:'版本差异',exact:true})).toHaveCount(0);
});
test('Root approved comparison closing an in-flight read ignores its late current values',async({page})=>{
 let release:(()=>void)|undefined;await setup(page);const current=await currentReads(page,async()=>{await new Promise<void>(resolve=>{release=resolve});return {data:currentRecord()}});await openComparison(page);await expect.poll(()=>current.length).toBe(2);await page.getByRole('button',{name:'返回历史审批依据',exact:true}).click();release!();await expect(page.getByText('历史金额',{exact:true})).toBeVisible();await expect(page.getByRole('region',{name:'版本对照',exact:true})).toHaveCount(0);await expect(page.getByText('987654321098765432109876543210.34',{exact:true})).toHaveCount(0);
});
test('Root approved comparison every new comparison reads a fresh saved record',async({page})=>{
 let n=0;await setup(page);const current=await currentReads(page,()=>{const r=currentRecord();if(++n>1){r.recordVersion=6;r.values[id(9)]='765.43'}return {data:r}});await openComparison(page);await expect(page.getByRole('region',{name:'版本对照',exact:true})).toContainText('987654321098765432109876543210.34');await page.getByRole('button',{name:'返回历史审批依据',exact:true}).click();await page.getByRole('button',{name:'查看版本差异',exact:true}).click();const region=page.getByRole('region',{name:'版本对照',exact:true});await expect(region).toContainText('765.43');await expect(region).toContainText('当前 v6');await expect(region).not.toContainText('987654321098765432109876543210.34');expect(current).toHaveLength(4);
});
test('Root approved comparison narrow screen keeps exact values reachable without outer overflow',async({page},testInfo)=>{
 await setup(page);await currentReads(page);await page.setViewportSize({width:390,height:844});await openComparison(page);const region=page.getByRole('region',{name:'版本对照',exact:true});await expect(region).toContainText('987654321098765432109876543210.34');await region.getByText('987654321098765432109876543210.34',{exact:true}).scrollIntoViewIfNeeded();expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);await expect(page.getByRole('button',{name:'保存修改',exact:true})).toHaveCount(0);await testInfo.attach('approved-comparison-small',{body:await page.screenshot(),contentType:'image/png'});
});

// Actual 5f3c screenshots showed a 56px bordered header and visually truncated
// exact decimals. Figma484:3503 requires the 44px caption header and clean rows;
// V015/P6 exact values must be readable, not merely present in hidden DOM text.
test('Root approved comparison uses source caption header and borderless data rows',async({page})=>{
 await setup(page);await currentReads(page);await openComparison(page);const table=page.getByRole('table',{name:'版本差异',exact:true});await expect(table).toBeVisible();
 for(const header of await table.locator('thead th').all()){
  expect.soft((await header.boundingBox())!.height).toBe(44);await expect.soft(header).toHaveCSS('font-size','12px');await expect.soft(header).toHaveCSS('font-weight','400');await expect.soft(header).toHaveCSS('border-right-width','0px');
 }
 for(const row of await table.locator('tbody tr').all())await expect.soft(row).toHaveCSS('border-bottom-width','0px');
 for(const cell of await table.locator('tbody td').all())await expect.soft(cell).toHaveCSS('border-right-width','0px');
});
for(const width of [1280,390])test('Root approved comparison full exact digits remain rendered inside their cell at width '+width,async({page})=>{
 await setup(page);await currentReads(page);await page.setViewportSize({width,height:900});await openComparison(page);const table=page.getByRole('table',{name:'版本差异',exact:true});await expect(table).toBeVisible();
 for(const value of ['123456789012345678901234567890.12','987654321098765432109876543210.34']){
  const text=table.getByText(value,{exact:true});await text.scrollIntoViewIfNeeded();await expect(text).toBeVisible();
  expect.soft(await text.evaluate(node=>{const cell=node.closest('td')!.getBoundingClientRect(),range=document.createRange();range.selectNodeContents(node);return [...range.getClientRects()].every(r=>r.left>=cell.left-.5&&r.right<=cell.right+.5&&r.top>=cell.top-.5&&r.bottom<=cell.bottom+.5);})).toBe(true);
 }
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
