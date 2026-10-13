import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
const workflow=readFileSync(new URL('../../.github/workflows/ci.yml',import.meta.url),'utf8'),go=workflow.slice(workflow.indexOf('\n  go:'),workflow.indexOf('\n  browser:'));
const pinned='55cc8345863c7cc4c66a329aec7e433d2d1c52a9';
test('V075 exact Go compiled cache restores only GOCACHE for OS architecture toolchain dependencies and source',()=>{
 assert.ok(go.includes('uses: actions/cache/restore@'+pinned));assert.match(go,/id: bff-build-cache/);assert.match(go,/go env GOCACHE/);assert.match(go,/path: \$\{\{ steps\.go-build-path\.outputs\.path \}\}/);
 assert.match(go,/key: bff-build-v1-\$\{\{ runner.os \}\}-\$\{\{ runner.arch \}\}-\$\{\{ hashFiles\('\.go-version', 'services\/bff\/go.mod', 'services\/bff\/go.sum', 'services\/bff\/\*\*\/\*.go'\) \}\}/);assert.doesNotMatch(go,/restore-keys:/);
});
test('V075 successful independent discovery saves missing compiled cache before potentially long real suite',()=>{
 const discover=go.indexOf('verify-go-required.mjs discover'),save=go.indexOf('uses: actions/cache/save@'+pinned),full=go.indexOf('go test -race -json -p 1 -count=1');assert.ok(discover>=0&&save>discover&&save<full);
 assert.match(go,/if: steps.bff-build-cache.outputs.cache-hit != 'true'/);assert.match(go,/key: \$\{\{ steps.bff-build-cache.outputs.cache-primary-key \}\}/);
});
test('V075 cache cannot gate or replace any race test vet build or shorten stored evidence',()=>{
 assert.match(go,/timeout-minutes: 10/);assert.equal((go.match(/\bgo test /g)||[]).length,2);assert.match(go,/go test -race -json -p 1 -count=1 -coverprofile=coverage.out \.\/\.\.\./);assert.match(go,/go vet \.\/\.\.\./);assert.match(go,/go build \.\/cmd\/bff/);
 const full=go.slice(go.indexOf('- name: Format, static checks, race tests and build'),go.indexOf('- name: Clean only'));
 assert.doesNotMatch(full,/cache-hit|if:|skip|continue-on-error/);assert.match(full,/verify-go-required.mjs verify/);assert.match(go,/go-run.jsonl/);assert.match(go,/b2-fixture-stop/);
});
