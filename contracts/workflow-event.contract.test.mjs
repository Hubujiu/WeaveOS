import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const path='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-events/{eventId}';
const schemas=api.components.schemas;
test('historical event is session-bound body-free GET, never a write or raw engine endpoint',()=>{
 const p=api.paths[path];assert.ok(p,'historical event route missing');assert.deepEqual(Object.keys(p),['get']);
 const op=p.get;assert.deepEqual(op.security,[{WebSession:[]}]);assert.equal(op.requestBody,undefined);
 assert.ok(op.parameters.some(v=>v.$ref==='#/components/parameters/ExpectedActor'));
 assert.equal(op.parameters.some(v=>v.$ref==='#/components/parameters/CsrfToken'),false);
 for(const code of ['200','400','401','403','404','409','503'])assert.equal(op.responses[code].headers['Cache-Control'].schema.const,'no-store');
 assert.equal(op.responses['200'].content['application/json'].schema.$ref,'#/components/schemas/WorkflowEventResultEnvelope');
});
test('historical event and allowed field projection use closed exact DTOs',()=>{
 for(const [name,keys] of Object.entries({WorkflowEventResult:['event','basis'],WorkflowEventSummary:['flowName','flowNameSource','id','instanceId','flowId','nodeId','targetNodeId','actorId','action','outcome','sequence','schemaVersion','recordVersion','occurredAt'],WorkflowHistoricalField:['fieldId','fieldName','fieldKind','value','valueLabels'],WorkflowHistoricalLabel:['label','deleted']})){
  const s=schemas[name];assert.ok(s,name+' missing');assert.equal(s.additionalProperties,false);assert.deepEqual(Object.keys(s.properties).sort(),keys.toSorted());assert.deepEqual(s.required.toSorted(),keys.toSorted());
 }
 const event=schemas.WorkflowEventSummary;assert.deepEqual(event.properties.outcome.enum,['success','no_effect']);assert.deepEqual(event.properties.action.enum,['start','agree','reject','withdraw','return']);
 const variants=schemas.WorkflowEventBasis.oneOf;assert.equal(variants.length,2);
 const missing=variants.find(v=>v.properties.status.const==='unavailable');assert.ok(missing);assert.equal(missing.properties.fields.maxItems,0);
 const available=variants.find(v=>v.properties.status.const==='available');assert.equal(available.properties.fields.maxItems,1600);
});
