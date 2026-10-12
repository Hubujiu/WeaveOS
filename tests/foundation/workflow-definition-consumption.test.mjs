import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
// V019 bounded DAG and V020/V044 closed GET candidate definition, never BPMN.
const source=stripTypeScriptTypes(await readFile(new URL('../../apps/web/src/applications/workflows/definitionContract.ts',import.meta.url),'utf8'),{mode:'transform'});const {validWorkflowDefinition:valid}=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
const id=n=>'bcbcbcbc-bcbc-4cbc-8cbc-'+String(n).padStart(12,'0');
const definition=()=>({id:id(9),name:'财务审批',revision:4,currentVersion:1,candidateVersion:2,state:'enabled',schemaVersion:3,allowWithdraw:true,graph:{version:1,nodes:[{id:id(1),kind:'start'},{id:id(2),kind:'approval',approval:{mode:'all',assigneeIds:[id(7),id(8)],editableFieldIds:[id(6)]}},{id:id(3),kind:'end'}],edges:[{from:id(1),to:id(2)},{from:id(2),to:id(3)}]},triggers:[{event:'manual',condition:null}]});
test('P17 accepts unchanged closed candidate definition distinct from published version',()=>{const d=definition(),copy=structuredClone(d);assert.equal(valid(d,id(9)),true);assert.deepEqual(d,copy)});
test('P17 accepts a current condition with both explicitly identified branches and two ends',()=>{const d=definition();d.graph.nodes.push({id:id(4),kind:'condition',condition:{operator:'and',children:[{fieldId:id(6),operator:'gt',value:'5000.00'}]}},{id:id(5),kind:'end'});d.graph.edges=[{from:id(1),to:id(4)},{from:id(4),to:id(2),branch:'true'},{from:id(4),to:id(5),branch:'false'},{from:id(2),to:id(3)}];d.triggers.push({event:'record.created',condition:{operator:'and',children:[{fieldId:id(6),operator:'gte',value:'0'}]}});assert.equal(valid(d,id(9)),true)});
test('P17 accepts unpublished disabled definition without automatically inventing triggers',()=>{const d=definition();Object.assign(d,{currentVersion:0,candidateVersion:1,state:'disabled',triggers:[]});assert.equal(valid(d,id(9)),true)});
for(const [name,patch]of [['different flow',{id:id(10)}],['extra BPMN',{bpmn:'<xml>'}],['unsafe revision',{revision:2**53}],['zero schema',{schemaVersion:0}],['old candidate',{candidateVersion:0}],['unknown state',{state:'active'}],['blank name',{name:' '}],['fake withdraw',{allowWithdraw:'true'}],['duplicate events',{triggers:[{event:'manual',condition:null},{event:'manual',condition:null}]}],['unapproved event',{triggers:[{event:'webhook',condition:null}]}],['trigger extra authority',{triggers:[{event:'manual',condition:null,actor:id(1)}]}]])test('P17 rejects '+name,()=>assert.equal(valid({...definition(),...patch},id(9)),false));
for(const [name,change]of [
 ['duplicate nodes',g=>g.nodes.push(structuredClone(g.nodes[1]))],
 ['unsupported script',g=>g.nodes[1]={id:id(2),kind:'script',script:'eval()'}],
 ['approval extra metadata',g=>g.nodes[1].approval.scope='all'],
 ['empty assignees',g=>g.nodes[1].approval.assigneeIds=[]],
 ['duplicate assignees',g=>g.nodes[1].approval.assigneeIds=[id(7),id(7)]],
 ['duplicate field',g=>g.nodes[1].approval.editableFieldIds=[id(6),id(6)]],
 ['noncanonical identity',g=>g.nodes[1].approval.assigneeIds=[id(7).toUpperCase()]],
 ['missing edge target',g=>g.edges[0].to=id(99)],
 ['branch on approval',g=>g.edges[1].branch='true'],
 ['cycle',g=>g.edges[1].to=id(1)],
 ['orphan end',g=>g.nodes.push({id:id(4),kind:'end'})],
 ['extra start',g=>g.nodes.push({id:id(4),kind:'start'})],
 ['missing start',g=>g.nodes[0].kind='approval'],
 ['duplicate edge',g=>g.edges.push({...g.edges[0]})],
 ['start configuration',g=>g.nodes[0].approval={mode:'any',assigneeIds:[id(7)],editableFieldIds:[]}],
 ['single branch condition',g=>{g.nodes[1]={id:id(2),kind:'condition',condition:{operator:'and',children:[{fieldId:id(6),operator:'eq',value:'x'}]}};g.edges[1].branch='true'}],
 ['oversized graph',g=>{g.nodes=Array.from({length:101},(_,i)=>({id:id(i+1),kind:i?'end':'start'}));}],
 ])test('P17 rejects graph '+name,()=>{const d=definition();change(d.graph);assert.equal(valid(d,id(9)),false)});
test('P17 does not accept hidden expression or numeric coercion in condition AST',()=>{for(const condition of [{operator:'and',children:[{fieldId:id(6),operator:'gt',value:5000}]},{operator:'and',children:[{fieldId:id(6),operator:'eq',value:'x',sql:'SELECT secret'}]}]){const d=definition();d.triggers=[{event:'record.updated',condition}];assert.equal(valid(d,id(9)),false)}});
