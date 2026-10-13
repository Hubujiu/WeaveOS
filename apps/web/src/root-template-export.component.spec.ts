import {test,expect,type Page} from '@playwright/test';
import {readFile} from 'node:fs/promises';
// V070 complete export + Figma487:5368; real Shell and real browser download.
const id=(n:number)=>'e6e6e6e6-e6e6-4e6e-8e6e-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),base=`applications/${appId}/forms/${viewId}/workflows/${flowId}`,route='/app/'+base+'/design';
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'end'}],edges:[{from:id(10),to:id(11)}]};
const manifest={format:'weaveos.structure-template',version:1,application:{id:appId,name:'../../采购应用'},directories:[],tables:[{id:id(8),name:'采购',directoryId:null,position:0,fields:[{id:id(5),name:'金额',kind:'number',required:false,default:'123456789012345678901234567890.12',config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}}]}],forms:[{id:viewId,tableId:id(8),name:'采购表单',directoryId:null,position:0,layout:[{id:id(9),kind:'field',fieldId:id(5),span:12}]}],workflows:[{id:flowId,tableId:id(8),viewId,name:'采购审批',graph,allowWithdraw:true,triggers:[{event:'manual',condition:null}]}],permissionGroups:[{id:id(12),name:'业务成员',enabled:true,memberIds:[actor.id],grants:[]}]};
type Mode='normal'|'denied'|'wrong-app'|'extra-root'|'delay'|'identity'|'member';
async function setup(page:Page,mode:Mode='normal'){
 const calls:{path:string;method:string}[]=[];let release:()=>void=()=>{};const held=new Promise<void>(resolve=>{release=resolve;});
 const app={id:appId,name:'../../采购应用',ownerUserId:mode==='member'?id(99):actor.id,policyRevision:1};
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length);calls.push({path,method:req.method()});let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===`applications/${appId}/forms/${viewId}/runtime`)data={appId,viewId,tableId:id(8),schemaVersion:1,viewVersion:1,policyRevision:1,fields:[],layout:[],capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 else if(path===base+'/definition')data={id:flowId,name:'采购审批',revision:3,currentVersion:1,candidateVersion:1,state:'enabled',schemaVersion:1,allowWithdraw:true,graph,triggers:[{event:'manual',condition:null}]};
 else if(path===`applications/${appId}/structure-template`){
  if(mode==='delay')await held;
  if(mode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  if(mode==='identity')return r.fulfill({status:409,json:{code:'AUTH_SESSION_CHANGED',data:null}});
  data=mode==='wrong-app'?{...manifest,application:{...manifest.application,id:id(99)}}:mode==='extra-root'?{...manifest,records:[{private:'not-template'}]}:manifest;
 }else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 await r.fulfill({json:{code:'OK',data,meta:null}});
 });return {calls,release};
}
async function open(page:Page){await page.goto(route);await page.getByRole('button',{name:'导出应用模板',exact:true}).click();return page.getByRole('dialog',{name:'导出应用模板',exact:true});}
test('Root template export confirmation is truthful, matches approved geometry and cancellation reads nothing',async({page},testInfo)=>{
 const f=await setup(page);const dialog=await open(page);await expect(dialog).toContainText('已保存');await expect(dialog).toContainText('权限组');await expect(dialog).toContainText('不包含业务记录');await expect.poll(async()=>{const box=await dialog.boundingBox();return box?{w:Math.round(box.width),h:Math.round(box.height)}:null;}).toEqual({w:620,h:440});await testInfo.attach('template-export-confirmation',{body:await page.screenshot(),contentType:'image/png'});expect(f.calls.filter(c=>c.path.endsWith('/structure-template'))).toEqual([]);await dialog.getByRole('button',{name:'取消',exact:true}).click();await expect(dialog).toHaveCount(0);await expect(page.getByRole('button',{name:'导出应用模板',exact:true})).toBeFocused();expect(f.calls.filter(c=>c.path.endsWith('/structure-template'))).toEqual([]);
});
test('Root explicit export downloads the complete exact manifest with a safe UUID filename and no mutations',async({page})=>{
 const f=await setup(page);const dialog=await open(page);const downloadPromise=page.waitForEvent('download');await dialog.getByRole('button',{name:'导出模板',exact:true}).click();const download=await downloadPromise;expect(download.suggestedFilename()).toBe('weaveos-template-'+appId+'.json');expect(JSON.parse(await readFile((await download.path())!,'utf8'))).toEqual(manifest);expect(f.calls.filter(c=>c.path.endsWith('/structure-template'))).toEqual([{path:`applications/${appId}/structure-template`,method:'GET'}]);expect(f.calls.filter(c=>c.method!=='GET')).toEqual([]);
});
for(const mode of ['denied','wrong-app','extra-root','identity'] as const)test('Root template '+mode+' response never produces a download',async({page})=>{
 const f=await setup(page,mode);const downloads:unknown[]=[];page.on('download',d=>downloads.push(d));const dialog=await open(page);await dialog.getByRole('button',{name:'导出模板',exact:true}).click();await expect(page.getByRole('alert')).toBeVisible();expect(downloads).toEqual([]);expect(f.calls.filter(c=>c.path.endsWith('/structure-template'))).toHaveLength(1);
});
test('Root cancelled pending template export rejects late response without a download and restores focus',async({page})=>{
 const f=await setup(page,'delay'),downloads:unknown[]=[];page.on('download',d=>downloads.push(d));const dialog=await open(page);await dialog.getByRole('button',{name:'导出模板',exact:true}).click();await expect.poll(()=>f.calls.filter(c=>c.path.endsWith('/structure-template')).length).toBe(1);const aborted=page.waitForEvent('requestfailed',request=>new URL(request.url()).pathname.endsWith('/structure-template'));await dialog.getByRole('button',{name:'取消',exact:true}).click();await aborted;await expect(dialog).toHaveCount(0);f.release();await page.getByRole('button',{name:'导出应用模板',exact:true}).click();await expect(page.getByRole('dialog',{name:'导出应用模板',exact:true}).getByRole('button',{name:'导出模板',exact:true})).toBeEnabled();expect(downloads).toEqual([]);
});
test('Root repeated export confirmation cannot start a second concurrent download',async({page})=>{
 const f=await setup(page,'delay');const dialog=await open(page);await dialog.getByRole('button',{name:'导出模板',exact:true}).evaluate(node=>{(node as HTMLButtonElement).click();(node as HTMLButtonElement).click();});await expect.poll(()=>f.calls.filter(c=>c.path.endsWith('/structure-template')).length).toBe(1);await expect(dialog.getByRole('button',{name:'导出模板',exact:true})).toBeDisabled();const downloadPromise=page.waitForEvent('download');f.release();await downloadPromise;expect(f.calls.filter(c=>c.path.endsWith('/structure-template'))).toHaveLength(1);
});
test('Root ordinary application member never sees a management template export control',async({page})=>{const f=await setup(page,'member');await page.goto(route);await expect(page.getByText('没有流程管理权限',{exact:true})).toBeVisible();await expect(page.getByRole('button',{name:'导出应用模板',exact:true})).toHaveCount(0);expect(f.calls.filter(c=>c.path.endsWith('/structure-template'))).toEqual([]);});
