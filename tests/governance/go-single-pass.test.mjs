import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {discoverRequiredGoTests as discover,verifyRequiredGoTests as verify} from '../../scripts/verify-go-required.mjs';
const sourceSha='a'.repeat(40),prefix='github.com/Hubujiu/WeaveOS/services/bff/';
const identities=[{package:prefix+'cmd/bff',name:'TestRootHostEnabledWiresRealDependenciesAndKeepsAuthentication'}, {package:prefix+'internal/appstructure',name:'TestRootRuntimeAdmissionUnavailablePublicationCreatesNothingAndCanRetry'}, {package:prefix+'internal/apprecordservice',name:'TestRootNativeCatalogStableKeyAndImmutablePriorVersion'}];
const lines=events=>events.map(e=>JSON.stringify(e)).join('\n')+'\n';
const listing=identities.flatMap(i=>[{Action:'start',Package:i.package},{Action:'output',Package:i.package,Output:i.name+'\n'},{Action:'pass',Package:i.package}]);
const events=identities.flatMap(i=>[{Action:'start',Package:i.package},{Action:'run',Package:i.package,Test:i.name},{Action:'pass',Package:i.package,Test:i.name},{Action:'pass',Package:i.package}]);
const manifest={schemaVersion:1,sourceSha,packages:identities.map(i=>i.package).sort(),tests:[...identities].sort((a,b)=>(a.package+'#'+a.name).localeCompare(b.package+'#'+b.name))};
test('V075 discovery freezes independent list identities and candidate SHA',()=>assert.deepEqual(discover(lines(listing),sourceSha),manifest));
test('V075 complete actual Go report proves each required test once',()=>assert.equal(verify(manifest,lines(events),sourceSha),3));
for(const [name,input]of [
 ['empty',''],['malformed','{bad json'],['missing package',lines(listing.filter(e=>e.Package!==identities[0].package))],
 ['duplicate listing',lines([...listing,{Action:'output',Package:identities[0].package,Output:identities[0].name+'\n'}])],
 ['failed list',lines([...listing,{Action:'fail',Package:identities[0].package}])],
 ['missing successful package',lines(listing.filter(e=>e.Action!=='pass'))],
])test('V075 discovery rejects '+name,()=>assert.throws(()=>discover(input,sourceSha)));
for(const [name,input]of [
 ['empty',''],['malformed','[]'],['missing test',lines(events.filter(e=>e.Test!==identities[0].name))],
 ['duplicate run',lines([...events,{Action:'run',Package:identities[0].package,Test:identities[0].name}])],
 ['duplicate pass',lines([...events,{Action:'pass',Package:identities[0].package,Test:identities[0].name}])],
 ['target skip',lines(events.map(e=>e.Test===identities[0].name&&e.Action==='pass'?{...e,Action:'skip'}:e))],
 ['target fail',lines(events.map(e=>e.Test===identities[0].name&&e.Action==='pass'?{...e,Action:'fail'}:e))],
 ['nested failure',lines([...events,{Action:'fail',Package:identities[0].package,Test:identities[0].name+'/denied'}])],
 ['nested skip',lines([...events,{Action:'skip',Package:identities[0].package,Test:identities[0].name+'/denied'}])],
 ['unlisted required test',lines([...events,{Action:'run',Package:identities[0].package,Test:'TestRootMainProcessNewCase'}])],
 ['missing package success',lines(events.filter(e=>e.Test||e.Action!=='pass'))],
 ['other package failure',lines([...events,{Action:'fail',Package:prefix+'internal/auth'}])],
])test('V075 verification rejects '+name,()=>assert.throws(()=>verify(manifest,input,sourceSha)));
test('V075 rejects stale candidate and malformed manifest',()=>{assert.throws(()=>verify(manifest,lines(events),'b'.repeat(40)));assert.throws(()=>verify({...manifest,tests:[]},lines(events),sourceSha));assert.throws(()=>discover(lines(listing),'not-a-commit'));});
const workflow=readFileSync(new URL('../../.github/workflows/ci.yml',import.meta.url),'utf8');
const go=workflow.slice(workflow.indexOf('\n  go:'),workflow.indexOf('\n  browser:'));
test('V075 real Go workflow discovers then runs full suite only once at original budget',()=>{
 assert.match(go,/timeout-minutes: 10/);assert.equal((go.match(/\bgo test /g)||[]).length,2);
 assert.match(go,/go test -race -json -p 1 -cover -list/);
 assert.match(go,/go test -race -json -p 1 -count=1 -coverprofile=coverage.out \.\/\.\.\./);
 assert.doesNotMatch(go,/\bgo test[^\n]*\s-run\s/);assert.doesNotMatch(go,/\bgo test[^\n]*\s-skip\s/);
 assert.match(go,/verify-go-required\.mjs discover/);assert.match(go,/verify-go-required\.mjs verify/);
 assert.match(go,/set -euo pipefail/);assert.match(go,/go vet \.\/\.\.\./);assert.match(go,/go build \.\/cmd\/bff/);
 assert.ok(go.indexOf('roles.sql')<go.indexOf('go test -race -json'));
 assert.match(go,/b2-fixture-start/);assert.match(go,/b2-fixture-stop/);assert.match(go,/go-required\.json/);assert.match(go,/go-run\.jsonl/);
});
