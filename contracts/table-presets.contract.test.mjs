import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
// Independent oracle: approved 2026-10-02 ADR008 A1–A3/A7.
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
const codes=JSON.parse(readFileSync('contracts/errors/codes.json','utf8'));
test('ADR008 personal table preset routes and three independent conflict codes',()=>{
 const base=api.paths['/api/v1/personnel/table-presets'];
 assert.ok(base?.get&&base?.post,'personal preset collection is missing');
 const item=api.paths['/api/v1/personnel/table-presets/{presetId}'];
 assert.ok(item?.get&&item?.put&&item?.delete,'CAS preset item routes are missing');
 assert.ok(base.post.responses['201']);assert.ok(item.delete.responses['204']);
 for(const code of ['PERSONNEL_PRESET_NAME_CONFLICT','PERSONNEL_PRESET_LIMIT_REACHED','PERSONNEL_PRESET_CONFLICT'])assert.equal(codes[code]?.httpStatus,409,code);
});
test('ADR008 preset DTOs isolate ownership and freeze independent capacities',()=>{
 const s=api.components.schemas;
 assert.ok(s.TablePresetCreateInput,'preset create DTO is absent');
 assert.deepEqual(s.TablePresetCreateInput.required,['view','name','hiddenColumnIds','schemaVersion']);
 assert.equal(s.TablePresetCreateInput.additionalProperties,false);
 assert.equal(s.TablePresetUpdateInput.properties.view,undefined);
 assert.equal(s.TablePresetCreateInput.properties.ownerId,undefined);
 assert.equal(s.TablePresetCreateInput['x-max-raw-body-bytes'],65536);
 assert.equal(s.TablePresetCreateInput['x-max-canonical-bytes'],32768);
 assert.equal(s.TablePresetList.properties.items.maxItems,20);
 assert.equal(s.TablePresetCreateInput.properties.name.maxLength,100);
 assert.equal(s.TablePresetCreateInput.properties.schemaVersion.const,1);
 assert.ok(s.TablePreset.required.includes('version'));
 assert.ok(s.TablePreset.required.includes('filter'));
});
