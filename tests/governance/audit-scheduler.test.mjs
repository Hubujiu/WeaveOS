import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {serverCompose,serverSchedules} from '../../infra/server/plan.mjs';
import {candidateCompose} from '../../infra/server/deploy/configuration.mjs';
const runtime=JSON.parse(readFileSync(new URL('../../infra/runtime/compose.json',import.meta.url)));
const images={bff:'sha256:'+'a'.repeat(64),web:'sha256:'+'b'.repeat(64)};

// FR05 / ADR005 D7: scheduling changes, archival and identity rules do not.
test('maintenance is a nondefault one-shot profile in runtime and installed config',()=>{
 for(const config of [runtime,serverCompose(runtime),candidateCompose(runtime,images)]){
  const task=config.services['audit-maintenance'];
  assert.deepEqual(task.profiles,['maintenance']);
  assert.deepEqual(task.command,['/app/audit-maintenance','--once']);
  assert.equal(task.restart,'no');
  assert.equal(Object.values(config.services).filter(s=>!s.profiles).length,4);
 }
});
test('hourly cron invokes the same single-run operation as manual maintenance',()=>{
 const cron=serverSchedules({publicTLS:true});
 assert.match(cron,/^0 \* \* \* \* root .*operations\.mjs audit/m);
 assert.equal(cron.split('\n').filter(line=>line.includes('operations.mjs audit')).length,1);
});
test('receiver rejects unreviewed profile/command/restart and topology mutations',()=>{
 for(const mutate of [c=>c.services.bff.profiles=['maintenance'],c=>c.services['audit-maintenance'].profiles=['other'],c=>c.services['audit-maintenance'].restart='always',c=>c.services['audit-maintenance'].command=['/bin/sh'],c=>c.services.extra={}]){
  const copy=structuredClone(runtime);mutate(copy);assert.throws(()=>candidateCompose(copy,images));
 }
});
