import {test,expect,type Page} from '@playwright/test';
// V044 versioned trigger configuration; Figma484:3052. No implicit publication.
const id=(n:number)=>'d7d7d7d7-d7d7-4d7d-8d7d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),base=`applications/${appId}/forms/${viewId}`,definitionPath=base+'/workflows/'+flowId+'/definition',route='/app/'+base+'/workflows/'+flowId+'/design';
const condition={operator:'or',children:[{fieldId:id(30),operator:'eq',value:'差旅'},{operator:'and',children:[{fieldId:id(31),operator:'gte',value:'5000.01'},{fieldId:id(32),operator:'eq',value:false}]}]};
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'condition',condition},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12),branch:'true'},{from:id(11),to:id(12),branch:'false'}]};
const triggers=[{event:'manual',condition:null},{event:'record.created',condition}];
async function setup(page:Page){
 const writes:{path:string;body:any}[]=[];let saved:any=null;const app={id:appId,name:'条件配置',ownerUserId:actor.id,policyRevision:1};
 const fields=[{id:id(30),name:'用途',kind:'text',operators:['eq','neq']},{id:id(31),name:'金额',kind:'money',operators:['eq','neq','gt','gte','lt','lte']},{id:id(32),name:'加急',kind:'boolean',operators:['eq','neq']}].map(f=>({id:f.id,name:f.name,kind:f.kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:f.operators,sortable:f.kind==='money',quickSearchable:f.kind==='text'}}));
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method();let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime')data={appId,viewId,tableId:id(8),schemaVersion:2,viewVersion:1,policyRevision:1,fields,layout:fields.map((f,i)=>({id:id(40+i),kind:'field',fieldId:f.id})),capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 else if(path===definitionPath&&method==='GET')data={id:flowId,name:'条件审批',revision:saved?4:3,currentVersion:1,candidateVersion:saved?3:2,state:'enabled',schemaVersion:2,allowWithdraw:true,graph:saved?.graph??graph,triggers:saved?.triggers??triggers};
 else if(method==='PUT'){const body=req.postDataJSON();writes.push({path,body});saved=body;data={operationId:body.operationId,id:flowId,revision:4,currentVersion:1,candidateVersion:3,state:'enabled'};}
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {writes};
}
async function settings(page:Page){await page.goto(route);await page.getByRole('tab',{name:'触发与设置',exact:true}).click();}
test('Root trigger selection saves all selected events without changing graph or implicitly publishing',async({page})=>{
 const f=await setup(page);await settings(page);await expect(page.getByRole('checkbox',{name:'记录新增',exact:true})).toBeChecked();await page.getByRole('checkbox',{name:'手动发起',exact:true}).uncheck();await page.getByRole('checkbox',{name:'记录修改',exact:true}).check();expect(f.writes).toEqual([]);await expect(page.getByRole('button',{name:'发布',exact:true})).toBeDisabled();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();expect(f.writes).toHaveLength(1);expect(f.writes[0].path).toBe(definitionPath);expect(f.writes[0].body.triggers).toEqual([{event:'record.created',condition},{event:'record.updated',condition:null}]);expect(f.writes[0].body.graph).toEqual(graph);
});
test('Root existing trigger condition preserves its full tree and exact decimal independent of node condition',async({page})=>{
 const f=await setup(page);await settings(page);const region=page.getByRole('region',{name:'记录新增触发条件',exact:true});await region.getByLabel('条件 1.2.1 的值',{exact:true}).fill('9999999999999999.99');await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const next=structuredClone(condition);next.children[1].children![0].value='9999999999999999.99';expect(f.writes[0].body.triggers).toEqual([{event:'manual',condition:null},{event:'record.created',condition:next}]);expect(f.writes[0].body.graph).toEqual(graph);
});
test('Root trigger invalid input blocks Save and leave cancellation retains the draft',async({page})=>{
 const f=await setup(page);await settings(page);const input=page.getByRole('region',{name:'记录新增触发条件',exact:true}).getByLabel('条件 1.2.1 的值',{exact:true});await input.fill('invalid');await expect(page.getByRole('alert')).toContainText('十进制');await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();await page.getByRole('button',{name:'返回主页',exact:true}).click();await page.getByRole('dialog',{name:'有未保存的修改',exact:true}).getByRole('button',{name:'继续编辑',exact:true}).click();await expect(input).toHaveValue('invalid');expect(f.writes).toEqual([]);
});
test('Root trigger empty selection is an explicit empty configuration and never resurrects old conditions',async({page})=>{
 const f=await setup(page);await settings(page);await page.getByRole('checkbox',{name:'手动发起',exact:true}).uncheck();await page.getByRole('checkbox',{name:'记录新增',exact:true}).uncheck();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();expect(f.writes[0].body.triggers).toEqual([]);await page.getByRole('checkbox',{name:'记录新增',exact:true}).check();await page.getByRole('button',{name:'保存',exact:true}).click();await expect.poll(()=>f.writes.length).toBe(2);expect(f.writes[1].body.triggers).toEqual([{event:'record.created',condition:null}]);
});
