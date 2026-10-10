import Ajv2020 from 'ajv/dist/2020.js';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
import assert from 'node:assert/strict';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const paths=['/api/v1/applications/{appId}/deletion','/api/v1/applications/{appId}/directories/{directoryId}/deletion','/api/v1/applications/{appId}/tables/{tableId}/deletion','/api/v1/applications/{appId}/forms/{viewId}/deletion'];
const ajv=new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'structure-deletion'});
const check=name=>ajv.compile({$ref:`structure-deletion#/components/schemas/${name}`});
const id='10000000-0000-4000-8000-000000000001', app='20000000-0000-4000-8000-000000000001';
test('V073 four noncascade commands expose Session CSRF actor and bounded closed request',()=>{
 for(const p of paths){const op=api.paths[p]?.post;assert.ok(op,p);assert.deepEqual(op.security,[{WebSession:[]}]);for(const name of ['CsrfToken','ExpectedActor'])assert.ok(op.parameters.some(x=>x.$ref===`#/components/parameters/${name}`));for(const status of ['200','400','401','403','404','409','415','503'])assert.ok(op.responses[status]);assert.match(op.description,/4096/);}
 for(const name of ['StructureDeletionRequest','ApplicationDeletionRequest','DirectoryDeletionRequest']){
  const valid=check(name);const input={operationId:id,expectedStructureVersion:0,expectedResourceVersion:name==='ApplicationDeletionRequest'?1:0};assert.equal(valid(input),true,JSON.stringify(valid.errors));
  for(const bad of [{...input,cascade:true},{...input,expectedStructureVersion:-1},{...input,expectedStructureVersion:9007199254740992},{...input,expectedResourceVersion:null},{...input,operationId:'00000000-0000-0000-0000-000000000000'}])assert.equal(valid(bad),false,JSON.stringify(bad));
  for(const key of Object.keys(input)){const bad={...input};delete bad[key];assert.equal(valid(bad),false,key);}
 }
 assert.equal(check('ApplicationDeletionRequest')({operationId:id,expectedStructureVersion:0,expectedResourceVersion:0}),false);
 assert.equal(check('DirectoryDeletionRequest')({operationId:id,expectedStructureVersion:0,expectedResourceVersion:1}),false);
});
test('V073 minimum six-key receipt is recoverable without scope or business disclosure',()=>{
 const valid=check('StructureDeletionResult');const result={operationId:id,appId:app,resourceKind:'form',id,structureVersion:1,deleted:true};assert.equal(valid(result),true);
 for(const bad of [{...result,values:{}},{...result,deleted:false},{...result,resourceKind:'workflow'},{...result,structureVersion:0},{...result,structureVersion:9007199254740992}])assert.equal(valid(bad),false);
 for(const key of Object.keys(result)){const bad={...result};delete bad[key];assert.equal(valid(bad),false);}
 const operation=check('ApplicationOperationEnvelope');const wrap=result=>({code:'OK',message:'success',meta:{requestId:'a'.repeat(32)},data:{operationId:id,status:'confirmed',httpStatus:200,location:'',result}});
 assert.equal(operation(wrap(result)),true,JSON.stringify(operation.errors));assert.equal(operation(wrap({...result,actorUserId:id})),false);
});
test('V073 dependency rejection is a finite bounded category array',()=>{
 const valid=check('StructureDeletionErrorEnvelope');const envelope=data=>({code:'APPLICATION_STRUCTURE_NOT_EMPTY',message:'blocked',meta:{requestId:'a'.repeat(32)},data});
 assert.equal(valid(envelope({dependencies:['records']})),true);
 for(const data of [{dependencies:['private-value']},{dependencies:['records'],records:[]},{dependencies:[]},{dependencies:['records','records']},{dependencies:['directories','tables','forms','permission_groups','workflows','drafts','pending_commands','grants','records']}])assert.equal(valid(envelope(data)),false,JSON.stringify(data));
 const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));assert.equal(codes.APPLICATION_STRUCTURE_NOT_EMPTY.httpStatus,409);
});
