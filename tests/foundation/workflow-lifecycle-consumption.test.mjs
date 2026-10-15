import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const encode=src=>'data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(src,{mode:'transform'})).toString('base64');
let src=await readFile(new URL('../../apps/web/src/applications/workflows/lifecycleContract.ts',import.meta.url),'utf8');
if(src.includes("from './taskPreviewContract'")){const shared=await readFile(new URL('../../apps/web/src/applications/workflows/taskPreviewContract.ts',import.meta.url),'utf8');src=src.replace("'./taskPreviewContract'",JSON.stringify(encode(shared)));}
const {validLifecyclePreview}=await import(encode(src));
const id=n=>'cccccccc-cccc-4ccc-8ccc-'+String(n).padStart(12,'0');
const loc={appId:id(1),viewId:id(2),recordId:id(3),instanceId:id(4)};
const field={id:id(5),name:'撤回时金额',kind:'money',required:false,default:null,config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'},presentation:{helpText:null,displayTimeZone:null}};
const fixture=()=>({basisToken:'life-basis',instance:{id:loc.instanceId,sequence:3,state:'active'},record:{id:loc.recordId,appId:loc.appId,viewId:loc.viewId,tableId:id(6),createdBy:id(7),createdAt:'2026-10-11T09:00:00Z',updatedAt:'2026-10-11T10:00:00Z',recordVersion:2,schemaVersion:1,values:{[field.id]:'123456789012345678901234567890.12'},referenceDisplays:{}},fields:[structuredClone(field)],canWithdraw:true,returnTargets:[]});
// V045 lifecycle token contract is nonempty <=256, not a guessed format.
test('lifecycle accepts exact path-bound withdrawal capability and precise masked values',()=>{const v=fixture();assert.equal(validLifecyclePreview(v,loc),true);assert.equal(v.record.values[field.id],'123456789012345678901234567890.12');});
test('lifecycle permits return-only qualification without inventing withdrawal',()=>{const v=fixture();v.canWithdraw=false;v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:2}];assert.equal(validLifecyclePreview(v,loc),true);assert.equal(v.canWithdraw,false);});
test('lifecycle accepts empty visible projection without inferring hidden fields',()=>{const v=fixture();v.fields=[];v.record.values={};assert.equal(validLifecyclePreview(v,loc),true);});
test('lifecycle retains current reference tombstone labels',()=>{const v=fixture();v.fields[0]={...field,kind:'member',config:{}};v.record.values[field.id]=id(8);v.record.referenceDisplays={[field.id]:{[id(8)]:{id:id(8),label:'离职成员',deleted:true}}};assert.equal(validLifecyclePreview(v,loc),true);});
const invalid={
 'extra top key':v=>v.actorId=id(99),'missing token':v=>delete v.basisToken,'empty token':v=>v.basisToken='','oversized token':v=>v.basisToken='a'.repeat(257),
 'wrong instance':v=>v.instance.id=id(99),'terminal state':v=>v.instance.state='withdrawn','extra instance':v=>v.instance.canWithdraw=true,'negative sequence':v=>v.instance.sequence=-1,'unsafe sequence':v=>v.instance.sequence=Number.MAX_SAFE_INTEGER+1,
 'wrong record':v=>v.record.id=id(99),'wrong app':v=>v.record.appId=id(99),'wrong view':v=>v.record.viewId=id(99),'extra record':v=>v.record.privateValue='hidden','hidden field':v=>v.record.values[id(99)]='hidden',
 'wrong money type':v=>v.record.values[field.id]=123,'bad date':v=>v.record.updatedAt='2026-02-30T00:00:00Z','duplicate field':v=>v.fields.push(v.fields[0]),'bad field config':v=>v.fields[0].config.roundingMode='RANDOM',
 'string capability':v=>v.canWithdraw='true','missing capability':v=>delete v.canWithdraw,'target null':v=>v.returnTargets=null,
 'extra target':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:1,actorId:id(7)}],
 'missing target task':v=>v.returnTargets=[{nodeId:id(8),activationEpoch:1}],
 'zero target epoch':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:0}],
 'unsafe target epoch':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:Number.MAX_SAFE_INTEGER+1}],
 'duplicate target node':v=>v.returnTargets=[{nodeId:id(8),taskId:id(9),activationEpoch:1},{nodeId:id(8),taskId:id(10),activationEpoch:2}],
 'target limit':v=>v.returnTargets=Array.from({length:101},(_,n)=>({nodeId:id(100+n),taskId:id(500+n),activationEpoch:1})),
 'field limit':v=>{v.fields=Array.from({length:201},(_,n)=>({...field,id:id(100+n)}));v.record.values=Object.fromEntries(v.fields.map(f=>[f.id,null]));},
};
for(const [name,change]of Object.entries(invalid))test('lifecycle rejects '+name,()=>{const v=fixture();change(v);assert.equal(validLifecyclePreview(v,loc),false);});
test('lifecycle rejects invalid structures or scope without throwing',()=>{for(const v of [null,[],{},true,1,''])assert.equal(validLifecyclePreview(v,loc),false);assert.equal(validLifecyclePreview(fixture(),{...loc,recordId:'bad'}),false);});
const {validInstanceActionRequest}=await import(encode(src));
const resource={kind:'instance-action',appId:loc.appId,viewId:loc.viewId,id:loc.recordId,instanceId:loc.instanceId};
const actionPath=`applications/${loc.appId}/forms/${loc.viewId}/records/${loc.recordId}/workflow-instances/${loc.instanceId}/actions`;
const request={path:actionPath,method:'POST',expectedStatus:202,body:{action:'withdraw',basisToken:'life-basis'}};
test('withdraw scoped request accepts exact four-ID instance resource without client operation ID',()=>assert.equal(validInstanceActionRequest(resource,request),true));
for(const [name,r,q]of [
 ['wrong resource',{...resource,kind:'task-action'},request],['extra task',{...resource,taskId:id(9)},request],['invalid identity',{...resource,instanceId:'../x'},request],
 ['another instance',resource,{...request,path:actionPath.replace(loc.instanceId,id(99))}],['task path',resource,{...request,path:actionPath.replace('/actions','/tasks/'+id(9)+'/actions')}],
 ['query',resource,{...request,path:actionPath+'?x=1'}],['wrong status',resource,{...request,expectedStatus:200}],['wrong method',resource,{...request,method:'PATCH'}],
 ['target forbidden',resource,{...request,body:{...request.body,targetNodeId:id(9)}}],['client operation',resource,{...request,body:{...request.body,operationId:id(9)}}],
 ['unapproved return',resource,{...request,body:{action:'return',basisToken:'life-basis'}}],['missing basis',resource,{...request,body:{action:'withdraw'}}],
])test('withdraw scoped request rejects '+name,()=>assert.equal(validInstanceActionRequest(r,q),false));
const apiSource=await readFile(new URL('../../apps/web/src/applications/api.ts',import.meta.url),'utf8');
const {applicationWorkflowApi,validWorkflowTransportRequest}=await import(encode(apiSource));
const actionBody={...request.body,operationId:id(50)};
for(const status of [200,202])test('withdraw transport preserves HTTP '+status+' and exact body',async t=>{
 const old=Object.getOwnPropertyDescriptor(globalThis,'document');Object.defineProperty(globalThis,'document',{configurable:true,value:{cookie:'__Host-csrf=synthetic'}});t.after(()=>{if(old)Object.defineProperty(globalThis,'document',old);else delete globalThis.document;});
 let sent;t.mock.method(globalThis,'fetch',async(url,init)=>{sent={url,init};return new Response(JSON.stringify({code:'OK',data:{status:status===202?'pending':'success'},meta:null}),{status});});
 const result=await applicationWorkflowApi(id(7),actionPath,'POST',actionBody);assert.equal(result.httpStatus,status);assert.equal(sent.url,'/api/v1/'+actionPath);assert.equal(sent.init.headers['X-CSRF-Token'],'synthetic');assert.equal(sent.init.headers['X-Expected-Actor-Id'],id(7));assert.deepEqual(JSON.parse(sent.init.body),actionBody);
});
for(const [name,path,body] of [['wrong action',actionPath,{...actionBody,action:'agree'}],['return pending decision',actionPath,{...actionBody,action:'return',targetNodeId:id(9)}],['extra target',actionPath,{...actionBody,targetNodeId:id(9)}],['query',actionPath+'?x=1',actionBody],['missing basis',actionPath,{operationId:id(50),action:'withdraw'}]])test('withdraw transport refuses '+name,()=>assert.equal(validWorkflowTransportRequest(path,'POST',body),false));
const operationSource=await readFile(new URL('../../apps/web/src/applications/workflows/taskActionContract.ts',import.meta.url),'utf8');
const {validWorkflowOperation}=await import(encode(operationSource));
const expected={operationId:id(50),instanceId:loc.instanceId,action:'withdraw'};
const success={operationId:id(50),commandId:id(51),instanceId:loc.instanceId,status:'success',instanceState:'withdrawn',sequence:4};
test('withdraw result accepts only actual withdrawn terminal receipt',()=>assert.equal(validWorkflowOperation(success,expected,200,'GET'),true));
for(const state of ['active','completed','rejected'])test('withdraw result refuses unrelated success '+state,()=>assert.equal(validWorkflowOperation({...success,instanceState:state},expected,200,'GET'),false));
test('task action result cannot borrow withdrawn success from instance action',()=>assert.equal(validWorkflowOperation(success,{operationId:id(50),instanceId:loc.instanceId},200,'GET'),false));
// P25 approved original Unicode reason; client admission is not server authorization.
const originalReason='  凭证\n🦊  ';
const returnBody={operationId:id(50),action:'return',basisToken:'life-basis',targetNodeId:id(8),reason:originalReason};
test('P25 return request retains explicit target and original reason in both admission layers',()=>{
 assert.equal(validWorkflowTransportRequest(actionPath,'POST',returnBody),true);
 const {operationId,...body}=returnBody;assert.equal(validInstanceActionRequest(resource,{...request,body}),true);
 for(const reason of ['',null,'\u0085\u3000','a\0b','\ud800','🦊'.repeat(501)]){
  assert.equal(validWorkflowTransportRequest(actionPath,'POST',{...returnBody,reason}),false);
  assert.equal(validInstanceActionRequest(resource,{...request,body:{...body,reason}}),false);
 }
 assert.equal(returnBody.reason,originalReason);
});
test('P25 reject transport preserves optional legacy omission but bounds new reason and forbids agree reason',()=>{
 const path=actionPath.replace('/actions','/tasks/'+id(9)+'/actions');
 const b={operationId:id(50),action:'reject',basisToken:'a'.repeat(43),reason:originalReason};
 assert.equal(validWorkflowTransportRequest(path,'POST',b),true);
 assert.equal(validWorkflowTransportRequest(path,'POST',{...b,action:'agree'}),false);
 for(const reason of ['',null,'\u0085','a\0b','\udfff','🦊'.repeat(501)])assert.equal(validWorkflowTransportRequest(path,'POST',{...b,reason}),false);
 for(const reason of ['🦊'.repeat(500),'\ufeff'])assert.equal(validWorkflowTransportRequest(path,'POST',{...b,reason}),true);
 const {reason,...legacy}=b;assert.equal(validWorkflowTransportRequest(path,'POST',legacy),true);
});
test('P25 return success requires active rather than completed rejected or withdrawn state',()=>{
 const e={...expected,action:'return'};
 assert.equal(validWorkflowOperation({...success,instanceState:'active'},e,200,'GET'),true);
 for(const instanceState of ['withdrawn','completed','rejected'])assert.equal(validWorkflowOperation({...success,instanceState},e,200,'GET'),false);
});
test('P25 scoped return admits approved target and reason before assigning operation identity',()=>{
 const {operationId,...body}=returnBody;
 assert.equal(validInstanceActionRequest(resource,{...request,body}),true);
});
