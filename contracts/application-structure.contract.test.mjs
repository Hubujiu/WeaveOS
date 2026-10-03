import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));
test('frozen application definition methods expose actual complete DTOs',()=>{
 assert.equal(api.openapi,'3.2.1');
 const base='/api/v1/applications/{appId}';
 for(const [suffix,methods] of Object.entries({'/structure':['get'],'/directories':['post'],'/directories/{directoryId}':['get','put'],'/tables':['post'],'/tables/{tableId}':['get','put'],'/forms':['post'],'/forms/{viewId}':['get','put'],'/forms/{viewId}/definition':['get','put'],'/forms/{viewId}/definition/preflight':['post']})){
  assert.ok(api.paths[base+suffix],`frozen path ${suffix} missing`);
  for(const method of methods) assert.ok(api.paths[base+suffix][method],`${method} ${suffix}`);
 }
 const schemas=api.components.schemas;
 assert.deepEqual(schemas.DefinitionInput.required,['expectedSchemaVersion','expectedViewVersion','fields','layout','optionMappings']);
 assert.deepEqual(schemas.DefinitionWrite.required,['operationId','expectedSchemaVersion','expectedViewVersion','fields','layout','optionMappings','confirmationToken']);
 assert.equal(schemas.DecimalConfigInput.additionalProperties,false);
 assert.equal(schemas.DecimalConfigInput.required,undefined,'decimal input keys are explicitly optional');
 assert.deepEqual(schemas.DecimalConfig.required,['precision','scale','roundingPlaces','roundingMode']);
 assert.equal(schemas.FormSource.oneOf.length,2,'atomic new_table and existing_table union');
 assert.ok(schemas.FormResult.required.includes('table'));
 assert.ok(schemas.LayoutNodeInput.oneOf.some(s=>s.properties?.span?.minimum===1&&s.properties.span.maximum===12));
 for(const name of ['Structure','Definition','Preflight','DefinitionSave','SchemaDependencyErrorData','SchemaImpactErrorData']) assert.ok(schemas[name]);
});
test('frozen structure errors have stable status and recoverability',()=>{
 for(const code of ['APPLICATION_STRUCTURE_CONFLICT','APPLICATION_SCHEMA_CONFLICT','APPLICATION_VIEW_CONFLICT','APPLICATION_SCHEMA_DEPENDENCY_BLOCKED','APPLICATION_SCHEMA_REQUIRED_BACKFILL','APPLICATION_SCHEMA_CONVERSION_FAILED','APPLICATION_SCHEMA_OPTION_MAPPING_REQUIRED','APPLICATION_SCHEMA_CONFIRMATION_REQUIRED','APPLICATION_SCHEMA_CONFIRMATION_STALE']) assert.equal(codes[code]?.httpStatus,409,code);
 assert.equal(codes.APPLICATION_OPERATION_UNCONFIRMED.httpStatus,503);
});
test('appendix A exposes minimal owner-only member candidates and actual group labels',()=>{const op=api.paths['/api/v1/applications/{appId}/member-candidates']?.get;assert.ok(op,'frozen candidate method missing');assert.deepEqual(op.parameters.filter(p=>p.in==='query').map(p=>p.name),['q','pageSize','pageToken']);const schemas=api.components.schemas;assert.deepEqual(schemas.MemberCandidate.required,['id','label','status']);assert.deepEqual(schemas.MemberCandidate.properties.status.enum,['active']);assert.ok(schemas.ApplicationMembers.required.includes('members'));assert.deepEqual(schemas.ApplicationMemberDisplay.required,['id','label','status','selectable']);assert.ok(schemas.Envelope.properties.meta.properties.pagination);});
test('appendix B marks optional negative actor constraint on every application method',()=>{assert.equal(codes.AUTH_SESSION_CHANGED?.httpStatus,409);for(const[path,item]of Object.entries(api.paths)){if(!path.startsWith('/api/v1/applications')&&!path.startsWith('/api/v1/application-operations/'))continue;for(const op of Object.values(item)){if(!op.responses)continue;assert.ok(op.parameters?.some(p=>p.$ref==='#/components/parameters/ExpectedActor'),`${path} negative guard header`);assert.ok(op.responses['409'],`${path} actor conflict`);}}assert.equal(api.components.parameters.ExpectedActor.required,false);});
test('all shared OpenAPI references resolve',()=>{function visit(v){if(!v||typeof v!=='object')return;if(v.$ref?.startsWith('#/')){let target=api;for(const p of v.$ref.slice(2).split('/')){target=target?.[p];assert.ok(target,`unresolved ${v.$ref}`);}}for(const child of Object.values(v))visit(child);}visit(api);});
test('appendix A.5 exposes minimal real department candidates',()=>{
 const op=api.paths['/api/v1/applications/{appId}/department-candidates']?.get;
 assert.ok(op,'accepted department candidate route missing');
 assert.deepEqual(op.parameters.filter(p=>p.in==='query').map(p=>p.name),['q','pageSize','pageToken']);
 const schema=api.components.schemas.DepartmentCandidate;
 assert.deepEqual(schema.required,['id','label','parentId','status']);
 assert.equal(schema.additionalProperties,false);
 assert.deepEqual(schema.properties.status.enum,['active']);
 assert.deepEqual(schema.properties.parentId.type,['string','null']);
 assert.ok(op.responses['403']);assert.ok(op.responses['409']);
});
