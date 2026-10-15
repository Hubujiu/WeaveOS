import test from 'node:test';import assert from 'node:assert/strict';import {readFile} from 'node:fs/promises';import {stripTypeScriptTypes} from 'node:module';
const source=await readFile(new URL('../../apps/web/src/applications/shell/leaveGuards.ts',import.meta.url),'utf8');
const {LeaveGuards}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const base={kind:'workflow-round',actorId:'actor',appId:'app',viewId:'view',recordId:'record',instanceId:'round'};
const controller=()=>({getStatus:()=> 'unknown',prepareLeave:()=>({ok:true})});
test('V069 round actions isolate previous instances of the same record',()=>{const guards=new LeaveGuards();guards.register(base,controller());guards.register({...base,instanceId:'other'},controller());assert.equal(guards.snapshot('actor').length,2);});
test('V069 round actions isolate records as well as instances',()=>{const guards=new LeaveGuards();guards.register(base,controller());guards.register({...base,recordId:'other'},controller());assert.equal(guards.snapshot('actor').length,2);});
test('V069 round action cannot replace withdrawal, manual or ordinary record guards',()=>{const guards=new LeaveGuards();for(const kind of ['workflow-round','workflow-instance','manual-start','record'])guards.register({...base,kind},controller());assert.equal(guards.snapshot('actor').length,4);});
test('V069 stale round cleanup keeps current registration',()=>{const guards=new LeaveGuards();const remove=guards.register(base,controller()),current=controller();guards.register(base,current);remove();assert.equal(guards.snapshot('actor')[0].controller,current);});
