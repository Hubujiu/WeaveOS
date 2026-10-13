import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const code=await readFile(new URL('../../apps/web/src/applications/forms/structureDeletion.ts',import.meta.url),'utf8');
const {validStructureDeletionRequest:request,validStructureDeletionReceipt:receipt,validStructureDeletionDependencies:dependencies}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(code,{mode:'transform'})).toString('base64'));
const id=n=>'abababab-abab-4bab-8bab-'+String(n).padStart(12,'0');
function fixture(kind='form'){
 const resource={kind:'structure-deletion',appId:id(1),resourceKind:kind,id:kind==='application'?id(1):id(2)};
 const path='applications/'+id(1)+(kind==='application'?'':'/'+{directory:'directories',table:'tables',form:'forms'}[kind]+'/'+id(2))+'/deletion';
 return {resource,request:{path,method:'POST',expectedStatus:200,body:{expectedStructureVersion:4,expectedResourceVersion:kind==='directory'?0:3}},context:{resource,operationId:id(3),expectedStructureVersion:4},receipt:{operationId:id(3),appId:id(1),resourceKind:kind,id:resource.id,structureVersion:5,deleted:true}};
}
test('P24 four deletion scopes require their exact bounded original request without mutation',()=>{
 for(const kind of ['application','directory','table','form']){const f=fixture(kind),before=JSON.stringify(f);assert.equal(request(f.resource,f.request),true);assert.equal(JSON.stringify(f),before);}
 for(const kind of ['directory','table','form']){const f=fixture(kind);f.request.body.expectedResourceVersion=0;f.request.body.expectedStructureVersion=0;assert.equal(request(f.resource,f.request),true);}
});
test('P24 deletion request rejects scope confusion, writes to other routes and open bodies',()=>{
 const f=fixture();
 for(const patch of [{path:f.request.path+'?force=true'},{path:f.request.path.replace('/deletion','/restoration')},{method:'DELETE'},{expectedStatus:204},{expectedStatus:202},{body:{...f.request.body,force:true}},{body:{...f.request.body,operationId:id(3)}},{body:{expectedStructureVersion:4}}])assert.equal(request(f.resource,{...f.request,...patch}),false);
 for(const patch of [{kind:'form'},{resourceKind:'unknown'},{appId:id(4)},{id:id(4)},{id:'00000000-0000-0000-0000-000000000000'},{appId:id(1).toUpperCase()},{restore:true}])assert.equal(request({...f.resource,...patch},f.request),false);
 const app=fixture('application');assert.equal(request({...app.resource,id:id(9)},app.request),false);
});
test('P24 resource CAS respects application positive, directory zero and bounded structural increment',()=>{
 for(const kind of ['application','directory','table','form']){const f=fixture(kind);for(const value of [-1,0.5,'4',Number.MAX_SAFE_INTEGER+1])for(const key of ['expectedStructureVersion','expectedResourceVersion'])assert.equal(request(f.resource,{...f.request,body:{...f.request.body,[key]:value}}),false);assert.equal(request(f.resource,{...f.request,body:{...f.request.body,expectedStructureVersion:Number.MAX_SAFE_INTEGER}}),false);}
 const app=fixture('application'),dir=fixture('directory');assert.equal(request(app.resource,{...app.request,body:{...app.request.body,expectedResourceVersion:0}}),false);assert.equal(request(dir.resource,{...dir.request,body:{...dir.request.body,expectedResourceVersion:1}}),false);
});
test('P24 deletion confirms only exact original six-key receipts with one structural increment',()=>{
 for(const kind of ['application','directory','table','form']){const f=fixture(kind),before=JSON.stringify(f);assert.equal(receipt(f.receipt,f.context),true);assert.equal(JSON.stringify(f),before);}
 const f=fixture();assert.equal(receipt({...f.receipt,structureVersion:1},{...f.context,expectedStructureVersion:0}),true);assert.equal(receipt({...f.receipt,structureVersion:Number.MAX_SAFE_INTEGER},{...f.context,expectedStructureVersion:Number.MAX_SAFE_INTEGER-1}),true);
});
test('P24 deletion cannot turn wrong scope, changed versions, recovery or leaked data into success',()=>{
 const f=fixture();for(const value of [null,[],{},...Object.keys(f.receipt).map(key=>{const v={...f.receipt};delete v[key];return v;}),...[{operationId:id(9)},{appId:id(9)},{id:id(9)},{resourceKind:'table'},{structureVersion:4},{structureVersion:6},{structureVersion:'5'},{structureVersion:5.5},{deleted:false},{deleted:'true'},{values:{}},{permissions:[]},{policyRevision:5},{pending:true}].map(p=>({...f.receipt,...p}))])assert.equal(receipt(value,f.context),false);
 assert.equal(receipt({...f.receipt,structureVersion:Number.MAX_SAFE_INTEGER+1},{...f.context,expectedStructureVersion:Number.MAX_SAFE_INTEGER}),false);
 assert.equal(receipt(f.receipt,{...f.context,operationId:'bad'}),false);
});
test('P24 nonempty dependencies retain only the current resource closed vocabulary',()=>{
 const allowed={application:['directories','tables','forms','permission_groups','workflows'],directory:['directories','tables','forms','grants'],table:['forms','workflows','drafts','pending_commands','records'],form:['workflows','drafts','pending_commands','grants']};
 for(const [kind,items] of Object.entries(allowed)){const v={dependencies:items},before=JSON.stringify(v);assert.equal(dependencies(v,kind),true);assert.equal(JSON.stringify(v),before);for(const item of items)assert.equal(dependencies({dependencies:[item]},kind),true);}
});
test('P24 dependencies refuse empty, foreign, duplicated or value-bearing errors',()=>{
 for(const kind of ['application','directory','table','form'])for(const value of [null,[],{dependencies:[]},{dependencies:['password']},{dependencies:['workflows','workflows']},{dependencies:['workflows'],counts:{workflows:1}},{dependencies:[{kind:'workflows',count:1}]},{dependencies:Array(9).fill('workflows')}])assert.equal(dependencies(value,kind),false);
 assert.equal(dependencies({dependencies:['records']},'application'),false);assert.equal(dependencies({dependencies:['tables']},'form'),false);assert.equal(dependencies({dependencies:['workflows']},'unknown'),false);
});
