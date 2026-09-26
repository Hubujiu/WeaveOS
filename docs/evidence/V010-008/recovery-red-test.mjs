import test from 'node:test';
import assert from 'node:assert/strict';
import { recoverRuntime } from './recovery.mjs';
// ADR004/Redis recovery: a database rollback must not carry the original Session
// namespace into the recovered deployment. Full runtime checks use actual Cookies.
test('recovery must rotate shared generation after verified restore before resuming traffic',()=>{
 let restored=false,updated=false,resumed=false;
 const current='accepted-before-recovery';
 const result=recoverRuntime({generation:current,
  pause(){},restore(){restored=true;},
  switchGeneration(generation){assert.ok(restored);assert.notEqual(generation,current);updated=true;},
  resume(){assert.ok(updated);resumed=true;},
 });
 assert.ok(restored&&updated&&resumed,'recovery must actually execute the ordered restoration and generation transition');
 assert.notEqual(result.generation,current);
});
test('failed restore leaves traffic paused and does not publish a new generation',()=>{
 let generation=false,resumed=false;
 assert.throws(()=>recoverRuntime({generation:'old',pause(){},restore(){throw new Error('synthetic restore failure');},switchGeneration(){generation=true;},resume(){resumed=true;}}));
 assert.equal(generation,false);assert.equal(resumed,false);
});
