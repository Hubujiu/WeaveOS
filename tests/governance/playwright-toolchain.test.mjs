import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
// Official stable 1.64.1 and its digest-addressed MCR OCI index, verified 2026-10-15.
// Playwright #42803 / WebKit #74619: Linux libsoup 3.6.5 use-after-free.
const version='1.64.1';
const image='mcr.microsoft.com/playwright:v1.64.1-noble@sha256:7852dc5a7d72f5fe458a597d690f6c42fbc8e9e26ddfa95f8e6ef552558d9bbf';
test('browser package is pinned to the official patched stable toolchain',()=>{
 const p=JSON.parse(readFileSync('package.json','utf8'));
 assert.equal(p.devDependencies['@playwright/test'],version);
 const lock=readFileSync('pnpm-lock.yaml','utf8');
 for(const name of ['@playwright/test','playwright','playwright-core']) assert.ok(lock.includes(`${name}@${version}:`)||lock.includes(`'${name}@${version}':`),name);
 assert.ok(!lock.includes('1.63.0'));
});
for(const path of ['infra/acceptance/run.mjs','infra/runtime/run.mjs','infra/acceptance/ingress.test.mjs','infra/runtime/retention.integration.test.mjs','services/workflow-engine/product-browser-runner.mjs','services/workflow-engine/run-formal-runtime.mjs']){
 test(`patched official immutable browser image stays aligned: ${path}`,()=>{
  const text=readFileSync(path,'utf8');
  const images=text.match(/mcr\.microsoft\.com\/playwright:[^'"\s]+/g);
  assert.ok(images?.length);for(const actual of images)assert.equal(actual,image);
 });
}
