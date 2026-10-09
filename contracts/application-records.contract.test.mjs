import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));
const schema=api.components.schemas;
test('ADR14 reference candidates bind field action record and live authority',()=>{
 const op=api.paths['/api/v1/applications/{appId}/forms/{viewId}/reference-candidates']?.get;
 assert.ok(op,'field-scoped active candidates route missing');
 const query=op.parameters.filter(p=>p.in==='query');
 assert.deepEqual(query.map(p=>p.name),['fieldId','action','recordId','q','pageSize','pageToken']);
 assert.deepEqual(query.find(p=>p.name==='action').schema.enum,['create','edit']);
 assert.equal(query.find(p=>p.name==='fieldId').required,true);
 assert.equal(query.find(p=>p.name==='pageSize').schema.maximum,50);
 assert.ok(op.parameters.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));
});
test('ADR14 draft write minimum and history safe displays are explicit',()=>{
 const base='/api/v1/applications/{appId}/forms/{viewId}';
 assert.deepEqual(schema.DraftMutationResult?.required,['operationId','id','draftVersion']);
 assert.equal(api.paths[base+'/drafts'].post.responses['201'].content['application/json'].schema.$ref,'#/components/schemas/DraftMutationResultEnvelope');
 assert.equal(api.paths[base+'/drafts/{draftId}'].patch.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/DraftMutationResultEnvelope');
 assert.ok(schema.RuntimeFieldAccess.required.includes('history'));
 assert.ok(schema.RuntimeCapabilities.required.includes('history'));
 assert.ok(schema.RecordHistoryChange.required.includes('fieldKind'));
 for(const field of ['fieldLabel','fieldDeleted','valueLabels'])assert.ok(schema.RecordHistoryChange.required.includes(field));
});
test('frozen record draft runtime and history methods are exact and actor guarded',()=>{
 const base='/api/v1/applications/{appId}/forms/{viewId}';
 for(const[suffix,methods]of Object.entries({'/runtime':['get'],'/records':['post'],'/records/{recordId}':['get','patch'],'/records/search':['post'],'/drafts':['get','post'],'/drafts/{draftId}':['get','patch','delete'],'/records/{recordId}/history':['get']})){
  for(const method of methods){const op=api.paths[base+suffix]?.[method];assert.ok(op,`${method} ${suffix} missing`);assert.ok(op.parameters?.some(p=>p.$ref==='#/components/parameters/ExpectedActor'));}
 }
 assert.equal(api.paths[base+'/records/{recordId}'].put,undefined);
 assert.equal(api.paths[base+'/drafts/{draftId}'].put,undefined);
 assert.equal(api.paths[base+'/drafts/{draftId}'].delete.responses['204'].content,undefined,'external204 has no body');
});
test('frozen record payloads and confirmations contain precise allowed values',()=>{
 for(const name of ['RecordCreate','RecordEdit','RecordMutationResult','RecordValues','BusinessRecord','DraftUpdate','RecordDraft','RecordDraftSummary','RecordSearch','RecordQuickSearch'])assert.ok(schema[name],`frozen ${name} missing`);
 assert.deepEqual(schema.RecordCreate.required,['operationId','expectedSchemaVersion','values']);
 assert.deepEqual(schema.RecordEdit.required,['operationId','expectedSchemaVersion','expectedRecordVersion','changes']);
 assert.deepEqual(schema.RecordMutationResult.required,['operationId','id','recordVersion','schemaVersion','createdAt','updatedAt']);
 assert.equal(schema.RecordMutationResult.properties.values,undefined);
 assert.equal(schema.RecordCreate.properties.values.$ref,'#/components/schemas/RecordValues');
 assert.ok(schema.BusinessRecord.required.includes('referenceDisplays'));
 assert.deepEqual(schema.DraftUpdate.required,['operationId','expectedDraftVersion','changes','removeFieldIds']);
 assert.ok(schema.RecordDraft.required.includes('conflicts'));
 assert.ok(schema.RecordDraftSummary.required.includes('hasConflicts'));
 assert.equal(schema.RecordSearch.properties.quickSearch.$ref,'#/components/schemas/RecordQuickSearch');
 assert.equal(schema.RecordQuickSearch.type,'object');
 assert.equal(schema.RecordQuickSearch.properties.fieldIds.uniqueItems,true);
});
test('runtime and history expose only frozen safe DTO and record errors',()=>{
 for(const name of ['RuntimeField','RecordReferenceDisplay','RecordHistoryEvent','ApplicationDataGrant'])assert.ok(schema[name],`frozen ${name} missing`);
 assert.ok(schema.RuntimeField.properties.input.properties.timePrecision.enum.includes('minute'));
 assert.equal(schema.RuntimeField.properties.config,undefined);
 assert.deepEqual(schema.RecordReferenceDisplay.required,['id','label','deleted']);
 assert.deepEqual(schema.RecordHistoryEvent.required,['id','recordVersionBefore','recordVersionAfter','actorId','occurredAt','origin','changes']);
 for(const code of ['APPLICATION_SCHEMA_NOT_READY','APPLICATION_RECORD_CONFLICT','APPLICATION_RECORD_FENCED','APPLICATION_QUERY_CHANGED','APPLICATION_QUERY_CONTEXT_EXPIRED','APPLICATION_DRAFT_CONFLICT','APPLICATION_DRAFT_BASE_CONFLICT'])assert.equal(codes[code]?.httpStatus,409,code);
 assert.equal(schema.ApplicationDataGrant.properties.resourceKind.const,'form');
 assert.ok(schema.ApplicationDataGrant.properties.action.enum.includes('data.history'));
 assert.ok(schema.SchemaDependency.properties.kind.enum.includes('data_grant'),'field removal must report the actual grant dependency');
});
test('FLOW07 ordinary edits of in-flight records have a registered read-only conflict',()=>{
 assert.equal(codes.WORKFLOW_RECORD_READ_ONLY?.httpStatus,409);
 assert.equal(codes.WORKFLOW_RECORD_READ_ONLY?.public,true);
 assert.ok(schema.RecordErrorEnvelope.allOf[1].properties.code.enum.includes('WORKFLOW_RECORD_READ_ONLY'));
 const op=api.paths['/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}'].patch;
 assert.match(op.responses['409'].description,/WORKFLOW_RECORD_READ_ONLY/);
});
