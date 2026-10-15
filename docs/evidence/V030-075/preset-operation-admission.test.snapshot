import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// Only synchronous request admission is under test. React's storage and the
// unresolved session transport are external stubs; no lifecycle claims.
const source=await readFile(new URL('../../apps/web/src/applications/useApplicationOperation.ts',import.meta.url),'utf8');
const javascript=stripTypeScriptTypes(source,{mode:'transform'}).replace(/^import .*;\n/gm,'');
const setup=`
let reads=0;
const useRef=value=>({current:value}),useEffect=()=>{},useState=value=>[typeof value==='function'?value():value,()=>{}];
const getRecovery=()=>null,registerUnsentPreflight=()=>()=>{};
const workspaceApi=()=>{reads++;return new Promise(()=>{});};
export const sessionReads=()=>reads;
`;
const {useApplicationOperation,sessionReads}=await import('data:text/javascript;base64,'+Buffer.from(setup+javascript).toString('base64'));
const id=n=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
function operation(kind){const resource={kind,appId:id(1),viewId:id(2),id:id(3)};return {resource,hook:useApplicationOperation({actorId:id(4),scope:'admission-'+kind,resource,confirmed:()=>{},unauthorized:()=>{},identityMismatch:()=>{}})};}
for(const kind of ['record','draft'])test('V017 '+kind+' cannot inherit preset PUT admission',()=>{
 const {resource,hook}=operation(kind),before=sessionReads();
 assert.throws(()=>hook.start({resource,path:`applications/${id(1)}/forms/${id(2)}/${kind==='draft'?'drafts':'records'}/${id(3)}`,method:'PUT',expectedStatus:200,body:{changes:{}}}),/Invalid scoped mutation packet/);
 assert.equal(sessionReads(),before);
});
for(const kind of ['record','draft'])test('V017 '+kind+' retains PATCH admission and identity preflight',()=>{
 const {resource,hook}=operation(kind),before=sessionReads();
 assert.doesNotThrow(()=>hook.start({resource,path:`applications/${id(1)}/forms/${id(2)}/${kind==='draft'?'drafts':'records'}/${id(3)}`,method:'PATCH',expectedStatus:200,body:{changes:{}}}));
 assert.equal(sessionReads(),before+1);
});
