import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
// V075 P26 and V015 complete visible-row field scope; no organisation directory.
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
test('P26 record-filter candidate GET is independent, bounded and read-only',()=>{
 const p=api.paths['/api/v1/applications/{appId}/forms/{viewId}/filter-candidates'];assert.ok(p,'missing read-only record-values endpoint');assert.deepEqual(Object.keys(p),['get']);const get=p.get;
 assert.deepEqual(get.security,[{WebSession:[]}]);assert.ok(get.parameters.some(x=>x.$ref==='#/components/parameters/ExpectedActor'));assert.equal(get.requestBody,undefined);
 const q=get.parameters.filter(x=>x.in==='query');assert.deepEqual(q.map(x=>x.name),['fieldId','q','pageSize','pageToken']);assert.equal(q[0].required,true);assert.equal(q[0].schema.format,'uuid');assert.equal(q[1].schema.maxLength,100);assert.deepEqual(q[2].schema,{type:'integer',minimum:1,maximum:50,default:20});assert.equal(q[3].schema.minLength,43);assert.equal(q[3].schema.maxLength,43);
 assert.equal(get.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/RecordFilterCandidatesEnvelope');for(const s of ['200','400','401','403','404','409','503'])assert.equal(get.responses[s].headers['Cache-Control'].schema.const,'no-store');
});
test('P26 candidate projection contains only current registry display and bounded cursor',()=>{
 const s=api.components.schemas,items=s.RecordFilterCandidates;assert.ok(items,'missing bounded display-only response');assert.equal(items.additionalProperties,false);assert.deepEqual(items.required,['items']);assert.deepEqual(Object.keys(items.properties),['items']);assert.equal(items.properties.items.maxItems,50);assert.equal(items.properties.items.items.$ref,'#/components/schemas/RecordReferenceDisplay');assert.deepEqual(Object.keys(s.RecordReferenceDisplay.properties),['id','label','deleted']);
 const envelope=s.RecordFilterCandidatesEnvelope;assert.ok(envelope);assert.equal(envelope.allOf[1].properties.data.$ref,'#/components/schemas/RecordFilterCandidates');assert.equal(envelope.allOf[1].properties.meta.properties.pagination.$ref,'#/components/schemas/CursorPagination');
});
