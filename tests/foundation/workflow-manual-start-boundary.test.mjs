import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/workflows/manualStartContract.ts',import.meta.url),'utf8');
const {validManualStartRequest}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id=n=>'dddddddd-dddd-4ddd-8ddd-'+String(n).padStart(12,'0');
const resource={kind:'manual-start',appId:id(1),viewId:id(2),id:id(3),flowId:id(4)};
const path=`applications/${id(1)}/forms/${id(2)}/records/${id(3)}/workflow-starts`;
const request={path,method:'POST',expectedStatus:202,body:{flowId:id(4),expectedWorkflowRevision:7,expectedSchemaVersion:2,expectedRecordVersion:9}};
test('V044 exact manual packet is accepted before shared operationId generation',()=>assert.equal(validManualStartRequest(resource,request),true));
for(const key of ['expectedWorkflowRevision','expectedSchemaVersion','expectedRecordVersion'])test('V044 preserves maximum safe '+key,()=>assert.equal(validManualStartRequest(resource,{...request,body:{...request.body,[key]:Number.MAX_SAFE_INTEGER}}),true));
for(const key of ['appId','viewId','id','flowId'])for(const value of ['', '../escape','00000000-0000-0000-0000-000000000000'])test('V044 refuses invalid resource '+key+' '+value,()=>assert.equal(validManualStartRequest({...resource,[key]:value},request),false));
for(const [name,r,q] of [
 ['record resource',{...resource,kind:'record'},request],['extra resource',{...resource,creationNonce:id(8)},request],['flow mismatch',resource,{...request,body:{...request.body,flowId:id(8)}}],
 ['wrong record',resource,{...request,path:path.replace(id(3),id(8))}],['query',resource,{...request,path:path+'?bypass=1'}],['fragment',resource,{...request,path:path+'#x'}],['trailing slash',resource,{...request,path:path+'/'}],
 ['PATCH',resource,{...request,method:'PATCH'}],['201',resource,{...request,expectedStatus:201}],['200',resource,{...request,expectedStatus:200}],['missing body',resource,{...request,body:undefined}],
 ['caller operationId',resource,{...request,body:{...request.body,operationId:id(8)}}],['actor body',resource,{...request,body:{...request.body,actorId:id(8)}}],['business values',resource,{...request,body:{...request.body,values:{}}}],
])test('V044 manual packet rejects '+name,()=>assert.equal(validManualStartRequest(r,q),false));
for(const key of ['expectedWorkflowRevision','expectedSchemaVersion','expectedRecordVersion'])for(const value of [0,-1,1.5,'1',undefined,Number.MAX_SAFE_INTEGER+1])test('V044 refuses invalid '+key+' '+String(value),()=>assert.equal(validManualStartRequest(resource,{...request,body:{...request.body,[key]:value}}),false));
