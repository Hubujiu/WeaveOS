import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
import Ajv2020 from 'ajv/dist/2020.js';
const api = JSON.parse(readFileSync(new URL('../../contracts/openapi/openapi.json', import.meta.url)));
const ajv = new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'lifecycle'});
const counts={RecordLifecycleResult:0,RecordLifecycleState:0};
for(const line of readFileSync(process.argv[2],'utf8').split('\n')) {
 let row;try {row=JSON.parse(line)} catch {continue}
 const match=row.Output?.match(/V072_SCHEMA (RecordLifecycleResult|RecordLifecycleState) (.*)/);if(!match)continue;
 const validate=ajv.compile({$ref:`lifecycle#/components/schemas/${match[1]}`});
 assert.equal(validate(JSON.parse(match[2])),true,JSON.stringify(validate.errors));counts[match[1]]++;
}
assert.equal(counts.RecordLifecycleResult,2);assert.equal(counts.RecordLifecycleState,1);console.log(JSON.stringify({passed:true,counts}));
