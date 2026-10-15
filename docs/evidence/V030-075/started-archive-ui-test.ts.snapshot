import {test,expect,type Page} from '@playwright/test';
// V075 source-written current initiated-instance contract and Figma493:8057.
const actor={id:'11111111-1111-4111-8111-111111111111',account:'reviewer'};
const id=(n:number)=>'22222222-2222-4222-8222-'+String(n).padStart(12,'0');
const route='/app/approvals/started';
const item=(n:number)=>({appId:id(90),viewId:id(91),recordId:id(100+n),instance:{id:id(n),flowId:id(92),name:'发起流程 '+n,initiatorId:actor.id,state:'active',createdAt:'2026-10-13T12:00:00Z',updatedAt:'2026-10-13T12:00:00Z',definitionVersion:1,sequence:1}});
const result=(body:any,token='started-v1')=>({items:body.page===1?Array.from({length:20},(_,i)=>item(i+1)):body.page===2?[item(21)]:[],total:21,page:body.page,pageSize:body.pageSize,queryVersion:token});
async function setup(page:Page,reply:(body:any)=>Promise<{status?:number;code?:string;data?:unknown}>|{status?:number;code?:string;data?:unknown}=body=>({data:result(body)})){
 const calls:any[]=[];await page.route('**/api/v1/**',async r=>{const request=r.request(),path=new URL(request.url()).pathname;
  if(path==='/api/v1/workflow-started/search'){const body=request.postDataJSON();calls.push({path,method:request.method(),body,headers:request.headers()});const v=await reply(body);return r.fulfill({status:v.status??200,json:{code:v.code??'OK',data:v.data??null,meta:null}}).catch(()=>{});}
  let data:unknown;if(path.endsWith('/sessions/current'))data=actor;else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};else if(path==='/api/v1/applications')data={items:[]};else if(path==='/api/v1/workflow-tasks/search'){const body=request.postDataJSON();data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'inbox'}}else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null}});
 });return calls;
}
test('Root started instances is an explicit persistent-shell tab with independent server pages',async({page},testInfo)=>{
 const calls=await setup(page);await page.setViewportSize({width:1440,height:1000});await page.goto('/app/approvals');await expect(page.getByText('暂无待办',{exact:true})).toBeVisible();expect(calls).toEqual([]);const shell=await page.getByTestId('workspace-shell').elementHandle();
 await page.getByRole('button',{name:'我发起的',exact:true}).click();await expect(page).toHaveURL(/\/app\/approvals\/started$/);await expect(page.getByRole('heading',{name:'我发起的',exact:true})).toBeVisible();await expect(page.getByText('发起流程 1',{exact:true})).toBeVisible();expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);expect(calls).toHaveLength(1);expect(calls[0].body).toEqual({page:1,pageSize:20});expect(calls[0].headers['x-expected-actor-id']).toBe(actor.id);
 await page.getByRole('button',{name:'下一页',exact:true}).click();await expect(page.getByText('发起流程 21',{exact:true})).toBeVisible();await expect(page.getByText('发起流程 1',{exact:true})).toHaveCount(0);expect(calls.at(-1).body).toEqual({page:2,pageSize:20,queryVersion:'started-v1'});
 await page.evaluate(()=>document.fonts.ready);await testInfo.attach('personal-started-desktop',{body:await page.screenshot(),contentType:'image/png'});await page.goBack();await expect(page).toHaveURL(/\/app\/approvals$/);await expect(page.getByText('暂无待办',{exact:true})).toBeVisible();
});
test('Root started query changes clear old history and require an explicit first-page refresh',async({page})=>{
 let changed=false;const calls=await setup(page,body=>{if(body.page===2){changed=true;return {status:409,code:'APPLICATION_QUERY_CHANGED'}}const data=result(body,changed?'started-v2':'started-v1');if(changed)data.items[0].instance.name='刷新后的发起记录';return {data}});
 await page.goto(route);await expect(page.getByText('发起流程 1',{exact:true})).toBeVisible();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect(page.getByRole('alert')).toContainText('发起记录已变化');await expect(page.getByText('发起流程 1',{exact:true})).toHaveCount(0);expect(calls).toHaveLength(2);
 await page.getByRole('button',{name:'刷新发起记录',exact:true}).click();await expect(page.getByText('刷新后的发起记录',{exact:true})).toBeVisible();expect(calls.at(-1).body).toEqual({page:1,pageSize:20});
});
test('Root started instances failures retry only a read and never report an uncertain approval',async({page})=>{
 let count=0;const calls=await setup(page,body=>++count===1?{status:503,code:'COMMON_SERVICE_UNAVAILABLE'}:{data:{...result(body),items:[],total:0}});await page.goto(route);await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText(/操作结果尚未确认/)).toHaveCount(0);await page.getByRole('button',{name:'重试',exact:true}).click();await expect(page.getByText('暂无发起记录',{exact:true})).toBeVisible();expect(calls).toHaveLength(2);expect(calls.map(x=>x.path)).toEqual(['/api/v1/workflow-started/search','/api/v1/workflow-started/search']);
});
test('Root started instances refuses another actor in an otherwise valid page',async({page})=>{
 await setup(page,body=>{const data=result(body);data.items[0].instance.initiatorId=id(99);return {data}});await page.goto(route);await expect(page.getByRole('alert')).toContainText('发起记录响应格式不正确');await expect(page.getByText('发起流程 1',{exact:true})).toHaveCount(0);
});
test('Root started instances revoked permission is not an empty successful list',async({page})=>{
 await setup(page,()=>({status:403,code:'APPLICATION_FORBIDDEN'}));await page.goto(route);await expect(page.getByRole('alert')).toContainText('没有');await expect(page.getByText('暂无发起记录',{exact:true})).toHaveCount(0);
});
test('Root started instances identity mismatch suspends old actor data',async({page})=>{
 await setup(page,()=>({status:409,code:'AUTH_SESSION_CHANGED'}));await page.goto(route);await expect(page.getByRole('button',{name:'重新核对身份',exact:true})).toBeVisible();await expect(page.getByText('发起流程 1',{exact:true})).toHaveCount(0);
});
test('Root started instances leaving discards a delayed response and returning reads afresh',async({page})=>{
 let release:(()=>void)|undefined;let first=true;const calls=await setup(page,async body=>{if(first){first=false;await new Promise<void>(resolve=>{release=resolve})}return {data:result(body)}});await page.goto(route);await expect.poll(()=>calls.length).toBe(1);await page.getByRole('button',{name:'首页',exact:true}).first().click();await expect(page).toHaveURL(/\/app$/);await expect(page.getByRole('main',{name:'主页',exact:true})).toBeVisible();release!();await expect(page.getByText('发起流程 1',{exact:true})).toHaveCount(0);await page.goBack();await expect(page.getByText('发起流程 1',{exact:true})).toBeVisible();expect(calls).toHaveLength(2);expect(calls[1].body).toEqual({page:1,pageSize:20});
});
test('Root started instances small screen keeps readable rows within the shared table scroll area',async({page},testInfo)=>{
 await setup(page);await page.setViewportSize({width:390,height:844});await page.goto(route);const text=page.getByText('发起流程 1',{exact:true});await expect(text).toBeVisible();await expect(text).toHaveCSS('font-size','14px');const width=(await text.boundingBox())!.width;expect(width).toBeGreaterThanOrEqual(89);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);await testInfo.attach('personal-started-small',{body:await page.screenshot(),contentType:'image/png'});
});

