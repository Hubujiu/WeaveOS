import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source=readFileSync(new URL('../../apps/web/src/applications/workflows/manualStartContract.ts',import.meta.url),'utf8');
const {validManualStartPage:page,validManualStartReceipt:receipt}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
// V044 ADR sixth/seventh slices: current authorized candidates and durable intent,
// never deployment/engine success, graph, assignees, business values or commandId.
const id=n=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
const option={flowId:id(1),name:'财务流程',workflowRevision:3,definitionVersion:2,schemaVersion:4,recordVersion:9};
const result={items:[option],total:1,page:1,pageSize:20,queryVersion:'manual-options-context'};
test('V044 authorized bounded candidates retain exact independent CAS versions',()=>assert.equal(page(result,1,20),true));
test('V044 no candidates is a valid authorized empty result',()=>assert.equal(page({...result,items:[],total:0},1,20),true));
test('V044 an arbitrary empty deep page is allowed without invented results',()=>assert.equal(page({...result,items:[],page:9},9,20),true));
for(const [name,change]of [['graph',{graph:{}}],['assignees',{assigneeIds:[id(4)]}],['values',{values:{secret:'hidden'}}],['missing name',{name:undefined}],['null name',{name:null}],['zero flow',{flowId:'00000000-0000-0000-0000-000000000000'}],['bad flow',{flowId:'flow-one'}],['uppercase flow',{flowId:id(1).toUpperCase()}],['unsafe revision',{workflowRevision:9007199254740992}],['string version',{recordVersion:'9'}],['zero schema',{schemaVersion:0}],['fractional definition',{definitionVersion:1.5}]])test('V044 candidate rejects '+name,()=>assert.equal(page({...result,items:[{...option,...change}]},1,20),false));
for(const [name,change]of [['duplicate flows',{items:[option,option],total:2}],['wrong page',{page:2}],['wrong page size',{pageSize:50}],['negative total',{total:-1}],['unsafe total',{total:9007199254740992}],['too many items',{items:Array.from({length:21},(_,i)=>({...option,flowId:id(100+i)})),total:21}],['items exceed total',{total:0}],['empty context',{queryVersion:''}],['long context',{queryVersion:'x'.repeat(257)}],['extra payload',{graph:{}}],['missing items',{items:undefined}]])test('V044 page rejects '+name,()=>assert.equal(page({...result,...change},1,20),false));
const accepted={operationId:id(2),flowId:id(1),instanceId:id(3),status:'accepted',ignored:false};
test('V044 accepted minimum receipt confirms intent only',()=>assert.equal(receipt(accepted,id(2),id(1)),true));
test('V044 ignored receipt identifies the original running instance',()=>assert.equal(receipt({...accepted,status:'ignored',ignored:true},id(2),id(1)),true));
for(const [name,change]of [['other operation',{operationId:id(9)}],['other flow',{flowId:id(9)}],['missing instance',{instanceId:undefined}],['zero instance',{instanceId:'00000000-0000-0000-0000-000000000000'}],['engine success',{status:'completed'}],['inconsistent accepted',{ignored:true}],['inconsistent ignored',{status:'ignored'}],['coerced ignored',{ignored:'false'}],['command capability',{commandId:id(9)}],['private graph',{graph:{}}],['record values',{values:{}}]])test('V044 receipt rejects '+name,()=>assert.equal(receipt({...accepted,...change},id(2),id(1)),false));
