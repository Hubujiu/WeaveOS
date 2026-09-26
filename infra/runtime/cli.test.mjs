import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
const root=resolve('.');
for(const name of ['audit-read','audit-maintenance'])test(`${name} without trusted configuration fails closed without exposing stdin`,()=>{
 const result=spawnSync('docker',['run','--rm','-i','--network','none','--mount',`type=bind,src=${root},dst=/repo,readonly`,'debian:bookworm-slim',`/repo/.work/cli/${name}`],{input:'synthetic-never-log-input',encoding:'utf8',timeout:10000});
 assert.equal(result.error,undefined,'compiled command must actually execute');
 assert.ok([0,1].includes(result.status),'Docker/image/loading failure is not an application result');
 assert.notEqual(result.status,0,'missing trusted configuration must refuse operation');
 assert.equal(`${result.stdout}${result.stderr}`.includes('synthetic-never-log-input'),false);
});
