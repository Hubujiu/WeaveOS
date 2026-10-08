import test from 'node:test';
import assert from 'node:assert/strict';
import {dependencyMounts} from '../../infra/ci/dependency-cache.mjs';
test('PRD V049: container Go cache binds only module and build dependencies',()=>{
 assert.deepEqual(dependencyMounts('/tmp/ci-public-cache','go'),[
  '--mount','type=bind,src=/tmp/ci-public-cache/go-mod,dst=/go/pkg/mod',
  '--mount','type=bind,src=/tmp/ci-public-cache/go-build,dst=/root/.cache/go-build']);
});
test('PRD V049: container pnpm store has one precise public dependency mount',()=>{
 assert.deepEqual(dependencyMounts('/tmp/ci-public-cache','node'),[
  '--mount','type=bind,src=/tmp/ci-public-cache/pnpm-store,dst=/repo/.work/pnpm-store']);
});
for(const root of ['', '.', '/tmp/cache,readonly', '/tmp/cache\nother'])test('PRD V049: reject ambiguous cache root '+JSON.stringify(root),()=>assert.throws(()=>dependencyMounts(root,'go')));
for(const kind of ['database','redis','runtime','settings','results'])test('PRD V049: no cache layout exists for '+kind,()=>assert.throws(()=>dependencyMounts('/tmp/ci-public-cache',kind)));
