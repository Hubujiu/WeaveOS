import {createHash} from 'node:crypto';
import {unresolvedAdvisories} from './advisory-review.mjs';
import test from 'node:test';import assert from 'node:assert/strict';import {spawnSync} from 'node:child_process';import {resolve} from 'node:path';
import {mkdirSync} from 'node:fs';
test('Go source and dependencies have no reachable published vulnerabilities',()=>{
 const result=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-e','GOFLAGS=-buildvcs=false','-w','/repo/services/bff','golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c','go','run','golang.org/x/vuln/cmd/govulncheck@v1.8.0','./...'],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});
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
test('required Go modules have no fixable advisories and unsafe OpenPGP is absent from every application dependency',()=>{
 const result=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-e','GOFLAGS=-buildvcs=false','-w','/repo/services/bff/cmd/bff','golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c','go','run','golang.org/x/vuln/cmd/govulncheck@v1.8.0','-scan','module'],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});
 const ids=[...result.stdout.matchAll(/More info: https:\/\/pkg.go.dev\/vuln\/(GO-[0-9-]+)/g)].map(m=>m[1]);
 assert.ok(result.status===0||result.status===1&&ids.length>0,'module scanner must complete successfully or report actual advisories');
 // ADR003/004 require known-vulnerability disposition. GO-2026-5932 has no
 // fixed release: an unused, unmaintained OpenPGP package shares x/crypto with
 // Argon2. Verify it is absent from ALL built application dependency graphs.
 let reviewedEvidence;
 if(ids.includes('GO-2026-6443')){
  // Root-approved review is tied to actual selected bytes and this run's behavioral evidence.
  const inspect=(args)=>spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','--mount','type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build','-e','GOFLAGS=-buildvcs=false','-w','/repo/services/bff','golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c',...args],{encoding:'utf8',timeout:180000,maxBuffer:8*1024*1024});
  const selected=inspect(['go','list','-m','-json','google.golang.org/grpc']);
  assert.equal(selected.status,0,'selected gRPC module must be inspectable');
  let module;assert.doesNotThrow(()=>{module=JSON.parse(selected.stdout)},'module metadata must be valid JSON');
  assert.equal(typeof module.Dir,'string','selected module cache path required');
  // No shell evaluation: inspect the exact module directory reported by the Go tool.
  const transport=inspect(['cat',module.Dir+'/internal/transport/http2_server.go']);
  assert.equal(transport.status,0,'selected transport source must be readable');
  const regression=inspect(['go','test','-json','-count=1','-run','^TestRootGRPCMissingAuthorityRejected$','./internal/securityreview']);
  assert.equal(regression.status,0,regression.stdout+regression.stderr);
  let events;assert.doesNotThrow(()=>{events=regression.stdout.split(/\r?\n/).filter(Boolean).map(line=>JSON.parse(line))},'real regression must return test JSON');
  const name='TestRootGRPCMissingAuthorityRejected',pkg='github.com/Hubujiu/WeaveOS/services/bff/internal/securityreview';
  reviewedEvidence={modulePath:module.Path,version:module.Version,sum:module.Sum,goModSum:module.GoModSum,replacement:module.Replace!=null,transportSHA256:createHash('sha256').update(transport.stdout).digest('hex'),regression:{exitCode:regression.status,passed:events.filter(e=>e.Action==='pass'&&e.Test===name&&e.Package===pkg).length,failed:events.filter(e=>e.Action==='fail').length,skipped:events.filter(e=>e.Action==='skip').length}};
 }
 assert.deepEqual(unresolvedAdvisories(ids,reviewedEvidence).filter(id=>id!=='GO-2026-5932'),[],'all unresolved fixable module advisories must be removed');
 const deps=spawnSync('docker',['run','--rm','--mount',`type=bind,src=${resolve('.')},dst=/repo,readonly`,'--mount','type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod','-e','GOFLAGS=-buildvcs=false','-w','/repo/services/bff','golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c','go','list','-deps','./...'],{encoding:'utf8',timeout:180000});
 assert.equal(deps.status,0,'entire application dependency graph must load');
 assert.equal(deps.stdout.split('\n').some(p=>p==='golang.org/x/crypto/openpgp'||p.startsWith('golang.org/x/crypto/openpgp/')),false,'unsupported OpenPGP must never be linked');
});

