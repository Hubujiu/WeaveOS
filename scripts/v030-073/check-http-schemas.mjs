import {readFileSync} from 'node:fs';
import assert from 'node:assert/strict';
import Ajv2020 from 'ajv/dist/2020.js';
const api=JSON.parse(readFileSync(new URL('../../contracts/openapi/openapi.json',import.meta.url)));
const ajv=new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'actual-v073'});const validators=new Map(),counts={};
for(const line of readFileSync(process.argv[2],'utf8').split('\n')){
 let entry;try{entry=JSON.parse(line);}catch{continue;}
 const m=entry.Output?.match(/V073_SCHEMA (\w+) (.+)/);if(!m)continue;
 if(!validators.has(m[1]))validators.set(m[1],ajv.compile({$ref:`actual-v073#/components/schemas/${m[1]}`}));
 const valid=validators.get(m[1]);assert.equal(valid(JSON.parse(m[2])),true,JSON.stringify(valid.errors));counts[m[1]]=(counts[m[1]]??0)+1;
}
assert.ok(counts.StructureDeletionResult>=1);assert.ok(counts.StructureDeletionErrorEnvelope>=3);console.log(JSON.stringify({passed:true,actualResponses:counts}));
