import {test,expect,type Page} from '@playwright/test';
// V019 existing recursive AST, current RuntimeView and Figma488:7017; no implementation-derived oracle.
const id=(n:number)=>'d6d6d6d6-d6d6-4d6d-8d6d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),base=`applications/${appId}/forms/${viewId}`,definitionPath=base+'/workflows/'+flowId+'/definition',route='/app/'+base+'/workflows/'+flowId+'/design';
const condition={operator:'or',children:[{fieldId:id(30),operator:'eq',value:'差旅'},{operator:'and',children:[{fieldId:id(31),operator:'gte',value:'5000.01'},{fieldId:id(32),operator:'eq',value:false}]}]};
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'condition',condition},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12),branch:'true'},{from:id(11),to:id(12),branch:'false'}]};
const triggers=[{event:'manual',condition:null}];
async function setup(page:Page){
 const writes:{path:string;body:any}[]=[];let saved:any=null;const app={id:appId,name:'条件配置',ownerUserId:actor.id,policyRevision:1};
 const fields=[{id:id(30),name:'用途',kind:'text',operators:['eq','neq']},{id:id(31),name:'金额',kind:'money',operators:['eq','neq','gt','gte','lt','lte']},{id:id(32),name:'加急',kind:'boolean',operators:['eq','neq']}].map(f=>({id:f.id,name:f.name,kind:f.kind,required:false,presentation:{helpText:null,displayTimeZone:null},input:{},access:{read:'all',create:true,edit:'all',history:'none'},query:{operators:f.operators,sortable:f.kind==='money',quickSearchable:f.kind==='text'}}));
 await page.route('**/api/v1/**',async r=>{const req=r.request(),path=new URL(req.url()).pathname.slice('/api/v1/'.length),method=req.method();let data:unknown;
 if(path==='sessions/current')data=actor;else if(path==='me/access')data={user:actor,bootstrapAdmin:false,personnelManage:false,identities:[],permissions:[],applications:[]};
 else if(path==='applications')data={items:[app]};else if(path==='applications/'+appId)data=app;else if(path==='applications/'+appId+'/access')data={appId,canEnter:true,policyRevision:1,menus:[{resourceKind:'application',resourceId:appId},{resourceKind:'form',resourceId:viewId}]};
 else if(path===base+'/runtime')data={appId,viewId,tableId:id(8),schemaVersion:2,viewVersion:1,policyRevision:1,fields,layout:fields.map((f,i)=>({id:id(40+i),kind:'field',fieldId:f.id})),capabilities:{create:true,read:'all',edit:'all',history:'none',search:true,draftCreate:true,draftEdit:true}};
 else if(path===definitionPath&&method==='GET')data={id:flowId,name:'条件审批',revision:saved?4:3,currentVersion:1,candidateVersion:saved?3:2,state:'enabled',schemaVersion:2,allowWithdraw:true,graph:saved?.graph??graph,triggers};
 else if(method==='PUT'){const body=req.postDataJSON();writes.push({path,body});saved=body;data={operationId:body.operationId,id:flowId,revision:4,currentVersion:1,candidateVersion:3,state:'enabled'};}
 else return r.fulfill({status:404,json:{code:'API_NOT_FOUND',data:null}});
 return r.fulfill({json:{code:'OK',data,meta:null}});
 });return {writes};
}
async function edit(page:Page){await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'条件节点',exact:true}).click();}
const value=(page:Page,path:string)=>page.getByLabel('条件 '+path+' 的值',{exact:true});
test('Root condition editor preserves nested groups and exact decimal text through explicit Save',async({page})=>{
 const f=await setup(page);await edit(page);await expect(value(page,'1.2.1')).toHaveValue('5000.01');await value(page,'1.2.1').fill('9007199254740993.01');expect(f.writes).toEqual([]);await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();
 const expected=structuredClone(graph);(expected.nodes[1].condition!.children[1].children![0]).value='9007199254740993.01';expect(f.writes).toHaveLength(1);expect(f.writes[0].path).toBe(definitionPath);expect(f.writes[0].body.graph).toEqual(expected);expect(f.writes[0].body.triggers).toEqual(triggers);expect(f.writes[0].body.expectedSchemaVersion).toBe(2);
});
test('Root condition editor changes a selected nested group without flattening parent or siblings',async({page})=>{
 const f=await setup(page);await edit(page);await page.getByLabel('条件组 1.2 的逻辑',{exact:true}).selectOption('or');await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const expected=structuredClone(graph);expected.nodes[1].condition!.children[1].operator='or';expect(f.writes[0].body.graph).toEqual(expected);
});
test('Root condition editor blocks invalid decimals and exposes only supported text comparisons',async({page})=>{
 const f=await setup(page);await edit(page);const comparison=page.getByLabel('条件 1.1 的比较',{exact:true});await expect(comparison).toBeVisible();expect(await comparison.locator('option').evaluateAll(options=>options.map(option=>(option as HTMLOptionElement).value))).toEqual(['eq','neq']);await value(page,'1.2.1').fill('not-a-number');await expect(page.getByRole('alert')).toContainText('十进制');await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);
});

test('Root condition editor distinguishes false and NULL without rewriting sibling conditions',async({page})=>{
 const f=await setup(page);await edit(page);await expect(value(page,'1.2.2')).toHaveValue('false');await value(page,'1.2.2').selectOption('true');await page.getByLabel('条件 1.1 使用 NULL',{exact:true}).check();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const expected=structuredClone(graph);expected.nodes[1].condition!.children[1].children![1].value=true;(expected.nodes[1].condition!.children[0] as any).value=null;expect(f.writes[0].body.graph).toEqual(expected);
});
test('Root condition editor appends an AND group only within the selected original group',async({page})=>{
 const f=await setup(page);await edit(page);await page.getByRole('button',{name:'条件组 1.2 添加且条件',exact:true}).click();await page.getByLabel('条件 1.2.3.1 的字段',{exact:true}).selectOption(id(30));await value(page,'1.2.3.1').fill('补充');await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const expected=structuredClone(graph);(expected.nodes[1].condition!.children[1].children! as any[]).push({operator:'and',children:[{fieldId:id(30),operator:'eq',value:'补充'}]});expect(f.writes[0].body.graph).toEqual(expected);
});
test('Root condition editor removes only selected condition and refuses an empty remaining group',async({page})=>{
 const f=await setup(page);await edit(page);await page.getByRole('button',{name:'删除条件 1.2.1',exact:true}).click();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const expected=structuredClone(graph);expected.nodes[1].condition!.children[1].children!.splice(0,1);expect(f.writes[0].body.graph).toEqual(expected);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'条件节点',exact:true}).click();await page.getByRole('button',{name:'删除条件 1.2.1',exact:true}).click();await expect(page.getByRole('alert')).toContainText('非空');await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();expect(f.writes).toHaveLength(1);
});
