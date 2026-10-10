// Real synthetic HTTPS captures, checked with standard JSON Schema 2020-12.
// The legacy local response helper does not support `not`/propertyNames and is
// deliberately left unchanged rather than silently dropping those assertions.
import {readFileSync} from 'node:fs';
import assert from 'node:assert/strict';
import Ajv2020 from 'ajv/dist/2020.js';
const api=JSON.parse(readFileSync('contracts/openapi/openapi.json','utf8'));
const validator=new Ajv2020({strict:false,validateFormats:false,coerceTypes:false,useDefaults:false,removeAdditional:false});validator.addSchema({...api,$id:'captured-private-presets'});
const file=process.argv[2];assert.ok(file,'Go JSONL capture path required');let count=0;
for(const line of readFileSync(file,'utf8').split('\n')){
 if(!line)continue;
 let record;try{record=JSON.parse(line);}catch{continue;}
 const text=record.Output??'';const marker='V071_CONTRACT_RESPONSE ';const index=text.indexOf(marker);if(index<0)continue;
 const capture=JSON.parse(text.slice(index+marker.length));
 assert.ok(['POST','GET'].includes(capture.method));assert.equal(capture.status,capture.method==='POST'?201:200);
 assert.match(capture.path,/^\/api\/v1\/applications\/[0-9a-f-]{36}\/forms\/[0-9a-f-]{36}\/table-presets(?:\/[0-9a-f-]{36})?$/);
 const path='/api/v1/applications/{appId}/forms/{viewId}/table-presets'+(capture.method==='GET'?'/{presetId}':'');
 const schema=api.paths[path][capture.method.toLowerCase()].responses[capture.status].content['application/json'].schema;
 const accepts=validator.compile({$ref:'captured-private-presets'+schema.$ref});
 assert.ok(accepts(capture.body),JSON.stringify(accepts.errors));assert.equal(capture.body.meta.requestId,capture.requestId);assert.ok(capture.requestId);count++;
}
assert.equal(count,2,'expected actual create and get response captures');console.log(`${count} actual HTTPS private preset responses passed published route schema`);
