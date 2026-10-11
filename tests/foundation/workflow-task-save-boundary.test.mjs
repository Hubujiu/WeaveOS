import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/workflows/taskSaveContract.ts',import.meta.url),'utf8');
const {validTaskRecordRequest}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id=n=>'66666666-6666-4666-8666-'+String(n).padStart(12,'0');
const resource={kind:'task-record',appId:id(1),viewId:id(2),id:id(3),instanceId:id(4),taskId:id(5)};
const path=`applications/${id(1)}/forms/${id(2)}/records/${id(3)}/workflow-instances/${id(4)}/tasks/${id(5)}/record`;
const request={path,method:'PATCH',expectedStatus:200,body:{basisToken:'b'.repeat(43),changes:{[id(6)]:'123456789012345678901234567890.12'}}};
test('node Save path is exact and accepts only a closed pre-operation packet',()=>assert.equal(validTaskRecordRequest(resource,request),true));
for(const value of [null,false,'text',[id(7)]])test('node Save preserves permitted field value '+JSON.stringify(value),()=>assert.equal(validTaskRecordRequest(resource,{...request,body:{...request.body,changes:{[id(6)]:value}}}),true));
const malformed={
 'ordinary record resource':{resource:{...resource,kind:'record'}},'draft resource':{resource:{...resource,kind:'draft'}},
 'missing task identity':{resource:{...resource,taskId:undefined}},'malformed instance identity':{resource:{...resource,instanceId:'../other'}},
 'extra resource scope':{resource:{...resource,creationNonce:id(7)}},'other record path':{request:{...request,path:path.replace(id(3),id(99))}},
 'query suffix':{request:{...request,path:path+'?override=true'}},'fragment suffix':{request:{...request,path:path+'#other'}},'action path':{request:{...request,path:path.replace(/record$/,'actions')}},
 'POST instead of PATCH':{request:{...request,method:'POST'}},'DELETE instead of PATCH':{request:{...request,method:'DELETE'}},'wrong success status':{request:{...request,expectedStatus:202}},
 'missing body':{request:{...request,body:undefined}},'extra body key':{request:{...request,body:{...request.body,actorId:id(8)}}},'client operation key':{request:{...request,body:{...request.body,operationId:id(8)}}},
 'empty changes':{request:{...request,body:{...request.body,changes:{}}}},'numeric conversion':{request:{...request,body:{...request.body,changes:{[id(6)]:123.45}}}},
 'object value':{request:{...request,body:{...request.body,changes:{[id(6)]:{hidden:'private'}}}}},'invalid changed field':{request:{...request,body:{...request.body,changes:{'../other':'x'}}}},
 'missing basis':{request:{...request,body:{changes:{[id(6)]:'x'}}}},'invalid basis':{request:{...request,body:{...request.body,basisToken:'short'}}},
};
for(const [name,change] of Object.entries(malformed))test('node Save rejects '+name,()=>assert.equal(validTaskRecordRequest(change.resource??resource,change.request??request),false));
