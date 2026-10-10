import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
test('V064 independent journal exposes original name and honest legacy provenance',()=>{
 const s=api.components.schemas.WorkflowEventSummary;
 assert.equal(s.additionalProperties,false);
 assert.equal(Object.keys(s.properties).length,14);
 assert.equal(s.required.length,14);
 assert.deepEqual(s.properties.flowName,{type:'string',minLength:1,maxLength:100});
 assert.deepEqual(s.properties.flowNameSource,{type:'string',enum:['captured','legacy_last_known']});
 assert.ok(s.required.includes('flowName'));assert.ok(s.required.includes('flowNameSource'));
});
test('V064 journal expansion is registered with its exact migration bytes',()=>{
 const file='migrations/00027_workflow_independent_journal.sql';
 const m=JSON.parse(readFileSync(new URL('../infra/server/deploy/compatibility.json',import.meta.url)));
 const entries=Object.values(m).filter(Array.isArray).flat();const entry=entries.find(e=>e.path===file);
 assert.ok(entry,'independent journal migration is not registered');
 assert.equal(entry.sha256,createHash('sha256').update(readFileSync(new URL('../db/'+file,import.meta.url))).digest('hex'));
});
