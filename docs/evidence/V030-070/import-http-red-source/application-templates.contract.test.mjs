import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));
const schemas=api.components.schemas;
test('V070 structure export and advisory preflight have closed guarded HTTP contracts',()=>{
 const path=api.paths['/api/v1/applications/{appId}/structure-template'];assert.ok(path,'export missing');
 assert.deepEqual(Object.keys(path).sort(),['get','head']);
 assert.equal(path.get.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/TemplateManifestEnvelope');
 assert.equal(path.head.responses['200'].content,undefined);
 const op=api.paths['/api/v1/application-templates/preflight']?.post;assert.ok(op,'preflight missing');
 assert.deepEqual(op.security,[{WebSession:[]}]);
 for(const name of ['ExpectedActor','CsrfToken'])assert.ok(op.parameters.some(p=>p.$ref===`#/components/parameters/${name}`));
 assert.equal(op['x-max-body-bytes'],4194304);
 assert.equal(op.requestBody.content['application/json'].schema.$ref,'#/components/schemas/TemplatePreflightRequest');
 assert.equal(schemas.TemplatePreflightRequest.additionalProperties,false);
 assert.deepEqual(schemas.TemplatePreflightRequest.required,['manifest','bindings']);
 assert.deepEqual(schemas.TemplateBinding.required,['kind','sourceId','targetId']);
 assert.equal(schemas.TemplateBinding.additionalProperties,false);
});
test('V070 full template excludes records owner and runtime artifacts at every resource boundary',()=>{
 const m=schemas.TemplateManifest;assert.ok(m,'full manifest missing');
 assert.equal(m.additionalProperties,false);assert.deepEqual(m.required,['format','version','application','directories','tables','forms','workflows','permissionGroups']);
 assert.equal(m.properties.format.const,'weaveos.structure-template');assert.equal(m.properties.version.const,1);
 for(const name of ['TemplateApplication','TemplateDirectory','TemplateTable','TemplateForm','TemplateWorkflow','TemplatePermissionGroup','TemplateGrant'])assert.equal(schemas[name].additionalProperties,false,name);
 assert.deepEqual(schemas.TemplateApplication.required,['id','name']);
 assert.deepEqual(schemas.TemplateWorkflow.required,['id','tableId','viewId','name','graph','allowWithdraw','triggers']);
 assert.deepEqual(schemas.TemplateCounts.required,['directories','tables','fields','forms','workflows','permissionGroups']);
 assert.equal(codes.APPLICATION_TEMPLATE_NOT_EXPORTABLE.httpStatus,409);assert.equal(codes.APPLICATION_TEMPLATE_TOO_LARGE.httpStatus,413);
});

import Ajv2020 from 'ajv/dist/2020.js';
const validator=new Ajv2020({strict:false,validateFormats:false});validator.addSchema({...api,$id:'template-api'});
const validates=name=>validator.compile({$ref:`template-api#/components/schemas/${name}`});
test('V070 input preserves approved field omission defaults while export is canonical',()=>{
 const id=n=>`00000000-0000-4000-8000-${String(n).padStart(12,'0')}`;
 const field={id:id(3),name:'Amount',kind:'money',required:false,default:'1.23',config:{},presentation:{helpText:null,displayTimeZone:null}};
 const manifest={format:'weaveos.structure-template',version:1,application:{id:id(1),name:'Empty'},directories:[],tables:[{id:id(2),name:'Amounts',directoryId:null,position:0,fields:[field]}],forms:[],workflows:[],permissionGroups:[]};
 const request=validates('TemplatePreflightRequest');assert.ok(request({manifest,bindings:[]}),JSON.stringify(request.errors));
 const output=validates('TemplateManifest');assert.equal(output(manifest),false,'export still requires normalized field config');
 field.config={precision:38,scale:2,roundingPlaces:2,roundingMode:'HALF_UP'};assert.ok(output(manifest),JSON.stringify(output.errors));
 
});

test('V070 existing graph permits explicit empty nonconditional edge branch',()=>{
 assert.ok(validates('TemplateEdge')({from:'00000000-0000-4000-8000-000000000001',to:'00000000-0000-4000-8000-000000000002',branch:''}));
});

test('V070 hot32 is pinned after reviewed round migrations without promoting deployment', async()=>{
 const {createHash}=await import('node:crypto');
 const compat=JSON.parse(readFileSync(new URL('../infra/server/deploy/compatibility.json',import.meta.url)));
 assert.equal(compat.backwardCompatible,false);
 const paths=compat.migrations.map(m=>m.path);
 const path='migrations/00032_application_template_import.sql';
 assert.equal(paths.filter(p=>p===path).length,1,'hot32 must be registered exactly once');
 assert.ok(paths.indexOf(path)>paths.indexOf('migrations/00031_workflow_round_operations.sql'));
 const raw=readFileSync(new URL('../db/'+path,import.meta.url));
 assert.equal(compat.migrations.find(m=>m.path===path).sha256,createHash('sha256').update(raw).digest('hex'));
});

test('V070 import exposes atomic 201 and exact operation recovery contract',()=>{
 const op=api.paths['/api/v1/application-templates/import']?.post;assert.ok(op,'atomic import HTTP missing');
 assert.deepEqual(op.security,[{WebSession:[]}]);
 assert.equal(op.requestBody.content['application/json'].schema.$ref,'#/components/schemas/TemplateImportRequest');
 assert.deepEqual(schemas.TemplateImportRequest.required,['operationId','manifest','bindings']);
 assert.equal(schemas.TemplateImportRequest.additionalProperties,false);
 assert.deepEqual(schemas.TemplateImportResult.required,['operationId','appId','policyRevision','structureVersion','status']);
 assert.equal(schemas.TemplateImportResult.additionalProperties,false);
 assert.equal(schemas.TemplateImportResult.properties.status.const,'imported');
 assert.equal(schemas.TemplateImportResult.properties.policyRevision.const,1);
 assert.equal(schemas.TemplateImportResult.properties.structureVersion.const,1);
 assert.equal(op.responses['201'].content['application/json'].schema.$ref,'#/components/schemas/TemplateImportEnvelope');
 assert.ok(op.responses['201'].headers.Location);
 assert.ok(op.responses['503']);
 assert.deepEqual(schemas.TemplateUnconfirmedData.required,['operationId']);
 assert.equal(schemas.TemplateUnconfirmedData.additionalProperties,false);
 for(const name of ['ExpectedActor','CsrfToken'])assert.ok(op.parameters.some(p=>p.$ref===`#/components/parameters/${name}`));
});
