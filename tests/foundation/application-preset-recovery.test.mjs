import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const url=source=>'data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64');
const read=p=>readFileSync(new URL('../../apps/web/src/applications/'+p,import.meta.url),'utf8');
const recoveryURL=url(read('recovery.ts'));
const recovery=await import(recoveryURL);
const {findPresetRecovery:find}=await import(url(read('records/presetRecovery.ts').replace("from '../recovery'",'from '+JSON.stringify(recoveryURL))));
const actor='aaaaaaaa-aaaa-4aaa-8aaa-000000000001',app='aaaaaaaa-aaaa-4aaa-8aaa-000000000002',view='aaaaaaaa-aaaa-4aaa-8aaa-000000000003';
function packet(extra={}){
 const resource={kind:'preset',appId:app,viewId:view,creationNonce:'aaaaaaaa-aaaa-4aaa-8aaa-000000000004',...extra};
 return {actorId:actor,scope:JSON.stringify([JSON.stringify(['presets',actor,app,view]),resource]),resource,path:`applications/${app}/forms/${view}/table-presets`,method:'POST',expectedStatus:201,operationId:'aaaaaaaa-aaaa-4aaa-8aaa-000000000005',body:{operationId:'aaaaaaaa-aaaa-4aaa-8aaa-000000000005',name:'精确方案',expectedSchemaVersion:2,filter:null,sort:null,hiddenColumnIds:[],columnOrder:[],columnWidths:{}}};
}
const clear=p=>recovery.clearRecovery(p.actorId,p.scope);
test('V017 missing scope has no invented preset recovery',()=>assert.equal(find(actor,app,view),null));
test('V017 exact actor and view reopens the original frozen preset request without mutating it',()=>{const p=packet();recovery.keepPacket(p,true);try{const found=find(actor,app,view);assert.deepEqual(found,p);assert.ok(Object.isFrozen(found));assert.ok(Object.isFrozen(found.body));assert.equal(recovery.scopedUnconfirmed(actor).length,1);}finally{clear(p);}});
test('V017 confirmed packets are not reintroduced as unknown writes',()=>{const p=packet();recovery.keepPacket(p,false);try{assert.equal(find(actor,app,view),null);}finally{clear(p);}});
for(const [name,args]of [['other actor',['bbbbbbbb-bbbb-4bbb-8bbb-000000000001',app,view]],['other app',[actor,'bbbbbbbb-bbbb-4bbb-8bbb-000000000002',view]],['other view',[actor,app,'bbbbbbbb-bbbb-4bbb-8bbb-000000000003']]])test('V017 preset recovery excludes '+name,()=>{const p=packet();recovery.keepPacket(p,true);try{assert.equal(find(...args),null);}finally{clear(p);}});
test('V017 record operations cannot become preset recovery',()=>{const p=packet({kind:'record'});recovery.keepPacket(p,true);try{assert.equal(find(actor,app,view),null);}finally{clear(p);}});
test('V017 foreign recovery scope cannot be replayed from the preset manager',()=>{const p={...packet(),scope:'foreign-scope'};recovery.keepPacket(p,true);try{assert.equal(find(actor,app,view),null);}finally{clear(p);}});
test('V017 updates preserve resource identity and exact CAS payload',()=>{const p=packet({id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000006',creationNonce:undefined});p.path+='/'+p.resource.id;p.method='PUT';p.expectedStatus=200;p.body.expectedVersion=7;recovery.keepPacket(p,true);try{assert.deepEqual(find(actor,app,view),p);}finally{clear(p);}});
test('V017 deletion preserves original operation and expected version query',()=>{const p=packet({id:'aaaaaaaa-aaaa-4aaa-8aaa-000000000006',creationNonce:undefined});p.path+='/'+p.resource.id;p.method='DELETE';p.expectedStatus=204;delete p.body;p.query={operationId:p.operationId,expectedVersion:7};recovery.keepPacket(p,true);try{assert.deepEqual(find(actor,app,view),p);}finally{clear(p);}});
