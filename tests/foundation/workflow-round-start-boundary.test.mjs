import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source=readFileSync(new URL('../../apps/web/src/applications/workflows/roundContract.ts',import.meta.url),'utf8');
const {validRoundStartRequest:valid}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
// V069 third slice: only the selected prior instance, no implicit data save.
const id=n=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');
const resource={kind:'round-start',appId:id(1),viewId:id(2),id:id(3),instanceId:id(4),flowId:id(5),roundKind:'resubmit'};
const path=`applications/${id(1)}/forms/${id(2)}/records/${id(3)}/workflows/${id(4)}/round-actions`;
const request={path,method:'POST',expectedStatus:202,body:{kind:'resubmit',expectedWorkflowRevision:7,expectedSchemaVersion:2,expectedRecordVersion:9}};
for(const kind of ['resubmit','review'])test('V069 exact '+kind+' admission does not include business changes',()=>assert.equal(valid({...resource,roundKind:kind},{...request,body:{...request.body,kind}}),true));
for(const key of ['appId','viewId','id','instanceId','flowId'])for(const value of ['../escape','00000000-0000-0000-0000-000000000000'])test('V069 invalid resource '+key+' '+value,()=>assert.equal(valid({...resource,[key]:value},request),false));
for(const [name,r,q]of [
 ['manual resource',{...resource,kind:'manual-start'},request],['extra resource',{...resource,creationNonce:id(8)},request],['kind mismatch',resource,{...request,body:{...request.body,kind:'review'}}],['unknown kind',{...resource,roundKind:'restart'},{...request,body:{...request.body,kind:'restart'}}],
 ['another instance',resource,{...request,path:path.replace(id(4),id(9))}],['withdraw path',resource,{...request,path:path.replace('/workflows/','/workflow-instances/').replace('round-actions','actions')}],['query',resource,{...request,path:path+'?x=1'}],['PATCH',resource,{...request,method:'PATCH'}],['200',resource,{...request,expectedStatus:200}],
 ['caller key',resource,{...request,body:{...request.body,operationId:id(9)}}],['implicit save',resource,{...request,body:{...request.body,changes:{[id(20)]:'99.00'}}}],['missing body',resource,{...request,body:undefined}],
])test('V069 round request rejects '+name,()=>assert.equal(valid(r,q),false));
for(const key of ['expectedWorkflowRevision','expectedSchemaVersion','expectedRecordVersion'])for(const value of [0,'1',1.5,undefined,Number.MAX_SAFE_INTEGER+1])test('V069 rejects '+key+' '+value,()=>assert.equal(valid(resource,{...request,body:{...request.body,[key]:value}}),false));
