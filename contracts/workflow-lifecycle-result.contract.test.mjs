import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import Ajv2020 from 'ajv/dist/2020.js';
// V045 accepted withdrawal/return outcomes, distinct from V037 task approval.
const api=JSON.parse(readFileSync(new URL('./openapi/openapi.json',import.meta.url)));
const ajv=new Ajv2020({strict:false,validateFormats:false});ajv.addSchema({...api,$id:'lifecycle-outcomes'});
const instance='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}/workflow-instances/{instanceId}';
const task=instance+'/tasks/{taskId}/actions',lookup='/api/v1/application-workflow-operations/{operationId}';
const validator=(path,method,status)=>ajv.compile({$ref:'lifecycle-outcomes'+api.paths[path][method].responses[status].content['application/json'].schema.$ref});
const id=n=>'eeeeeeee-eeee-4eee-8eee-'+String(n).padStart(12,'0');
const pending={operationId:id(1),commandId:id(2),instanceId:id(3),status:'pending'};
const withdrawn={...pending,status:'success',instanceState:'withdrawn',sequence:3};
const envelope=data=>({code:'OK',message:'',data,meta:null});
for(const [path,method]of [[instance+'/actions','post'],[lookup,'get']])test('V075 existing lifecycle withdrawn success is valid at '+method+' '+path,()=>assert.equal(validator(path,method,'200')(envelope(withdrawn)),true));
test('V075 task-only approval does not acquire a withdrawn outcome',()=>assert.equal(validator(task,'post','200')(envelope(withdrawn)),false));
test('V075 lifecycle return remains active and pending stays a separate 202 shape',()=>{
 assert.equal(validator(instance+'/actions','post','200')(envelope({...withdrawn,instanceState:'active'})),true);
 assert.equal(validator(instance+'/actions','post','202')(envelope(pending)),true);
 assert.equal(validator(instance+'/actions','post','202')(envelope(withdrawn)),false);
 assert.equal(validator(instance+'/actions','post','200')(envelope(pending)),false);
});
test('V075 common lookup retains all previous task states without overlapping alternatives',()=>{
 for(const state of ['active','completed','rejected','withdrawn'])assert.equal(validator(lookup,'get','200')(envelope({...withdrawn,instanceState:state})),true,state);
 assert.equal(validator(lookup,'get','200')(envelope(pending)),true);
 assert.equal(validator(lookup,'get','200')(envelope({...pending,status:'no_effect',instanceState:'unchanged',reason:'terminal_instance',sequence:0})),true);
});
for(const [name,value]of [['extra private data',{...withdrawn,values:{secret:1}}],['missing sequence',Object.fromEntries(Object.entries(withdrawn).filter(([k])=>k!=='sequence'))],['zero sequence',{...withdrawn,sequence:0}],['unsafe sequence',{...withdrawn,sequence:9007199254740992}],['invented state',{...withdrawn,instanceState:'cancelled'}]])test('V075 lifecycle final remains closed for '+name,()=>assert.equal(validator(instance+'/actions','post','200')(envelope(value)),false));
