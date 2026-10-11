import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import Ajv2020 from 'ajv/dist/2020.js';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const ajv=new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'copy-contract'});
const validator=name=>{assert.ok(api.components.schemas[name],`missing ${name}`);return ajv.compile({$ref:`copy-contract#/components/schemas/${name}`});};
const id='10000000-0000-4000-8000-000000000001',app='20000000-0000-4000-8000-000000000001';
const plan={source:{appId:app,kind:'form',id},target:{appId:app,directoryId:null,name:'Copy'}};
const pre={...plan,expectedSourceFingerprint:'a'.repeat(64),expectedTargetStructureVersion:0,expectedTargetPolicyRevision:1,bindings:[],groupMappings:[]};
test('V074 three commands register Session CSRF actor closed contracts',()=>{
 for(const action of ['plan','preflight','execute']){const op=api.paths[`/api/v1/structure-copies/${action}`]?.post;assert.ok(op,action);assert.deepEqual(op.security,[{WebSession:[]}]);assert.equal(op['x-max-body-bytes'],4194304);for(const p of ['CsrfToken','ExpectedActor'])assert.ok(op.parameters.some(x=>x.$ref===`#/components/parameters/${p}`));for(const code of ['400','401','403','404','405','409','413','415','503'])assert.ok(op.responses[code]);assert.ok(op.responses[action==='execute'?'201':'200']);}
});
test('V074 closed scopes and explicit mappings retain null target semantics',()=>{
 for(const [name,input] of [['StructureCopyPlanRequest',plan],['StructureCopyPreflightRequest',pre],['StructureCopyExecuteRequest',{...pre,operationId:id}]]){
  const valid=validator(name);assert.equal(valid(input),true,JSON.stringify(valid.errors));
  for(const key of Object.keys(input)){const bad=structuredClone(input);delete bad[key];assert.equal(valid(bad),false,key);}
  assert.equal(valid({...input,records:[]}),false);
 }
 const valid=validator('StructureCopyPlanRequest');
 assert.equal(valid({source:{appId:app,kind:'application',id:app},target:{appId:null,directoryId:null,name:'App'}}),true);
 for(const bad of [{...plan,target:{...plan.target,appId:null}},{...plan,source:{...plan.source,kind:'workflow'}},{...plan,target:{...plan.target,name:'a\u0000b'}},{...plan,target:{...plan.target,name:''}},{...plan,source:{...plan.source,id:'00000000-0000-0000-0000-000000000000'}}])assert.equal(valid(bad),false);
 const mapped=validator('StructureCopyPreflightRequest');for(const bad of [{...pre,bindings:null},{...pre,groupMappings:null},{...pre,expectedSourceFingerprint:'A'.repeat(64)},{...pre,expectedTargetPolicyRevision:null},{...pre,expectedTargetStructureVersion:9007199254740992},{...pre,groupMappings:[{sourceGroupId:id,name:'New',targetGroupId:app}]}])assert.equal(mapped(bad),false);
});
test('V074 seven-key minimum recovery and source error never expose configuration',()=>{
 const result={operationId:id,appId:app,resourceKind:'form',id,structureVersion:1,policyRevision:1,status:'copied'};
 const valid=validator('StructureCopyResult');assert.equal(valid(result),true);
 for(const key of Object.keys(result)){const bad={...result};delete bad[key];assert.equal(valid(bad),false);}
 for(const bad of [{...result,manifest:{}},{...result,status:'imported'},{...result,policyRevision:0},{...result,resourceKind:'flow'}])assert.equal(valid(bad),false);
 const operation=validator('ApplicationOperationEnvelope');assert.equal(operation({code:'OK',message:'success',meta:{requestId:'a'.repeat(32)},data:{operationId:id,status:'confirmed',httpStatus:201,location:'/api/v1/applications/'+app+'/forms/'+id,result}}),true,JSON.stringify(operation.errors));
 const error=validator('StructureCopyErrorEnvelope');const envelope=data=>({code:'APPLICATION_COPY_SOURCE_CHANGED',message:'changed',meta:{requestId:'a'.repeat(32)},data});assert.equal(error(envelope(null)),true);assert.equal(error(envelope({manifest:{}})),false);
 const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));assert.equal(codes.APPLICATION_COPY_SOURCE_CHANGED.httpStatus,409);
});
