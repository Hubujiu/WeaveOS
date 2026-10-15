import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const code=await readFile(new URL('../../apps/web/src/applications/templates/copyRequest.ts',import.meta.url),'utf8');
const {validStructureCopyPlanRequest:plan,validStructureCopyPreflightRequest:pre,validStructureCopyRequest:execute}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(code,{mode:'transform'})).toString('base64'));
const id=n=>'cacacaca-caca-4aca-8aca-'+String(n).padStart(12,'0');
const p={source:{appId:id(1),kind:'form',id:id(2)},target:{appId:id(3),directoryId:null,name:' 表单副本 '}};
const v={...p,expectedSourceFingerprint:'a'.repeat(64),expectedTargetStructureVersion:0,expectedTargetPolicyRevision:1,bindings:[],groupMappings:[]};
const resource={kind:'structure-copy',sourceAppId:id(1),sourceKind:'form',sourceId:id(2)};
const req={resource,path:'structure-copies/execute',method:'POST',expectedStatus:201,body:v};
test('P22 plans preserve four source namespaces, explicit new-application nulls and names without mutation',()=>{
 const before=JSON.stringify(p);assert.equal(plan(p),true);assert.equal(JSON.stringify(p),before);
 for(const kind of ['directory','table','form'])for(const directoryId of [null,id(4)])assert.equal(plan({...p,source:{...p.source,kind},target:{...p.target,directoryId}}),true);
 assert.equal(plan({source:{appId:id(1),kind:'application',id:id(1)},target:{appId:null,directoryId:null,name:'新应用'}}),true);
 for(const name of ['🦊'.repeat(100),'\u0085'+ '字'.repeat(100)+'\u0085','\uFEFF'])assert.equal(plan({...p,target:{...p.target,name}}),true);
 for(const name of ['', ' \t\r\n\u0085', '🦊'.repeat(101),'a\0b','\uD800'])assert.equal(plan({...p,target:{...p.target,name}}),false);
});
test('P22 closed plan rejects wrong scope, omitted nullable keys and extra data',()=>{
 for(const bad of [null,{}, {...p,records:[]},{...p,source:{...p.source,kind:'workflow'}},{...p,source:{...p.source,id:'00000000-0000-0000-0000-000000000000'}},{...p,target:{...p.target,appId:null}},{...p,target:{appId:id(3),name:'x'}},{...p,target:{...p.target,directoryId:'bad'}},{...p,source:{...p.source,extra:true}},{...p,source:{...p.source,kind:'application'},target:{appId:null,directoryId:null,name:'x'}}])assert.equal(plan(bad),false);
});
test('P22 preflight retains explicit typed mappings and original target CAS without inventing mapping coverage',()=>{
 const user={kind:'user',sourceId:id(8),targetId:id(8)},department={kind:'department',sourceId:id(8),targetId:id(9)};
 const input={...v,bindings:[user,department],groupMappings:[{sourceGroupId:id(7),name:'新组'}]},before=JSON.stringify(input);assert.equal(pre(input),true);assert.equal(JSON.stringify(input),before);
 assert.equal(pre({...v,source:{appId:id(1),kind:'application',id:id(1)},target:{appId:null,directoryId:null,name:'副本'},expectedTargetPolicyRevision:0}),true);
 for(const edit of [{expectedSourceFingerprint:'A'.repeat(64)},{expectedTargetStructureVersion:-1},{expectedTargetStructureVersion:9007199254740992},{expectedTargetPolicyRevision:0},{bindings:null},{bindings:[user,user]},{bindings:[{...user,kind:'role'}]},{bindings:[{...user,targetId:'bad'}]},{bindings:[{...user,label:'x'}]},{groupMappings:null},{groupMappings:[{sourceGroupId:id(7),name:'x'},{sourceGroupId:id(7),name:'y'}]},{groupMappings:[{sourceGroupId:id(7),name:'x',targetGroupId:id(6)}]},{groupMappings:Array.from({length:129},(_,i)=>({sourceGroupId:id(i+10),name:'组'}))},{operationId:id(10)}])assert.equal(pre({...v,...edit}),false,JSON.stringify(edit));
});
test('P22 execute requires exact original source resource and endpoint and never modifies input',()=>{
 const before=JSON.stringify(req);assert.equal(execute(resource,req),true);assert.equal(JSON.stringify(req),before);
 for(const edit of [{kind:'template-import'},{...resource,sourceId:id(9)},{...resource,sourceKind:'table'},{...resource,sourceAppId:id(9)},{...resource,appId:id(3)}])assert.equal(execute(edit,req),false);
 for(const edit of [{path:'structure-copies/preflight'},{path:'structure-copies/execute?x=1'},{method:'PUT'},{expectedStatus:200},{body:{...v,operationId:id(9)}},{body:{...v,bindings:Array.from({length:40000},(_,i)=>({kind:'user',sourceId:id(i+100),targetId:id(9)}))}}])assert.equal(execute(resource,{...req,...edit}),false);
});
