import {test,expect,type Page} from '@playwright/test';
// V019 exact-edge insertion + V075 P17 and Figma488:7017/488:6874.
const id=(n:number)=>'d8d8d8d8-d8d8-4d8d-8d8d-'+String(n).padStart(12,'0');
const actor={id:id(1),account:'manager'},appId=id(2),viewId=id(3),flowId=id(4),fieldId=id(5),base=`applications/${appId}/forms/${viewId}`,definitionPath=base+'/workflows/'+flowId+'/definition',route='/app/'+base+'/workflows/'+flowId+'/design';
const graph={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'approval',approval:{mode:'all',assigneeIds:[id(20),id(21)],editableFieldIds:[fieldId]}},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12)}]};
const triggers=[{event:'manual',condition:null},{event:'record.created',condition:{operator:'and',children:[{fieldId,operator:'eq',value:'原触发条件'}]}}];
type Options={graph?:unknown;candidateMode?:'normal'|'changed'|'denied'|'skew'|'late';write?:(body:any,n:number)=>{status:number;code:string;data?:unknown};lookupFails?:boolean;readAfterFails?:boolean};
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
  data={id:flowId,name:'部门审批',revision:saved?4:3,currentVersion:1,candidateVersion:saved?3:2,state:'enabled',schemaVersion:1,allowWithdraw:saved?saved.allowWithdraw:true,graph:saved?.graph??options.graph??graph,triggers:saved?.triggers??triggers};
 }else if(path===base+'/workflows/approvers/search'){
  if(options.candidateMode==='late')await new Promise<void>(resolve=>{release=resolve;});
  if(options.candidateMode==='denied')return r.fulfill({status:403,json:{code:'APPLICATION_FORBIDDEN',data:null}});
  if(options.candidateMode==='changed'&&body.page===2){refreshed=true;return r.fulfill({status:409,json:{code:'COMMON_QUERY_CHANGED',data:null}});}
  data={items:body.page===2?[{id:id(22),label:'次页审批人',status:'active'}]:[{id:id(20),label:refreshed?'刷新后审批人':'原审批人甲',status:'active'},{id:id(21),label:'原审批人乙',status:'active'}],total:21,page:body.page,pageSize:body.pageSize,queryVersion:refreshed?'new-token':'original-token',schemaVersion:options.candidateMode==='skew'?2:1,policyRevision:1};
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
async function selectOriginal(page:Page){await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true}).click();}
test('Root graph insertion selects real approvers then inserts one new approval on only the selected successor edge',async({page})=>{
 const f=await setup(page);await selectOriginal(page);await page.getByRole('button',{name:'在此节点后添加审批节点',exact:true}).click();const picker=page.getByRole('dialog',{name:'选择审批人',exact:true});await expect(picker.getByRole('button',{name:'确认',exact:true})).toBeDisabled();await picker.getByRole('checkbox',{name:'原审批人甲',exact:true}).check();await picker.getByRole('button',{name:'确认',exact:true}).click();await expect(page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true})).toHaveCount(2);expect(f.writes).toEqual([]);await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const body=f.writes[0].body;expect(body.graph.nodes).toHaveLength(4);const added=body.graph.nodes.find((n:any)=>!graph.nodes.some(old=>old.id===n.id));expect(added).toEqual({id:expect.stringMatching(/^[0-9a-f-]{36}$/),kind:'approval',approval:{mode:'all',assigneeIds:[id(20)],editableFieldIds:[]}});expect(body.graph.nodes.filter((n:any)=>n.id!==added.id)).toEqual(graph.nodes);expect(body.graph.edges).toEqual([{from:id(10),to:id(11)},{from:id(11),to:added.id},{from:added.id,to:id(12)}]);expect(body.triggers).toEqual(triggers);expect(f.writes).toHaveLength(1);
});
test('Root graph insertion cancel leaves no placeholder node no dirty draft and no write',async({page})=>{
 const f=await setup(page);await selectOriginal(page);await page.getByRole('button',{name:'在此节点后添加审批节点',exact:true}).click();const picker=page.getByRole('dialog',{name:'选择审批人',exact:true});await picker.getByRole('checkbox',{name:'原审批人甲',exact:true}).check();await picker.getByRole('button',{name:'取消',exact:true}).click();await expect(picker).toHaveCount(0);await expect(page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'审批节点',exact:true})).toHaveCount(1);await expect(page.getByRole('button',{name:'保存',exact:true})).toBeDisabled();expect(f.writes).toEqual([]);await expect(page.getByRole('button',{name:'在此节点后添加审批节点',exact:true})).toBeFocused();
});
test('Root graph insertion on true branch preserves the distinct false edge with the same target',async({page})=>{
 const original={version:1,nodes:[{id:id(10),kind:'start'},{id:id(11),kind:'condition',condition:{operator:'and',children:[{fieldId,operator:'eq',value:'条件'}]}},{id:id(12),kind:'end'}],edges:[{from:id(10),to:id(11)},{from:id(11),to:id(12),branch:'true'},{from:id(11),to:id(12),branch:'false'}]};const f=await setup(page,{graph:original});await page.goto(route);await page.getByRole('region',{name:'流程图',exact:true}).getByRole('button',{name:'条件节点',exact:true}).click();await page.getByRole('button',{name:'在条件成立分支添加审批节点',exact:true}).click();const picker=page.getByRole('dialog',{name:'选择审批人',exact:true});await picker.getByRole('checkbox',{name:'原审批人甲',exact:true}).check();await picker.getByRole('button',{name:'确认',exact:true}).click();await page.getByRole('button',{name:'保存',exact:true}).click();await expect(page.getByText('定义已保存',{exact:true})).toBeVisible();const next=f.writes[0].body.graph,node=next.nodes.find((n:any)=>n.kind==='approval');expect(next.nodes.filter((n:any)=>n.id!==node.id)).toEqual(original.nodes);expect(next.edges).toEqual([{from:id(10),to:id(11)},{from:id(11),to:node.id,branch:'true'},{from:node.id,to:id(12)},{from:id(11),to:id(12),branch:'false'}]);
});
