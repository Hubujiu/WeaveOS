import test from 'node:test';
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
