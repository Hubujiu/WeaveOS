import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import Ajv2020 from 'ajv/dist/2020.js';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const ajv=new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'reason-contract'});
const check=(name,value)=>ajv.compile({$ref:'reason-contract#/components/schemas/'+name})(value);
const id='eeeeeeee-eeee-4eee-8eee-000000000001';
const task={operationId:id,action:'reject',basisToken:'basis'};
const back={...task,action:'return',targetNodeId:id};
const event={id,instanceId:id,flowId:id,nodeId:id,targetNodeId:null,actorId:id,action:'reject',outcome:'success',sequence:1,schemaVersion:1,recordVersion:1,occurredAt:'2026-10-14T01:00:00Z',flowName:'报销',flowNameSource:'captured'};
for(const [name,value,key] of [['WorkflowTaskAction',task,'reason'],['WorkflowLifecycleAction',back,'reason'],['WorkflowEventSummary',event,'actionReason']]) {
 test('P25 '+name+' accepts original Unicode reason',()=>assert.equal(check(name,{...value,[key]:'  凭证\n🦊  '}),true));
 test('P25 '+name+' preserves legacy omission',()=>assert.equal(check(name,value),true));
 test('P25 '+name+' rejects invalid scalars and non-text',()=>{for(const reason of ['',null,3,'\u0085\u3000','a\0b','\ud800','\udfff','🦊'.repeat(501)])assert.equal(check(name,{...value,[key]:reason}),false,JSON.stringify(reason));});
 test('P25 '+name+' accepts maximum scalar count and non-whitespace BOM',()=>{for(const reason of ['🦊'.repeat(500),'\ufeff'])assert.equal(check(name,{...value,[key]:reason}),true);});
}
for(const action of ['agree','withdraw','start'])test('P25 event forbids reason for '+action,()=>assert.equal(check('WorkflowEventSummary',{...event,action,actionReason:'not permitted'}),false));
test('P25 unrelated task and lifecycle actions prohibit human reason',()=>{
 assert.equal(check('WorkflowTaskAction',{...task,action:'agree',reason:'x'}),false);
 assert.equal(check('WorkflowLifecycleAction',{...task,action:'withdraw',reason:'x'}),false);
});
