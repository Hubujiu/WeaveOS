import {test,expect,type Page} from '@playwright/test';
// V070 full manifest import; P21 first UI slice, Figma487:5146/5238.
const id=(n:number)=>'e9e9e9e9-e9e9-4e9e-8e9e-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'creator'},newApp=id(9),route='/app/application-templates/import';
const manifest={format:'weaveos.structure-template',version:1,application:{id:id(2),name:'空模板'},directories:[],tables:[],forms:[],workflows:[],permissionGroups:[]};
const counts={directories:0,tables:0,fields:0,forms:0,workflows:0,permissionGroups:0};
type Mode='normal'|'no-create'|'preflight-denied'|'preflight-delay'|'unknown'|'bad-receipt';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string;body:any}[]=[],writes:any[]=[];let release:()=>void=()=>{},changed=false;const held=new Promise<void>(r=>{release=r;});
 const result=(operationId:string)=>({operationId,appId:newApp,policyRevision:1,structureVersion:1,status:'imported'});
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method(),body=req.postData()?req.postDataJSON():null;calls.push({path,method,body});let data:unknown;
 if(path==='sessions/current')data=changed?{id:id(99),account:'other'}:actor;
 else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:mode==='no-create'?[]:[{code:'applications.create',name:'应用管理',category:'system',appId:null}],applications:[]};
 else if(path==='applications')data={items:[]};
 else if(path==='application-templates/preflight'){
  if(mode==='preflight-delay')await held;
  if(mode==='preflight-denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  data={valid:true,counts};
 }else if(path==='application-templates/import'){
  writes.push(body);
  if(mode==='unknown'&&writes.length===1)return r.fulfill({status:503,json:{code:'APPLICATION_OPERATION_UNCONFIRMED',data:{operationId:body.operationId}}});
  return r.fulfill({status:201,headers:{Location:'/api/v1/applications/'+newApp},json:{code:'OK',data:mode==='bad-receipt'?{...result(body.operationId),appId:id(98)}:result(body.operationId),meta:null}});
 }else if(path.startsWith('application-operations/'))data={operationId:path.split('/').at(-1),status:'confirmed',httpStatus:201,location:'/api/v1/applications/'+newApp,result:result(path.split('/').at(-1)!)};
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,writes,release,changeIdentity:()=>{changed=true;}};
}
async function open(page:Page){await page.goto('/app/applications');await page.getByRole('button',{name:'导入应用模板',exact:true}).click();await expect(page).toHaveURL(new RegExp(route+'$'));return page.getByRole('dialog',{name:'导入应用模板',exact:true});}
async function file(page:Page,value:unknown=manifest){await page.getByLabel('选择模板文件',{exact:true}).setInputFiles({name:'结构模板.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(value))});}
async function checked(page:Page){await file(page);await page.getByRole('button',{name:'检查模板',exact:true}).click();await expect(page.getByText('模板检查通过',{exact:true})).toBeVisible();}
test('Root import file selection uses the approved dialog and cancellation sends no template request',async({page},info)=>{
 const f=await setup(page);const dialog=await open(page);await expect(dialog).toContainText('.json');await expect(dialog.getByRole('button',{name:'检查模板',exact:true})).toBeDisabled();await expect.poll(async()=>{const b=await dialog.boundingBox();return b?{w:Math.round(b.width),h:Math.round(b.height)}:null;}).toEqual({w:660,h:490});await info.attach('template-import-file',{body:await page.screenshot(),contentType:'image/png'});await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(page).toHaveURL(/\/app\/applications$/);await expect(page.getByRole('button',{name:'导入应用模板',exact:true})).toBeFocused();expect(f.calls.filter(c=>c.path.startsWith('application-templates/'))).toEqual([]);
});
test('Root template preflight never creates an app and explicit confirmation submits the exact manifest once',async({page})=>{
 const f=await setup(page);await open(page);await checked(page);expect(f.writes).toEqual([]);expect(f.calls.find(c=>c.path==='application-templates/preflight')!.body).toEqual({manifest,bindings:[]});await expect(page.getByText('导入后流程保持关闭',{exact:true})).toBeVisible();await page.getByRole('button',{name:'确认导入',exact:true}).evaluate(n=>{(n as HTMLButtonElement).click();(n as HTMLButtonElement).click();});await expect(page.getByText('模板已导入',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);expect(f.writes[0]).toEqual({operationId:expect.any(String),manifest,bindings:[]});await expect(page.getByRole('button',{name:'打开新应用',exact:true})).toBeVisible();expect(f.calls.some(c=>/publish|enable/.test(c.path))).toBe(false);
});
for(const kind of ['extra-root','duplicate','oversized'] as const)test('Root import rejects '+kind+' file before any template request',async({page})=>{
 const f=await setup(page);await open(page);const raw=kind==='extra-root'?JSON.stringify({...manifest,records:[]}):kind==='duplicate'?JSON.stringify(manifest).replace('"version":1','"version":1,"version":1'):' '.repeat(1048577);await page.getByLabel('选择模板文件',{exact:true}).setInputFiles({name:'bad.json',mimeType:'application/json',buffer:Buffer.from(raw)});await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('button',{name:'检查模板',exact:true})).toBeDisabled();expect(f.calls.filter(c=>c.path.startsWith('application-templates/'))).toEqual([]);
});
test('Root denied template preflight shows an error without claiming import or keeping confirmation',async({page})=>{
 const f=await setup(page,'preflight-denied');await open(page);await file(page);await page.getByRole('button',{name:'检查模板',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();await expect(page.getByRole('button',{name:'确认导入',exact:true})).toHaveCount(0);await expect(page.getByText('模板已导入',{exact:true})).toHaveCount(0);expect(f.writes).toEqual([]);
});
test('Root cancelling template preflight aborts the real request and a late result cannot reopen confirmation',async({page})=>{
 const f=await setup(page,'preflight-delay');await open(page);await file(page);await page.getByRole('button',{name:'检查模板',exact:true}).click();await expect.poll(()=>f.calls.filter(c=>c.path==='application-templates/preflight').length).toBe(1);const abort=page.waitForEvent('requestfailed',r=>r.url().endsWith('/application-templates/preflight'));await page.getByRole('button',{name:'取消',exact:true}).click();await abort;f.release();await expect(page).toHaveURL(/\/app\/applications$/);await expect(page.getByRole('dialog',{name:'导入应用模板',exact:true})).toHaveCount(0);expect(f.writes).toEqual([]);
});
test('Root template unknown import recovers its original receipt after actual route unmount',async({page})=>{
 const f=await setup(page,'unknown');await open(page);await checked(page);await page.getByRole('button',{name:'确认导入',exact:true}).click();await expect(page.getByRole('button',{name:'核查导入结果',exact:true})).toBeVisible();await expect(page.getByLabel('选择模板文件',{exact:true})).toBeDisabled();await page.getByRole('button',{name:'关闭并保留待核查操作',exact:true}).click();await expect(page).toHaveURL(/\/app\/applications$/);await page.getByRole('button',{name:'导入应用模板',exact:true}).click();await expect(page.getByRole('button',{name:'核查导入结果',exact:true})).toBeVisible();await page.getByRole('button',{name:'核查导入结果',exact:true}).click();await expect(page.getByText('模板已导入',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);expect(f.calls.some(c=>c.path==='application-operations/'+f.writes[0].operationId)).toBe(true);
});
test('Root malformed import receipt cannot claim completion and retries preserve the exact original body',async({page})=>{
 const f=await setup(page,'bad-receipt');await open(page);await checked(page);await page.getByRole('button',{name:'确认导入',exact:true}).click();await expect(page.getByRole('button',{name:'重试原导入',exact:true})).toBeVisible();await expect(page.getByText('模板已导入',{exact:true})).toHaveCount(0);await page.getByRole('button',{name:'重试原导入',exact:true}).click();await expect.poll(()=>f.writes.length).toBe(2);expect(f.writes[1]).toEqual(f.writes[0]);
});
test('Root identity change after preflight cannot send a template import',async({page})=>{
 const f=await setup(page);await open(page);await checked(page);f.changeIdentity();await page.getByRole('button',{name:'确认导入',exact:true}).click();await expect(page.getByRole('alert')).toContainText('登录身份已变化');expect(f.writes).toEqual([]);
});
test('Root non-creator has no import control and a direct import route cannot expose upload',async({page})=>{
 const f=await setup(page,'no-create');await page.goto('/app/applications');await expect(page.getByRole('heading',{name:'应用中心',exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'导入应用模板',exact:true})).toHaveCount(0);await page.goto(route);await expect(page.getByText('没有应用创建权限',{exact:true})).toBeVisible();await expect(page.getByLabel('选择模板文件',{exact:true})).toHaveCount(0);expect(f.calls.filter(c=>c.path.startsWith('application-templates/'))).toEqual([]);
});
