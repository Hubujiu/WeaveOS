import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import Ajv2020 from 'ajv/dist/2020.js';
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
const codes=JSON.parse(readFileSync('contracts/errors/codes.json','utf8'));
const s=api.components.schemas;
const validator=new Ajv2020({strict:false,validateFormats:false});validator.addSchema({...api,$id:'private-presets'});
const compile=name=>{assert.ok(s[name],`approved schema ${name} missing`);return validator.compile({$ref:`private-presets#/components/schemas/${name}`});};
const id=n=>`00000000-0000-4000-8000-${String(n).padStart(12,'0')}`;
const state={name:'方案',filter:null,sort:null,hiddenColumnIds:[],columnOrder:[],columnWidths:{}};
const base='/api/v1/applications/{appId}/forms/{viewId}/table-presets';
test('V071 private view routes require trusted session actor and write CSRF',()=>{
 for(const [suffix,methods] of [['',['get','head','post']],['/{presetId}',['get','head','put','delete']]]){
  for(const method of methods){const op=api.paths[base+suffix]?.[method];assert.ok(op,`${method} route missing`);assert.deepEqual(op.security,[{WebSession:[]}]);assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));if(!['get','head'].includes(method))assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/CsrfToken'));}
 }
 const item=api.paths[base+'/{presetId}'];assert.equal(item.delete.responses['204'].content,undefined);assert.equal(item.head.responses['200'].content,undefined);
 assert.deepEqual(item.delete.parameters.filter(p=>p.in==='query').map(p=>p.name).sort(),['expectedVersion','operationId']);
 assert.ok(api.paths[base].post.responses['201'].headers.Location);
 for(const code of ['APPLICATION_PRESET_NAME_CONFLICT','APPLICATION_PRESET_LIMIT_REACHED','APPLICATION_PRESET_CONFLICT'])assert.equal(codes[code]?.httpStatus,409);
});
test('V071 closed configuration distinguishes editable trees and unsafe display input',()=>{
 const accepts=compile('ApplicationPresetState');assert.ok(accepts(state),JSON.stringify(accepts.errors));
 const leaf={fieldId:id(1),operator:'eq',value:'literal'};
 const group={operator:'and',children:[leaf]};
 for(const filter of [group,{operator:'or',children:[group]}])assert.ok(accepts({...state,filter}),JSON.stringify(accepts.errors));
 for(const delta of [{ownerId:id(2)},{hiddenColumnIds:[id(1),id(1)]},{columnOrder:['selection']},{columnWidths:{[id(1)]:0}},{columnWidths:{[id(1)]:1.25}},{columnWidths:{[id(1)]:9007199254740992}},{filter:{operator:'or',children:[leaf]}},{filter:{operator:'and',children:[]}},{filter:{operator:'and',children:[group]}}])assert.equal(accepts({...state,...delta}),false,JSON.stringify(delta));
 assert.equal(s.ApplicationPresetCreate['x-max-raw-body-bytes'],65536);assert.equal(s.ApplicationPresetState['x-max-canonical-bytes'],32768);assert.equal(s.ApplicationPresetFilter['x-max-canonical-bytes'],16384);
 assert.equal(s.ApplicationPresetList.properties.items.maxItems,20);
});
test('V071 invalid item schema cannot expose original operands fields or settings',()=>{
 const accepts=compile('ApplicationPresetItem');
 const invalid={id:id(1),name:'失效方案',version:3,invalid:true,reason:'PERMISSION_CHANGED'};
 assert.ok(accepts(invalid),JSON.stringify(accepts.errors));
 for(const extra of [{filter:{secret:'operand'}},{columnOrder:[id(2)]},{columnWidths:{[id(2)]:123}},{createdAt:'2026-10-10T00:00:00Z'},{appId:id(2)},{reason:'internal SQL'}])assert.equal(accepts({...invalid,...extra}),false,JSON.stringify(extra));
 const valid={...state,id:id(1),appId:id(2),viewId:id(3),version:1,invalid:false,createdAt:'2026-10-10T00:00:00Z',updatedAt:'2026-10-10T00:00:00Z'};assert.ok(accepts(valid),JSON.stringify(accepts.errors));
 assert.equal(accepts({...valid,invalid:true,reason:'FIELD_UNAVAILABLE'}),false,'cannot retain state when invalid');
});
test('V071 minimum receipt survives operation recovery without broadening old result DTOs',()=>{
 const accepts=compile('ApplicationOperationEnvelope');const receipt={operationId:id(1),id:id(2),version:1};
 const envelope={code:'OK',message:'success',meta:{requestId:'a'.repeat(32)},data:{operationId:id(1),status:'confirmed',httpStatus:201,location:'/api/v1/applications/'+id(3)+'/forms/'+id(4)+'/table-presets/'+id(2),result:receipt}};
 assert.ok(accepts(envelope),JSON.stringify(accepts.errors));
 assert.equal(accepts({...envelope,data:{...envelope.data,result:{...receipt,filter:{}}}}),false,'recovery cannot leak private config');
});
