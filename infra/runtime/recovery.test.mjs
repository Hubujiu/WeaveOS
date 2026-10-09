import test from 'node:test';
import assert from 'node:assert/strict';
import { recoverRuntime } from './recovery.mjs';
// ADR004/Redis recovery: a database rollback must not carry the original Session
// namespace into the recovered deployment. Full runtime checks use actual Cookies.
test('recovery must rotate shared generation after verified restore before resuming traffic',()=>{
 const events=[];
 const current='accepted-before-recovery';
 const result=recoverRuntime({generation:current,
  pause(){events.push(['pause']);},restore(){events.push(['restore']);},
  switchGeneration(generation){events.push(['switch',generation]);},
  resume(){events.push(['resume']);},
 });
 assert.equal(typeof result.generation,'string');assert.ok(result.generation.length>0);
 assert.notEqual(result.generation,current,'the previous Session namespace must be invalidated');
 assert.deepEqual(events,[['pause'],['restore'],['switch',result.generation],['resume']],'returned generation must be the one actually activated between restore and resume');
});
test('failed restore leaves traffic paused and does not publish a new generation',()=>{
 const events=[],failure=new Error('synthetic restore failure');
 assert.throws(()=>recoverRuntime({generation:'old',pause(){events.push('pause');},restore(){events.push('restore');throw failure;},switchGeneration(){events.push('switch');},resume(){events.push('resume');}}),error=>error===failure);
 assert.deepEqual(events,['pause','restore'],'failed restoration must stop with traffic paused');
});
