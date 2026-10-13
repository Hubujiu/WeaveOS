import {test,expect,type Page} from '@playwright/test';
// V075 source-written global approved-history contract and Figma493:8258.
const actor={id:'11111111-1111-4111-8111-111111111111',account:'reviewer'};
const id=(n:number)=>'22222222-2222-4222-8222-'+String(n).padStart(12,'0');
const route='/app/approvals/completed';
const item=(n:number)=>({appId:id(90),viewId:id(91),recordId:id(100+n),event:{id:id(n),instanceId:id(200+n),flowId:id(92),flowName:'已审批流程 '+n,flowNameSource:'captured',nodeId:id(93),targetNodeId:null,actorId:actor.id,action:'agree',outcome:'success',sequence:2,schemaVersion:1,recordVersion:1,occurredAt:'2026-10-13T12:00:00Z'}});
const result=(body:any,token='approved-v1')=>({items:body.page===1?Array.from({length:20},(_,i)=>item(i+1)):body.page===2?[item(21)]:[],total:21,page:body.page,pageSize:body.pageSize,queryVersion:token});
async function setup(page:Page,reply:(body:any)=>Promise<{status?:number;code?:string;data?:unknown}>|{status?:number;code?:string;data?:unknown}=body=>({data:result(body)})){
 const calls:any[]=[];await page.route('**/api/v1/**',async r=>{const request=r.request(),path=new URL(request.url()).pathname;
  if(path==='/api/v1/workflow-approvals/search'){const body=request.postDataJSON();calls.push({path,method:request.method(),body,headers:request.headers()});const v=await reply(body);return r.fulfill({status:v.status??200,json:{code:v.code??'OK',data:v.data??null,meta:null}}).catch(()=>{});}
  let data:unknown;if(path.endsWith('/sessions/current'))data=actor;else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};else if(path==='/api/v1/applications')data={items:[]};else if(path==='/api/v1/workflow-tasks/search'){const body=request.postDataJSON();data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'inbox'}}else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
  return r.fulfill({json:{code:'OK',data,meta:null}});
 });return calls;
}
test('Root approved history is an explicit persistent-shell tab with independent server pages',async({page},testInfo)=>{
 const calls=await setup(page);await page.setViewportSize({width:1440,height:1000});await page.goto('/app/approvals');await expect(page.getByText('暂无待办',{exact:true})).toBeVisible();expect(calls).toEqual([]);const shell=await page.getByTestId('workspace-shell').elementHandle();
 await page.getByRole('button',{name:'我已审批',exact:true}).click();await expect(page).toHaveURL(/\/app\/approvals\/completed$/);await expect(page.getByRole('heading',{name:'我已审批',exact:true})).toBeVisible();await expect(page.getByText('已审批流程 1',{exact:true})).toBeVisible();expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);expect(calls).toHaveLength(1);expect(calls[0].body).toEqual({page:1,pageSize:20});expect(calls[0].headers['x-expected-actor-id']).toBe(actor.id);
 await page.getByRole('button',{name:'下一页',exact:true}).click();await expect(page.getByText('已审批流程 21',{exact:true})).toBeVisible();await expect(page.getByText('已审批流程 1',{exact:true})).toHaveCount(0);expect(calls.at(-1).body).toEqual({page:2,pageSize:20,queryVersion:'approved-v1'});
 await page.evaluate(()=>document.fonts.ready);await testInfo.attach('personal-approved-desktop',{body:await page.screenshot(),contentType:'image/png'});await page.goBack();await expect(page).toHaveURL(/\/app\/approvals$/);await expect(page.getByText('暂无待办',{exact:true})).toBeVisible();
});
test('Root approved query changes clear old history and require an explicit first-page refresh',async({page})=>{
 let changed=false;const calls=await setup(page,body=>{if(body.page===2){changed=true;return {status:409,code:'APPLICATION_QUERY_CHANGED'}}const data=result(body,changed?'approved-v2':'approved-v1');if(changed)data.items[0].event.flowName='刷新后的已审批';return {data}});
 await page.goto(route);await expect(page.getByText('已审批流程 1',{exact:true})).toBeVisible();await page.getByRole('button',{name:'下一页',exact:true}).click();await expect(page.getByRole('alert')).toContainText('已审批记录已变化');await expect(page.getByText('已审批流程 1',{exact:true})).toHaveCount(0);expect(calls).toHaveLength(2);
 await page.getByRole('button',{name:'刷新已审批',exact:true}).click();await expect(page.getByText('刷新后的已审批',{exact:true})).toBeVisible();expect(calls.at(-1).body).toEqual({page:1,pageSize:20});
});
test('Root approved history failures retry only a read and never report an uncertain approval',async({page})=>{
 let count=0;const calls=await setup(page,body=>++count===1?{status:503,code:'COMMON_SERVICE_UNAVAILABLE'}:{data:{...result(body),items:[],total:0}});await page.goto(route);await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByText(/操作结果尚未确认/)).toHaveCount(0);await page.getByRole('button',{name:'重试',exact:true}).click();await expect(page.getByText('暂无已审批记录',{exact:true})).toBeVisible();expect(calls).toHaveLength(2);expect(calls.map(x=>x.path)).toEqual(['/api/v1/workflow-approvals/search','/api/v1/workflow-approvals/search']);
});
test('Root approved history refuses another actor in an otherwise valid page',async({page})=>{
 await setup(page,body=>{const data=result(body);data.items[0].event.actorId=id(99);return {data}});await page.goto(route);await expect(page.getByRole('alert')).toContainText('已审批响应格式不正确');await expect(page.getByText('已审批流程 1',{exact:true})).toHaveCount(0);
});
test('Root approved history revoked permission is not an empty successful list',async({page})=>{
 await setup(page,()=>({status:403,code:'APPLICATION_FORBIDDEN'}));await page.goto(route);await expect(page.getByRole('alert')).toContainText('没有');await expect(page.getByText('暂无已审批记录',{exact:true})).toHaveCount(0);
});
test('Root approved history identity mismatch suspends old actor data',async({page})=>{
 await setup(page,()=>({status:409,code:'AUTH_SESSION_CHANGED'}));await page.goto(route);await expect(page.getByRole('button',{name:'重新核对身份',exact:true})).toBeVisible();await expect(page.getByText('已审批流程 1',{exact:true})).toHaveCount(0);
});
test('Root approved history leaving discards a delayed response and returning reads afresh',async({page})=>{
 let release:(()=>void)|undefined;let first=true;const calls=await setup(page,async body=>{if(first){first=false;await new Promise<void>(resolve=>{release=resolve})}return {data:result(body)}});await page.goto(route);await expect.poll(()=>calls.length).toBe(1);await page.getByRole('button',{name:'首页',exact:true}).first().click();await expect(page).toHaveURL(/\/app$/);release!();await expect(page.getByText('已审批流程 1',{exact:true})).toHaveCount(0);await page.goBack();await expect(page.getByText('已审批流程 1',{exact:true})).toBeVisible();expect(calls).toHaveLength(2);expect(calls[1].body).toEqual({page:1,pageSize:20});
});
test('Root approved history small screen keeps readable rows within the shared table scroll area',async({page},testInfo)=>{
 await setup(page);await page.setViewportSize({width:390,height:844});await page.goto(route);const text=page.getByText('已审批流程 1',{exact:true});await expect(text).toBeVisible();await expect(text).toHaveCSS('font-size','14px');const width=(await text.boundingBox())!.width;expect(width).toBeGreaterThanOrEqual(89);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth)).toBe(true);await testInfo.attach('personal-approved-small',{body:await page.screenshot(),contentType:'image/png'});
});
