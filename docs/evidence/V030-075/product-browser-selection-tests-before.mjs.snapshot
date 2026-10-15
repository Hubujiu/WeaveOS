import test from 'node:test';
import assert from 'node:assert/strict';
import {verifySelection} from '../../scripts/check-ci-selection.mjs';
const sha='a'.repeat(40);
const ci=['go','browser','workflow-engine','workflow-rpc','execution-rpc','workflow-recovery','workflow-actions','workflow-formal-runtime','workflow-schema-cli','workflow-backup'];
const product=['components','component-coverage','product'];
function fixture(kind,flags){
 const p={schemaVersion:1,sourceSha:sha,...flags,reasons:['fixture']};
 const n={preflight:{result:'success',outputs:{plan:JSON.stringify(p)}},...(kind==='ci'?{governance:{result:'success'}}:{})};
 for(const id of kind==='ci'?ci:product){const selected=flags[kind==='product'?'product':id==='browser'?'browser':'backend'];n[id]={result:selected?'success':'skipped'};}
 return n;
}
for(const kind of ['ci','product'])for(const flags of [{backend:false,browser:false,product:false},{backend:true,browser:false,product:false},{backend:false,browser:true,product:true},{backend:true,browser:true,product:true}])test(`V059 selected exact successes accepted ${kind} ${JSON.stringify(flags)}`,()=>assert.doesNotThrow(()=>verifySelection({kind,needs:fixture(kind,flags),sourceSha:sha})));
for(const kind of ['ci','product'])for(const id of ['preflight',...(kind==='ci'?['governance',...ci]:product)])for(const result of ['failure','cancelled','skipped',undefined])test(`V059 rejects ${kind}/${id} ${result}`,()=>{
 const needs=fixture(kind,{backend:true,browser:true,product:true});needs[id]={...needs[id],result};assert.throws(()=>verifySelection({kind,needs,sourceSha:sha}));
});
for(const mutate of [n=>delete n.go,n=>n.unreviewed={result:'success'},n=>n.preflight.outputs.plan='',n=>n.preflight.outputs.plan='{}',n=>{const p=JSON.parse(n.preflight.outputs.plan);p.backend='false';n.preflight.outputs.plan=JSON.stringify(p);},n=>{const p=JSON.parse(n.preflight.outputs.plan);p.sourceSha='b'.repeat(40);n.preflight.outputs.plan=JSON.stringify(p);},n=>n.browser.result='failure'])test('V059 malformed, stale or failed unselected results cannot pass',()=>{
 const needs=fixture('ci',{backend:false,browser:false,product:false});mutate(needs);assert.throws(()=>verifySelection({kind:'ci',needs,sourceSha:sha}));
});
test('V059 unknown workflow kind is refused',()=>assert.throws(()=>verifySelection({kind:'unknown',needs:{},sourceSha:sha})));
