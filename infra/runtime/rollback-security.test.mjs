import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync,mkdirSync} from 'node:fs';import {execFileSync,spawnSync} from 'node:child_process';import {resolve} from 'node:path';
const source=readFileSync('infra/runtime/run.mjs','utf8').match(/const previous=packageImages\(\{root,commit:'([0-9a-f]{40})'/)?.[1];
assert.ok(source,'rollback needs an exact previously verified source');
const directory=resolve('.work',`rollback-audit-${Date.now()}`);mkdirSync(directory,{recursive:true});
const archive=resolve(directory,'source.tar');execFileSync('git',['archive','--output',archive,source]);execFileSync('tar',['-xf',archive,'-C',directory]);
test('configured rollback source has no reachable Go advisory',()=>{
 const r=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${directory},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-e','GOFLAGS=-buildvcs=false','-w','/repo/services/bff','golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244','go','run','golang.org/x/vuln/cmd/govulncheck@v1.8.0','./...'],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});assert.equal(r.status,0,r.stdout+r.stderr);
});
test('configured rollback source has no frontend dependency advisory',()=>{
 const r=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${directory},dst=/repo,readonly`,'-w','/repo','node:24.14.0-bookworm-slim','sh','-ec','npm install --global pnpm@10.28.2 --ignore-scripts >/dev/null; pnpm audit --json'],{encoding:'utf8',timeout:180000,maxBuffer:4*1024*1024});assert.equal(r.status,0,'rollback must pass the same frontend dependency audit');
});