test('Root started page distinguishes pending, active and confirmed terminal instance states',async({page})=>{
 const states=['starting','active','completed','rejected','withdrawn','no_effect'];
 await setup(page,body=>{const data=result(body);for(let i=0;i<6;i++){data.items[i].instance.state=states[i];if(i===0)data.items[i].instance.sequence=0}return {data}});
 await page.goto(route);for(const text of ['正在发起','审批中','已通过','已驳回','已撤回','未执行'])await expect(page.getByText(text,{exact:true}).first()).toBeVisible();
 await expect(page.getByRole('button',{name:'同意',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'撤回',exact:true})).toHaveCount(0);
});

// V075 15:35 independent retained-round variant. Display provenance rather than
// disguising a removed runtime projection as a currently executable instance.
for(const source of ['captured','legacy_last_known'] as const)test('Root started retained round displays archived provenance and '+source+' name source',async({page})=>{
 const calls=await setup(page,body=>({data:{...result(body),total:1,items:[{...item(1),archived:true,flowNameSource:source,instance:{...item(1).instance,name:'保留的差旅流程',state:'rejected'}}]}}));
 await page.goto(route);await expect(page.getByText('保留的差旅流程',{exact:true})).toBeVisible();await expect(page.getByText('已驳回',{exact:true})).toBeVisible();await expect(page.getByText('已归档',{exact:true})).toBeVisible();
 await expect(page.getByText('最后已知名称',{exact:true})).toHaveCount(source==='legacy_last_known'?1:0);
 await expect(page.getByRole('button',{name:'撤回',exact:true})).toHaveCount(0);await expect(page.getByRole('button',{name:'同意',exact:true})).toHaveCount(0);expect(calls).toHaveLength(1);expect(calls[0].body).toEqual({page:1,pageSize:20});
});
