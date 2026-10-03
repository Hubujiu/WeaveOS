import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {test} from 'node:test';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const codes=JSON.parse(readFileSync(new URL('./errors/codes.json',import.meta.url)));
const schema=api.components.schemas;
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
});
