import test from 'node:test';import assert from 'node:assert/strict';import {spawnSync} from 'node:child_process';import {resolve} from 'node:path';
import {mkdirSync} from 'node:fs';
test('Go source and dependencies have no reachable published vulnerabilities',()=>{
 const result=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-w','/repo/services/bff','golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244','go','run','golang.org/x/vuln/cmd/govulncheck@v1.8.0','./...'],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});
 assert.equal(result.status,0,result.stdout+result.stderr);
});
test('tracked source archive passes a redacted Gitleaks secret scan',()=>{
 const directory=resolve('.work',`gitleaks-${Date.now()}`);mkdirSync(directory,{recursive:true});
 const archive=resolve(directory,'source.tar');
 for(const [command,args] of [['git',['archive','--format=tar','--output',archive,'HEAD']],['tar',['-xf',archive,'-C',directory]]])assert.equal(spawnSync(command,args,{encoding:'utf8'}).status,0,'tracked source archive must be available');
 const scan=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${directory},dst=/scan,readonly`,'zricethezav/gitleaks:v8.30.1@sha256:c00b6bd0aeb3071cbcb79009cb16a60dd9e0a7c60e2be9ab65d25e6bc8abbb7f','dir','/scan','--redact','--no-banner','--log-level','warn'],{encoding:'utf8',timeout:180000,maxBuffer:1024*1024});
 assert.equal(scan.status,0,'Secret scanner failed; review redacted findings privately');
});
test('frontend dependency audit reports no published vulnerabilities',()=>{
 const result=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'-w','/repo','node:24.14.0-bookworm-slim','sh','-ec','npm install --global pnpm@10.28.2 --ignore-scripts >/dev/null; pnpm audit --json'],{encoding:'utf8',timeout:180000,maxBuffer:4*1024*1024});
 let summary;try{const r=JSON.parse(result.stdout);summary=JSON.stringify({counts:r.metadata?.vulnerabilities,findings:Object.values(r.advisories??{}).map(a=>({id:a.github_advisory_id,module:a.module_name,fixed:a.patched_versions}))});}catch{summary='Dependency audit did not return valid JSON';}
 assert.equal(result.status,0,summary);
});
test('all required Go modules are patched, including code paths not reached by the application',()=>{
 const result=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-w','/repo/services/bff/cmd/bff','golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244','go','run','golang.org/x/vuln/cmd/govulncheck@v1.8.0','-scan','module'],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});
 assert.equal(result.status,0,result.stdout+result.stderr);
});
