import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// V074 / V075 P22: original copy identities must not replace another guard.
const source=await readFile(new URL('../../apps/web/src/applications/shell/leaveGuards.ts',import.meta.url),'utf8');
const {LeaveGuards}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const base={kind:'structure-copy',actorId:'actor',appId:'app',resourceKind:'form',resourceId:'first'};
const controller=()=>({getStatus:()=> 'unknown',prepareLeave:()=>({ok:true})});
test('structural copy guards distinguish both resource ID and resource kind',()=>{const g=new LeaveGuards();g.register(base,controller());g.register({...base,resourceId:'second'},controller());g.register({...base,resourceKind:'table'},controller());assert.equal(g.snapshot('actor').length,3);});
test('structural copy stale cleanup preserves replacement and independent structure edits',()=>{const g=new LeaveGuards();const stale=g.register(base,controller()),current=controller();g.register(base,current);g.register({...base,resourceId:'second'},controller());g.register({kind:'structure',actorId:'actor',appId:'app'},controller());stale();assert.equal(g.snapshot('actor').length,3);assert.ok(g.snapshot('actor').some(e=>e.controller===current));});
