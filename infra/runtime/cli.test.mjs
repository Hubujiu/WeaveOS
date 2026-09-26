import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { resolve } from 'node:path';
const root=resolve('.');
const base='debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251';
// Build the real executables in this checkout; a warm local .work directory
// cannot be a prerequisite for a fresh CI runner.
const build=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${root},dst=/repo`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-w','/repo/services/bff','golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244','sh','-ec','mkdir -p /repo/.work/cli; CGO_ENABLED=0 go build -o /repo/.work/cli/audit-read ./cmd/audit-read; CGO_ENABLED=0 go build -o /repo/.work/cli/audit-maintenance ./cmd/audit-maintenance'],{encoding:'utf8',timeout:300000});
assert.equal(build.status,0,'actual CLI build must finish before behavioral tests');
assert.equal(spawnSync('docker',['pull',base],{encoding:'utf8',timeout:180000}).status,0,'runtime base must be available before the timed command test');
for(const name of ['audit-read','audit-maintenance'])test(`${name} without trusted configuration fails closed without exposing stdin`,()=>{
 const result=spawnSync('docker',['run','--rm','-i','--network','none','--mount',`type=bind,src=${root},dst=/repo,readonly`,base,`/repo/.work/cli/${name}`],{input:'synthetic-never-log-input',encoding:'utf8',timeout:10000});
 assert.equal(result.error,undefined,'compiled command must actually execute');
 assert.ok([0,1].includes(result.status),'Docker/image/loading failure is not an application result');
 assert.notEqual(result.status,0,'missing trusted configuration must refuse operation');
 assert.equal(`${result.stdout}${result.stderr}`.includes('synthetic-never-log-input'),false);
});
