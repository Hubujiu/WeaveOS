import test from 'node:test';
import {createHash} from 'node:crypto';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const schemas=api.components.schemas;
test('V044: definition triggers are optional on save and required on read',()=>{
 assert.equal(schemas.WorkflowDefinitionSave.properties.triggers?.$ref,'#/components/schemas/WorkflowTriggers');
 assert.equal(schemas.WorkflowDefinitionSave.required.includes('triggers'),false);
 assert.equal(schemas.WorkflowDefinition.properties.triggers?.$ref,'#/components/schemas/WorkflowTriggers');
 assert.ok(schemas.WorkflowDefinition.required.includes('triggers'));
});
test('V044: closed event-condition configuration has three bounded event kinds',()=>{
 const list=schemas.WorkflowTriggers;assert.ok(list);assert.equal(list.type,'array');assert.equal(list.maxItems,3);
 assert.equal(list.items.$ref,'#/components/schemas/WorkflowTrigger');
 const item=schemas.WorkflowTrigger;assert.equal(item.additionalProperties,false);
 assert.deepEqual([...item.required].sort(),['condition','event']);
 assert.deepEqual([...item.properties.event.enum].sort(),['manual','record.created','record.updated']);
 assert.deepEqual(item.properties.condition.anyOf,[{$ref:'#/components/schemas/RecordFilterGroup'},{type:'null'}]);
});

test('V044: migration 24 is registered once after 23 with exact immutable bytes',()=>{
 const manifest=JSON.parse(readFileSync(new URL('../infra/server/deploy/compatibility.json',import.meta.url)));
 const path='migrations/00024_workflow_trigger_configuration.sql';
 const matches=manifest.migrations.filter(item=>item.path===path);
 assert.equal(matches.length,1,'migration 24 must be registered exactly once');
 assert.equal(matches[0].sha256,createHash('sha256').update(readFileSync(new URL('../db/'+path,import.meta.url))).digest('hex'));
 assert.ok(manifest.migrations.findIndex(item=>item.path===path)>manifest.migrations.findIndex(item=>item.path==='migrations/00023_workflow_record_order_index.sql'));
});
