import {readFileSync} from 'node:fs';
import assert from 'node:assert/strict';
import Ajv2020 from 'ajv/dist/2020.js';
const api=JSON.parse(readFileSync(new URL('../../contracts/openapi/openapi.json',import.meta.url)));
const ajv=new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'actual-v074'});const validators=new Map(),counts={},streams=new Map();
function inspect(line){
 const m=line.match(/V074_SCHEMA (\w+) (.+)/);if(!m)return;
 if(!validators.has(m[1]))validators.set(m[1],ajv.compile({$ref:`actual-v074#/components/schemas/${m[1]}`}));
 const valid=validators.get(m[1]);assert.equal(valid(JSON.parse(m[2])),true,JSON.stringify(valid.errors));counts[m[1]]=(counts[m[1]]??0)+1;
}
for(const line of readFileSync(process.argv[2],'utf8').split('\n')){
 let entry;try{entry=JSON.parse(line);}catch{continue;}
 if(typeof entry.Output!=='string')continue;
 const key=`${entry.Package}/${entry.Test??''}`;
 const lines=((streams.get(key)??'')+entry.Output).split('\n');streams.set(key,lines.pop());
 for(const complete of lines)inspect(complete);
}
for(const remainder of streams.values())assert.ok(!remainder.includes('V074_SCHEMA'),'truncated HTTP evidence');
for(const name of ['StructureCopyPlanEnvelope','StructureCopyPreflightEnvelope','StructureCopyEnvelope','StructureCopyErrorEnvelope'])assert.ok(counts[name]>=1,name);
console.log(JSON.stringify({passed:true,actualResponses:counts}));
