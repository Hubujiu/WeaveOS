import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
const workflow=readFileSync(new URL('../../.github/workflows/ci.yml',import.meta.url),'utf8'),go=workflow.slice(workflow.indexOf('\n  go:'),workflow.indexOf('\n  browser:'));
const pinned='55cc8345863c7cc4c66a329aec7e433d2d1c52a9';
const prefix="bff-build-v2-${{ runner.os }}-${{ runner.arch }}-${{ steps.go-build-path.outputs.image }}-${{ hashFiles('.go-version', 'services/bff/go.mod', 'services/bff/go.sum', '.github/workflows/ci.yml') }}-";
test('V075 compiled cache uses one environment-bound incremental fallback and exact source primary',()=>{
 assert.ok(go.includes('uses: actions/cache/restore@'+pinned));assert.match(go,/id: bff-build-cache/);assert.match(go,/go env GOCACHE/);
 const block=go.slice(go.indexOf('id: bff-build-cache'),go.indexOf('- name: Migrate isolated PostgreSQL'));
 assert.ok(block.includes('path: ${{ steps.go-build-path.outputs.path }}'));
 assert.ok(block.includes("key: "+prefix+"${{ hashFiles('services/bff/**/*.go') }}"));
 const match=block.match(/restore-keys: \|\n([\s\S]*)/);assert.ok(match,'same-environment source fallback is missing');
 assert.deepEqual(match[1].trim().split(/\n/).map(line=>line.trim()),[prefix]);
 assert.doesNotMatch(block,/bff-build-v1|enableCrossOsArchive|lookup-only|fail-on-cache-miss/);
});
test('V075 compiled cache binds actual immutable runner image and refuses missing identity',()=>{
 const identity=go.slice(go.indexOf('- name: Locate Go compiled cache'),go.indexOf('id: bff-build-cache'));
 assert.ok(identity.includes('"${ImageOS:?}"'));assert.ok(identity.includes('"${ImageVersion:?}"'));
 assert.ok(identity.includes("printf 'image=%s-%s\\n' \"$ImageOS\" \"$ImageVersion\" >> \"$GITHUB_OUTPUT\""));
 assert.ok(identity.includes('set -euo pipefail'));
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
