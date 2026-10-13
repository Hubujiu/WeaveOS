import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// V075 P17: independent workflow identities cannot replace each other's dirty/unknown state.
const source=await readFile(new URL('../../apps/web/src/applications/shell/leaveGuards.ts',import.meta.url),'utf8');
const {LeaveGuards}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const base={kind:'workflow-definition',actorId:'actor',appId:'app',viewId:'view',flowId:'first'};
const controller=()=>({getStatus:()=> 'unknown',prepareLeave:()=>({ok:true})});
test('definition guards keep two different flows independent within the same form',()=>{const g=new LeaveGuards(),one=controller(),two=controller();g.register(base,one);g.register({...base,flowId:'second'},two);assert.equal(g.snapshot('actor').length,2);assert.deepEqual(new Set(g.snapshot('actor').map(e=>e.controller)),new Set([one,two]));});
test('definition guard is separate from the form designer and another actor',()=>{const g=new LeaveGuards();g.register(base,controller());g.register({kind:'designer',actorId:'actor',appId:'app',viewId:'view'},controller());g.register({...base,actorId:'other'},controller());assert.equal(g.snapshot('actor').length,2);assert.equal(g.snapshot('other').length,1);});
test('definition guard stale cleanup preserves replacement and a different workflow',()=>{const g=new LeaveGuards();const stale=g.register(base,controller()),current=controller();g.register(base,current);g.register({...base,flowId:'second'},controller());stale();assert.equal(g.snapshot('actor').length,2);assert.ok(g.snapshot('actor').some(e=>e.controller===current));});
