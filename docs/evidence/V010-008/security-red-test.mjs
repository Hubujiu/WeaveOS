import test from 'node:test';import assert from 'node:assert/strict';import {spawnSync} from 'node:child_process';import {resolve} from 'node:path';
test('Go source and dependencies have no reachable published vulnerabilities',()=>{
 const result=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-w','/repo/services/bff','golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244','go','run','golang.org/x/vuln/cmd/govulncheck@v1.8.0','./...'],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});
 assert.equal(result.status,0,result.stdout+result.stderr);
});
