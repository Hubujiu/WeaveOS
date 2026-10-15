import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
const data=s=>'data:text/javascript;base64,'+Buffer.from(s).toString('base64');
async function load(url){let code=stripTypeScriptTypes(await readFile(url,'utf8'),{mode:'transform'});for(const m of [...code.matchAll(/from ['"]([^'"]+)['"]/g)])code=code.replace(m[0],`from '${await load(new URL(m[1]+'.ts',url))}'`);return data(code);}
const {validTemplateImportRequest}=await import(await load(new URL('../../apps/web/src/applications/templates/importRequest.ts',import.meta.url)));
const id=n=>'e7e7e7e7-e7e7-4e7e-8e7e-'+String(n).padStart(12,'0');
const resource={kind:'template-import'},manifest={format:'weaveos.structure-template',version:1,application:{id:id(1),name:'模板'},directories:[],tables:[],forms:[],workflows:[],permissionGroups:[]};
const request={resource,path:'application-templates/import',method:'POST',expectedStatus:201,body:{manifest,bindings:[]}};
test('V070 template import is a global creation resource with exact original body and no fake target app',()=>{const before=structuredClone(request);assert.equal(validTemplateImportRequest(resource,request),true);assert.deepEqual(request,before);});
test('V070 explicit typed binding accepts a same-ID mapping only when actually supplied',()=>{assert.equal(validTemplateImportRequest(resource,{...request,body:{manifest,bindings:[{kind:'user',sourceId:id(2),targetId:id(2)},{kind:'department',sourceId:id(2),targetId:id(3)}]}}),true);});
test('V070 exact request refuses target IDs, wrong route/method/status and caller operation keys',()=>{for(const r of [null,{kind:'record'},{...resource,appId:id(1)},{...resource,creationNonce:id(9)}])assert.equal(validTemplateImportRequest(r,request),false);for(const edit of [{path:'application-templates/preflight'},{path:'https://example.invalid/import'},{path:'application-templates/import?x=1'},{method:'PUT'},{expectedStatus:200},{body:{...request.body,operationId:id(3)}}])assert.equal(validTemplateImportRequest(resource,{...request,...edit}),false);});
test('V070 import rejects non-array, duplicate, untyped and incomplete bindings without discarding them',()=>{const b={kind:'user',sourceId:id(2),targetId:id(3)};for(const bindings of [null,{},[b,b],[{...b,kind:'any'}],[{...b,targetId:'bad'}],[{...b,sourceId:'00000000-0000-0000-0000-000000000000'}],[{...b,label:'extra'}],[{kind:'user',sourceId:id(2)}]])assert.equal(validTemplateImportRequest(resource,{...request,body:{manifest,bindings}}),false);});
test('V070 import rejects incomplete or oversized manifest instead of silently dropping structures',()=>{for(const m of [null,{}, {...manifest,records:[]},{...manifest,application:{id:id(1),name:'x'.repeat(1048576)}}])assert.equal(validTemplateImportRequest(resource,{...request,body:{manifest:m,bindings:[]}}),false);});
