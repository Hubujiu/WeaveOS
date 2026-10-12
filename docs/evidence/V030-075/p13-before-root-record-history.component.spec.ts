import {test,expect,type Page} from '@playwright/test';
// V015 §§11.4/14.3, V017 §4; Figma483:2928. Actual Shell, only external HTTP mocked.
const id=(n:number)=>'ffffffff-ffff-4fff-8fff-'+String(n).padStart(12,'0'),actor={id:id(1),account:'history-reader'},appId=id(2),viewId=id(3),recordId=id(4),fieldId=id(5),tableId=id(6),base=`applications/${appId}/forms/${viewId}`,history=base+'/records/'+recordId+'/history';
const before='12345678901234567890.01',after='12345678901234567890.02';
async function setup(page:Page,mode:'normal'|'denied'|'leak'|'no-grant'|'late'='normal'){
 const calls:{path:string;method:string;query:string;body:any}[]=[];let release:(()=>void)|undefined;
 const row={id:recordId,appId,viewId,tableId,createdBy:actor.id,createdAt:'2026-10-12T09:00:00Z',updatedAt:'2026-10-12T10:00:00Z',recordVersion:3,schemaVersion:1,values:{[fieldId]:after},referenceDisplays:{}};
 const runtime={appId,viewId,tableId,schemaVersion:1,viewVersion:1,policyRevision:1,fields:[{id:fieldId,name:'金额',kind:'money',required:false,presentation:{helpText:null,displayTimeZone:null},input:{decimal:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'}},access:{read:'all',create:true,edit:'all',history:mode==='no-grant'?'none':'all'},query:{operators:['eq','neq'],sortable:true,quickSearchable:false}}],layout:[{id:id(7),kind:'field',fieldId}],capabilities:{create:true,read:'all',edit:'all',history:mode==='no-grant'?'none':'all',search:true,draftCreate:true,draftEdit:true}};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),url=new URL(req.url()),path=url.pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,query:url.search,body});let data:unknown;const app={id:appId,name:'历史测试',ownerUserId:id(99),policyRevision:1};
 if(path==='sessions/current')data=actor;
 else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;
 else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime')data=runtime;
 else if(path===base+'/records/search')data={items:[row],total:1,page:body.page,pageSize:body.pageSize,sort:body.sort,queryVersion:'records',schemaVersion:1,viewVersion:1};
 else if(path===base+'/records/'+recordId)data=row;
 else if(path===base+'/records/'+recordId+'/workflow-instances/search')data={items:[],total:0,page:body.page,pageSize:body.pageSize,queryVersion:'flows'};
 else if(path===history){
  if(mode==='late')await new Promise<void>(resolve=>{release=resolve;});
  if(mode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  const second=url.searchParams.get('pageToken')==='history.next';const change={fieldId,fieldKind:'money',fieldLabel:'历史金额',fieldDeleted:false,before,after,valueLabels:{}};const event={id:id(second?21:20),actorId:actor.id,recordVersionBefore:second?0:2,recordVersionAfter:second?1:3,occurredAt:'2026-10-12T10:00:00Z',origin:second?'task_save':'ordinary',changes:[change],...(mode==='leak'?{privateValue:'hidden-history-secret'}:{})};
  return r.fulfill({status:200,json:{code:'OK',data:{items:[event]},meta:{pagination:{hasMore:!second,nextPageToken:second?null:'history.next'}}}});
 }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({status:200,json:{code:'OK',data,meta:null}});
 });return {calls,release:()=>release?.()};
}
async function open(page:Page){await page.goto('/app/'+base);await page.getByRole('button',{name:'打开记录：金额 '+after,exact:true}).click();await expect(page.getByRole('region',{name:'记录详情',exact:true})).toBeVisible();}
test('Root saved-record history is read only and keeps exact old and new decimal strings',async({page})=>{
 const api=await setup(page);await open(page);expect(api.calls.filter(c=>c.path===history)).toEqual([]);await page.getByRole('button',{name:'查看历史',exact:true}).click();const dialog=page.getByRole('dialog',{name:'记录保存历史',exact:true});await expect(dialog).toContainText('历史金额');await expect(dialog.getByText(before,{exact:true})).toBeVisible();await expect(dialog.getByText(after,{exact:true})).toBeVisible();await expect(dialog).toContainText('普通保存');expect(api.calls.filter(c=>c.path===history).every(c=>c.method==='GET'&&new URLSearchParams(c.query).get('pageSize')==='20')).toBe(true);expect(api.calls.filter(c=>['PATCH','DELETE','PUT'].includes(c.method)||c.method==='POST'&&!c.path.endsWith('/search'))).toEqual([]);await test.info().attach('record-saved-history',{body:await page.screenshot({fullPage:true}),contentType:'image/png'});
});
test('Root record history cursor replaces its bounded batch and refreshes from the first batch',async({page})=>{
 const api=await setup(page);await open(page);await page.getByRole('button',{name:'查看历史',exact:true}).click();const dialog=page.getByRole('dialog',{name:'记录保存历史',exact:true});await expect(dialog).toContainText('普通保存');await dialog.getByRole('button',{name:'下一批历史',exact:true}).click();await expect(dialog).toContainText('节点保存');await expect(dialog).not.toContainText('普通保存');await dialog.getByRole('button',{name:'刷新历史',exact:true}).click();await expect(dialog).toContainText('普通保存');const queries=api.calls.filter(c=>c.path===history).map(c=>new URLSearchParams(c.query).get('pageToken'));expect(queries).toEqual([null,'history.next',null]);
});
test('Root record history permission failure does not discard an independently editable record',async({page})=>{
 await setup(page,'denied');await open(page);await page.getByRole('button',{name:'编辑记录',exact:true}).click();await page.getByRole('textbox',{name:'金额',exact:true}).fill('12.');await page.getByRole('button',{name:'查看历史',exact:true}).click();const dialog=page.getByRole('dialog',{name:'记录保存历史',exact:true});await expect(dialog.getByRole('alert')).toContainText('权限');await dialog.getByRole('button',{name:'关闭',exact:true}).click();await expect(page.getByRole('textbox',{name:'金额',exact:true})).toHaveValue('12.');await expect(page.getByRole('textbox',{name:'金额',exact:true})).toBeEditable();await page.getByRole('button',{name:'返回主页',exact:true}).click();await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'继续编辑',exact:true}).click();await expect(page.getByRole('textbox',{name:'金额',exact:true})).toHaveValue('12.');
});
test('Root record history rejects extra private fields instead of displaying a partial event',async({page})=>{
 await setup(page,'leak');await open(page);await page.getByRole('button',{name:'查看历史',exact:true}).click();const dialog=page.getByRole('dialog',{name:'记录保存历史',exact:true});await expect(dialog.getByRole('alert')).toBeVisible();await expect(dialog).not.toContainText('hidden-history-secret');await expect(dialog).not.toContainText('历史金额');
});
test('Root no history grant does not expose an entry or issue a history read',async({page})=>{
 const api=await setup(page,'no-grant');await open(page);await expect(page.getByRole('textbox',{name:'金额',exact:true})).toHaveValue(after);await expect(page.getByRole('button',{name:'查看历史',exact:true})).toHaveCount(0);expect(api.calls.filter(c=>c.path===history)).toEqual([]);
});
test('Root closing pending record history ignores its late response and leaves the record mounted',async({page})=>{
 const api=await setup(page,'late');await open(page);await page.getByRole('button',{name:'查看历史',exact:true}).click();const dialog=page.getByRole('dialog',{name:'记录保存历史',exact:true});await expect.poll(()=>api.calls.filter(c=>c.path===history).length).toBe(1);await dialog.getByRole('button',{name:'关闭',exact:true}).click();api.release();await expect(dialog).toHaveCount(0);await expect(page.getByRole('textbox',{name:'金额',exact:true})).toHaveValue(after);await expect(page.getByText('历史金额',{exact:true})).toHaveCount(0);
});
