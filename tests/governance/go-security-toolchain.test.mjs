import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

// V058: independent official patch pins. This is configuration coverage only;
// the actual source/module scans, builds and rollback remain separate gates.
const version = '1.27.2';
const image = 'golang:1.27.2@sha256:5bc7f572bbaa98885a3a1fd9c0aa76b59e3e14e8628bfc316bbfd0c701e4818c';
const images = [
 ['infra/acceptance/run.mjs', 1],
 ['infra/runtime/artifacts.mjs', 1],
 ['infra/runtime/run.mjs', 2],
 ['infra/runtime/cli.test.mjs', 1],
 ['infra/runtime/rollback-security.test.mjs', 1],
 ['infra/runtime/security.test.mjs', 4],
];
const compilerGuards = [
 'services/workflow-engine/generate-proto.sh',
 'services/workflow-engine/run-execution-rpc-interop.sh',
 'services/workflow-engine/run-rpc-interop.sh',
 'services/workflow-engine/run-action-interop.mjs',
 'services/workflow-engine/run-recovery-interop.mjs',
 'services/workflow-engine/run-formal-runtime.mjs',
];

test('V058: all active Go builders and security scans use the reviewed patch', () => {
 assert.equal(readFileSync('.go-version', 'utf8').trim(), version);
 assert.match(readFileSync('services/bff/go.mod', 'utf8'), /^\s*golang\.org\/x\/net v0\.60\.0(?:\s|$)/m);
 for (const [path, count] of images) {
  const references = [...readFileSync(path, 'utf8').matchAll(/['"](golang:[^'"]+)['"]/g)].map(match => match[1]);
  assert.deepEqual(references, Array(count).fill(image), path);
 }
 // Preserve the exact archived-source builder check, not a version bypass.
 assert.ok(readFileSync('infra/runtime/artifacts.mjs', 'utf8').includes("readFileSync(resolve(src,'.go-version'),'utf8').trim()!=='1.27.2'"));
 for (const path of compilerGuards) {
  const versions = readFileSync(path, 'utf8').match(/go1\.\d+\.\d+/g) ?? [];
  assert.deepEqual(versions, ['go' + version], path);
 }
});
