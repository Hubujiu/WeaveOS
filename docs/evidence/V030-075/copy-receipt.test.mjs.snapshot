import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
// V074 ADR/Data and V075 P22: one original operation, explicit new root, exact target increments.
const code=await readFile(new URL('../../apps/web/src/applications/templates/copyReceipt.ts',import.meta.url),'utf8');
const {validStructureCopyReceipt:valid}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(code,{mode:'transform'})).toString('base64'));
const op='e1e1e1e1-e1e1-41e1-81e1-e1e1e1e1e1e1',source='e2e2e2e2-e2e2-42e2-82e2-e2e2e2e2e2e2',target='e3e3e3e3-e3e3-43e3-83e3-e3e3e3e3e3e3',oldRoot='e4e4e4e4-e4e4-44e4-84e4-e4e4e4e4e4e4',newRoot='e5e5e5e5-e5e5-45e5-85e5-e5e5e5e5e5e5';
const context={operationId:op,source:{appId:source,kind:'form',id:oldRoot},targetAppId:target,expectedStructureVersion:4,expectedPolicyRevision:7,newGroupCount:0};
const receipt={operationId:op,appId:target,resourceKind:'form',id:newRoot,structureVersion:5,policyRevision:7,status:'copied'};
test('P22 copy accepts all four explicit scopes and exact independent target revision rules without mutation',()=>{
 const before=JSON.stringify([context,receipt]);assert.equal(valid(receipt,context),true);assert.equal(JSON.stringify([context,receipt]),before);
 for(const kind of ['directory','table','form']){
  assert.equal(valid({...receipt,resourceKind:kind},{...context,source:{...context.source,kind}}),true);
  assert.equal(valid({...receipt,resourceKind:kind,policyRevision:8},{...context,source:{...context.source,kind},newGroupCount:2}),true);
 }
 assert.equal(valid({...receipt,appId:source},{...context,targetAppId:source}),true);
 assert.equal(valid({...receipt,structureVersion:1},{...context,expectedStructureVersion:0}),true);
 const appContext={...context,source:{appId:source,kind:'application',id:source},targetAppId:null,expectedStructureVersion:0,expectedPolicyRevision:0};
 assert.equal(valid({...receipt,resourceKind:'application',id:target,structureVersion:1,policyRevision:1},appContext),true);
});
test('P22 copy rejects wrong original identity, copied scope, root reuse and unconfirmed or leaky envelopes',()=>{
 for(const value of [null,[],{}, {...receipt,operationId:target},{...receipt,appId:source},{...receipt,id:oldRoot},{...receipt,id:'00000000-0000-0000-0000-000000000000'},{...receipt,id:newRoot.toUpperCase()},{...receipt,resourceKind:'table'},{...receipt,status:'pending'},{...receipt,status:'enabled'},{...receipt,manifest:{}},{...receipt,values:{}},{...receipt,valid:true}])assert.equal(valid(value,context),false,JSON.stringify(value));
 for(const key of Object.keys(receipt)){const value={...receipt};delete value[key];assert.equal(valid(value,context),false,key);}
});
test('P22 copy cannot silently change target CAS or infer policy changes from general success',()=>{
 for(const patch of [{structureVersion:4},{structureVersion:6},{policyRevision:8},{policyRevision:0},{structureVersion:'5'},{structureVersion:5.1},{structureVersion:Number.MAX_SAFE_INTEGER+1}])assert.equal(valid({...receipt,...patch},context),false,JSON.stringify(patch));
 assert.equal(valid(receipt,{...context,newGroupCount:1}),false);
 assert.equal(valid({...receipt,structureVersion:Number.MAX_SAFE_INTEGER},{...context,expectedStructureVersion:Number.MAX_SAFE_INTEGER-1}),true);
 assert.equal(valid({...receipt,structureVersion:Number.MAX_SAFE_INTEGER+1},{...context,expectedStructureVersion:Number.MAX_SAFE_INTEGER}),false);
 assert.equal(valid({...receipt,policyRevision:Number.MAX_SAFE_INTEGER+1},{...context,newGroupCount:1,expectedPolicyRevision:Number.MAX_SAFE_INTEGER}),false);
 for(const patch of [{operationId:'bad'},{targetAppId:null},{expectedStructureVersion:-1},{expectedPolicyRevision:0},{newGroupCount:-1},{newGroupCount:129},{newGroupCount:0.5}])assert.equal(valid(receipt,{...context,...patch}),false);
});
test('P22 application copy binds a new application identity and fixed initial versions',()=>{
 const c={...context,source:{appId:source,kind:'application',id:source},targetAppId:null,expectedStructureVersion:0,expectedPolicyRevision:0},r={...receipt,resourceKind:'application',id:target,structureVersion:1,policyRevision:1};
 assert.equal(valid(r,c),true);
 for(const patch of [{appId:source,id:source},{id:newRoot},{structureVersion:2},{policyRevision:2}])assert.equal(valid({...r,...patch},c),false);
 for(const patch of [{targetAppId:target},{expectedStructureVersion:1},{expectedPolicyRevision:1},{source:{appId:source,kind:'application',id:oldRoot}}])assert.equal(valid(r,{...c,...patch}),false);
});
