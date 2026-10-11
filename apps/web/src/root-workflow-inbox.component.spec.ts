import {test,expect,type Page} from '@playwright/test';
// Oracle: V061 PRD/ADR + V075 first-stage contract, not current DOM.
const actor={id:'11111111-1111-4111-8111-111111111111',account:'reviewer'};
const id=(n:number)=>'22222222-2222-4222-8222-'+String(n).padStart(12,'0');
const item=(n:number)=>({id:id(n),appId:id(90),viewId:id(91),recordId:id(92+n),instanceId:id(93+n),flowId:id(94),nodeId:id(95),flowName:'财务审批 '+n,createdAt:'2026-10-11T09:00:00Z',definitionVersion:1,activationEpoch:1,sequence:0});
async function setup(page:Page,reply:(body:any)=>{status?:number;code?:string;data?:unknown}){
 const calls:any[]=[];
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(),path=new URL(req.url()).pathname;
  let data:unknown;
  if(path.endsWith('/sessions/current'))data=actor;
  else if(path.endsWith('/me/access'))data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
  else if(path==='/api/v1/applications')data={items:[]};
  else if(path==='/api/v1/workflow-tasks/search'){
   const body=req.postDataJSON();calls.push({body,headers:req.headers(),method:req.method()});
   const result=reply(body);
   return route.fulfill({status:result.status??200,json:{code:result.code??'OK',message:'',data:result.data??null,meta:null}});
  }else return route.fulfill({status:404,json:{code:'APPLICATION_NOT_FOUND',data:null}});
  return route.fulfill({status:200,json:{code:'OK',message:'',data,meta:null}});
 });
 return calls;
}
test('Root inbox is reachable in persistent shell and uses authoritative server pagination',async({page})=>{
 const calls=await setup(page,body=>({data:{items:[item(body.page)],total:21,page:body.page,pageSize:20,queryVersion:'inbox-v1'}}));
 await page.goto('/app');
 const shell=await page.getByTestId('workspace-shell').elementHandle();
 await page.getByRole('button',{name:'我的待办',exact:true}).click();
 await expect(page).toHaveURL(/\/app\/approvals$/);
 await expect(page.getByRole('heading',{name:'我的待办',exact:true})).toBeVisible();
 await expect(page.getByText('财务审批 1',{exact:true})).toBeVisible();
 expect(await shell!.evaluate(n=>n.isConnected)).toBe(true);
 expect(calls[0].body).toEqual({page:1,pageSize:20});
 expect(calls[0].method).toBe('POST');
 expect(calls[0].headers['x-expected-actor-id']).toBe(actor.id);
 await page.getByRole('button',{name:'下一页',exact:true}).click();
 await expect(page.getByText('财务审批 2',{exact:true})).toBeVisible();
 expect(calls[1].body).toEqual({page:2,pageSize:20,queryVersion:'inbox-v1'});
 await expect(page.getByText('财务审批 1',{exact:true})).toHaveCount(0);
});
test('Root inbox query change requires explicit refresh, resets first page and drops stale token',async({page})=>{
 let changed=false;
 const calls=await setup(page,body=>{
  if(body.page===2){changed=true;return {status:409,code:'APPLICATION_QUERY_CHANGED'};}
  return {data:{items:[item(changed?3:1)],total:21,page:1,pageSize:20,queryVersion:changed?'inbox-v2':'inbox-v1'}};
 });
 await page.goto('/app/approvals');
 await expect(page.getByText('财务审批 1',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'下一页',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('待办已变化');
 expect(calls).toHaveLength(2);
 await expect(page.getByText('财务审批 1',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'刷新待办',exact:true}).click();
 await expect(page.getByText('财务审批 3',{exact:true})).toBeVisible();
 expect(calls[2].body).toEqual({page:1,pageSize:20});
});
test('Root inbox read failure is not an unconfirmed write and retry repeats only the read',async({page})=>{
 let first=true;
 const calls=await setup(page,body=>{
  if(first){first=false;return {status:503,code:'COMMON_SERVICE_UNAVAILABLE'};}
  return {data:{items:[],total:0,page:body.page,pageSize:20,queryVersion:'empty'}};
 });
 await page.goto('/app/approvals');
 await expect(page.getByRole('alert')).toBeVisible();
 await expect(page.getByText(/操作结果尚未确认/)).toHaveCount(0);
 await page.getByRole('button',{name:'重试',exact:true}).click();
 await expect(page.getByText('暂无待办', {exact:true})).toBeVisible();
 expect(calls).toHaveLength(2);
 expect(calls[1].body).toEqual({page:1,pageSize:20});
});
test('Root inbox rejects malformed or cross-page results instead of displaying untrusted rows',async({page})=>{
 await setup(page,()=>({data:{items:[{...item(1),sequence:-1}],total:1,page:1,pageSize:20,queryVersion:'bad'}}));
 await page.goto('/app/approvals');
 await expect(page.getByRole('alert')).toContainText('待办响应格式不正确');
 await expect(page.getByText('财务审批 1',{exact:true})).toHaveCount(0);
});
test('Root inbox direct route handles revoked access without inventing empty success',async({page})=>{
 await setup(page,()=>({status:403,code:'APPLICATION_FORBIDDEN'}));
 await page.goto('/app/approvals');
 await expect(page.getByRole('alert')).toContainText('没有');
 await expect(page.getByText('暂无待办',{exact:true})).toHaveCount(0);
 await page.getByTestId('workspace-nav-home').click();
 await expect(page).toHaveURL(/\/app$/);
});
test('Root inbox expired query never silently restarts pagination',async({page})=>{
 const calls=await setup(page,body=>body.queryVersion?{status:409,code:'APPLICATION_QUERY_CONTEXT_EXPIRED'}:{data:{items:[item(1)],total:21,page:1,pageSize:20,queryVersion:'q-old'}});
 await page.goto('/app/approvals');await expect(page.getByText('财务审批 1',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'下一页',exact:true}).click();
 await expect(page.getByRole('alert')).toContainText('刷新');await expect(page.getByText('财务审批 1',{exact:true})).toHaveCount(0);expect(calls).toHaveLength(2);
 await page.getByRole('button',{name:'刷新待办',exact:true}).click();await expect(page.getByText('财务审批 1',{exact:true})).toBeVisible();
 expect(calls[2].body).toEqual({page:1,pageSize:20});
});
test('Root inbox leaving during read discards late response and back navigation fetches anew',async({page})=>{
 await setup(page,()=>({data:{items:[item(2)],total:1,page:1,pageSize:20,queryVersion:'fresh'}}));
 let release!:()=>void;const gate=new Promise<void>(r=>release=r);let entered=false;
 await page.route('**/api/v1/workflow-tasks/search',async route=>{
  if(entered)return route.fallback();entered=true;await gate;
  await route.fulfill({json:{code:'OK',data:{items:[item(1)],total:1,page:1,pageSize:20,queryVersion:'late'},meta:null}}).catch(()=>{});
 });
 await page.goto('/app/approvals');await expect.poll(()=>entered).toBe(true);
 await page.getByTestId('workspace-nav-home').click();release();
 await expect(page.getByRole('heading',{name:'主页',exact:true})).toBeVisible();await expect(page.getByText('财务审批 1',{exact:true})).toHaveCount(0);
 await page.goBack();await expect(page.getByText('财务审批 2',{exact:true})).toBeVisible();await expect(page.getByText('财务审批 1',{exact:true})).toHaveCount(0);
});
test('Root inbox identity mismatch suspends old account data until explicit identity check',async({page})=>{
 await setup(page,()=>({status:409,code:'AUTH_SESSION_CHANGED'}));
 await page.goto('/app/approvals');
 await expect(page.getByRole('alert')).toContainText('登录身份已变化');
 await expect(page.getByRole('button',{name:'重新核对身份',exact:true})).toBeVisible();
 await expect(page.getByText('暂无待办',{exact:true})).toHaveCount(0);
 await expect(page.getByTestId('workspace-shell')).toHaveCount(0);
});
