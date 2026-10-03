import assert from 'node:assert/strict';
import { test } from 'node:test';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, rmSync, statSync, symlinkSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { startB2TestDatabase, stopB2TestDatabase, runB2FixtureCommand, runAcceptance } from '../../infra/acceptance/run.mjs';

// Oracle: root's six-item B2 CI PLAN, read from PRD/ADR009 on 2026-10-02.
// These tests replace only the Docker command boundary. They prove wiring and
// ownership/cleanup decisions, not PostgreSQL semantics or product acceptance.
// Real appschema/full-Go/product runs remain mandatory integration evidence.
const image = 'postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722';
const ownerLabel = 'com.weaveos.b2-fixture.owner';
const kindLabel = 'com.weaveos.test-fixture';

function localFiles(t) {
  const directory = mkdtempSync(join(tmpdir(), 'weaveos-b2-ci-test-meta-'));
  const stateFile = join(directory, 'state.json');
  t.after(() => {
    if (existsSync(stateFile)) {
      const s = JSON.parse(readFileSync(stateFile, 'utf8'));
      if (s.root?.startsWith('/tmp/weaveos-b2-') && existsSync(s.root) && statSync(s.root).uid === process.getuid()) rmSync(s.root, { recursive: true });
    }
    rmSync(directory, { recursive: true, force: true });
  });
  return { directory, stateFile };
}

function prepareFixture(t) {
  const f = localFiles(t);
  const root = mkdtempSync(join(tmpdir(), 'weaveos-b2-ci-test-'));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const socket = join(root, 'socket'); mkdirSync(socket);
  const owner = 'a'.repeat(32);
  const state = { version: 1, root, socket, owner, container: 'weaveos-b2-ci-' + owner, uid: process.getuid(), gid: process.getgid() };
  writeFileSync(f.stateFile, JSON.stringify(state), { mode: 0o600 });
  return { ...f, state };
}

function dockerModel(stateFile, behavior = {}) {
  const calls = [];
  const execute = (command, args, options = {}) => {
    calls.push({ command, args: [...args], options });
    assert.equal(command, 'docker', 'fixture operations must stay at the Docker boundary');
    if (args[0] === 'run' && args.includes('--detach') && behavior.runFailure) throw behavior.runFailure;
    if (args[0] === 'inspect') {
      const s = JSON.parse(readFileSync(stateFile, 'utf8'));
      return JSON.stringify([{
        Config: { Image: image, Labels: { [kindLabel]: 'b2-ci', [ownerLabel]: behavior.wrongOwner ? 'different-owner' : s.owner }, Env: ['POSTGRES_USER=weaveos_b2_test', 'POSTGRES_DB=weaveos_b2_isolated_test'] },
        HostConfig: { NetworkMode: behavior.wrongNetwork ? 'bridge' : 'none', PortBindings: {} },
        Mounts: [{ Type: 'bind', Source: behavior.wrongMount ? '/tmp/some-other-socket' : s.socket, Destination: '/var/run/postgresql' }],
        State: { Running: true },
      }]);
    }
    if (args[0] === 'rm' && behavior.removeFailure) throw behavior.removeFailure;
    return '';
  };
  return { calls, execute };
}

test('B2 CI: fixture creates a private dedicated socket database with no network/ports', async t => {
  const f = localFiles(t), docker = dockerModel(f.stateFile);
  const state = await startB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute });
  assert.ok(state, 'CI must create and return the dedicated fixture');
  const url = new URL(state.url);
  assert.equal(url.pathname, '/weaveos_b2_isolated_test');
  assert.equal(url.host, '');
  assert.equal(url.searchParams.get('host'), state.socket);
  assert.equal(url.searchParams.get('user'), 'weaveos_b2_test');
  assert.ok(state.socket.startsWith('/tmp/weaveos-b2-'));
  assert.equal(statSync(state.root).mode & 0o777, 0o700, 'host socket parent stays private');
  assert.equal(statSync(f.stateFile).mode & 0o777, 0o600);
  const run = docker.calls.find(c => c.args[0] === 'run' && c.args.includes('--detach'));
  assert.ok(run, 'real fixture provisioning command required');
  assert.equal(run.args[run.args.indexOf('--network') + 1], 'none');
  assert.ok(!run.args.some(a => a === '-p' || a === '--publish' || a === '--privileged'));
  assert.ok(run.args.includes(image));
  assert.ok(run.args.includes('POSTGRES_DB=weaveos_b2_isolated_test'));
  assert.ok(run.args.includes('listen_addresses='));
  assert.ok(docker.calls.some(c => c.args[0] === 'exec' && c.args.includes('pg_isready')), 'readiness must precede exposing the fixture');
  await stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute });
});

test('B2 CI: an existing state file is never overwritten', async t => {
  const f = prepareFixture(t), docker = dockerModel(f.stateFile);
  const before = readFileSync(f.stateFile, 'utf8');
  await assert.rejects(startB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute }));
  assert.equal(readFileSync(f.stateFile, 'utf8'), before);
  assert.equal(docker.calls.length, 0);
});

test('B2 CI: partial startup failure is nonzero and removes only its own fixture', async t => {
  const f = localFiles(t), error = new Error('isolated startup failure');
  const docker = dockerModel(f.stateFile, { runFailure: error });
  await assert.rejects(startB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute }), /isolated startup failure/);
  assert.ok(docker.calls.some(c => c.args[0] === 'rm' && c.args.includes('--volumes')));
  assert.equal(existsSync(f.stateFile), false);
});

test('B2 CI: successful cleanup removes its container/data/socket/state and is idempotent', async t => {
  const f = prepareFixture(t), docker = dockerModel(f.stateFile);
  await stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute });
  assert.ok(docker.calls.some(c => c.args[0] === 'rm' && c.args.includes('--volumes') && c.args.includes(f.state.container)));
  assert.equal(existsSync(f.state.root), false);
  assert.equal(existsSync(f.stateFile), false);
  const count = docker.calls.length;
  await stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute });
  assert.equal(docker.calls.length, count);
});

for (const reason of ['wrongOwner', 'wrongNetwork', 'wrongMount']) test('B2 CI: cleanup refuses ' + reason, async t => {
  const f = prepareFixture(t), docker = dockerModel(f.stateFile, { [reason]: true });
  await assert.rejects(stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute }));
  assert.ok(!docker.calls.some(c => c.args[0] === 'rm' || c.args[0] === 'stop'));
  assert.equal(existsSync(f.state.root), true);
  assert.equal(existsSync(f.stateFile), true);
});

test('B2 CI: cleanup failure is not swallowed and retains recoverable state', async t => {
  const f = prepareFixture(t), docker = dockerModel(f.stateFile, { removeFailure: new Error('fixture removal failed') });
  await assert.rejects(stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute }), /fixture removal failed/);
  assert.equal(existsSync(f.stateFile), true);
  assert.equal(existsSync(f.state.root), true);
});

test('B2 CI: cleanup refuses an outside root without any Docker action', async t => {
  const f = prepareFixture(t), docker = dockerModel(f.stateFile);
  writeFileSync(f.stateFile, JSON.stringify({ ...f.state, root: '/tmp', socket: '/tmp/socket' }));
  await assert.rejects(stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute }));
  assert.equal(docker.calls.length, 0);
  writeFileSync(f.stateFile, JSON.stringify(f.state));
});

test('B2 CI: cleanup refuses symlink state and never touches its target', async t => {
  const f = prepareFixture(t), target = join(f.directory, 'private-original.json');
  writeFileSync(target, readFileSync(f.stateFile), { mode: 0o600 });
  rmSync(f.stateFile); symlinkSync(target, f.stateFile);
  const docker = dockerModel(f.stateFile);
  await assert.rejects(stopB2TestDatabase({ stateFile: f.stateFile, execute: docker.execute }));
  assert.equal(docker.calls.length, 0);
  assert.equal(existsSync(target), true);
  rmSync(f.stateFile); writeFileSync(f.stateFile, JSON.stringify(f.state), { mode: 0o600 });
});

test('B2 CI: bootstrap exports the dedicated URL and always-cleanup accepts the actual state', async t => {
  const f = localFiles(t), docker = dockerModel(f.stateFile), environmentFile = join(f.directory, 'github-env');
  writeFileSync(environmentFile, 'EXISTING_VARIABLE=retained\n', { mode: 0o600 });
  await runB2FixtureCommand(['b2-fixture-start', environmentFile, f.stateFile], { execute: docker.execute });
  const content = readFileSync(environmentFile, 'utf8');
  assert.ok(content.startsWith('EXISTING_VARIABLE=retained\n'));
  assert.match(content, /^WEAVEOS_B2_TEST_DATABASE_URL=postgres:\/\/\/weaveos_b2_isolated_test\?/m);
  assert.ok(!content.includes('PASSWORD='));
  await runB2FixtureCommand(['b2-fixture-stop', f.stateFile], { execute: docker.execute });
  assert.equal(existsSync(f.stateFile), false);
});

test('B2 CI: bootstrap rejects unknown commands instead of silently succeeding', async () => {
  await assert.rejects(runB2FixtureCommand(['unrecognized-command']));
});

test('B2 CI: Go job supplies the fixture before race tests with unconditional owned cleanup', () => {
  const ci = readFileSync(new URL('../../.github/workflows/ci.yml', import.meta.url), 'utf8');
  const go = ci.slice(ci.indexOf('  go:\n'), ci.indexOf('  browser:\n'));
  assert.ok(go.includes('b2-fixture-start'), 'Go job must provision the dedicated B2 fixture');
  assert.ok(go.includes('b2-fixture-stop'), 'Go job must clean its own fixture on failure too');
  assert.ok(go.indexOf('b2-fixture-start') < go.indexOf('go test -race'));
  assert.match(go, /if:\s*always\(\)[\s\S]*b2-fixture-stop/);
  assert.ok(!go.includes('continue-on-error'));
  assert.match(ci, /permissions:\s*\n\s*contents: read/);
  assert.ok(!ci.includes('pull_request_target'));
});

async function acceptanceProbe(t) {
  const directory = mkdtempSync(join(tmpdir(), 'weaveos-b2-ci-test-acceptance-'));
  const stateFile = join(directory, 'b2-test-database.json');
  const docker = dockerModel(stateFile);
  const calls = [];
  const probeError = new Error('probe stops after nested Go test entrypoint');
  const execute = (command, args, options = {}) => {
    calls.push({ command, args: [...args], options });
    if (command === 'docker' && args[0] !== 'compose') {
      if (args[0] === 'run' && args.at(-1)?.startsWith('go test -race')) throw probeError;
      if (args[0] === 'run' && args.includes('-w')) return '';
      return docker.execute(command, args, options);
    }
    return command === 'docker' && args.includes('ps') ? 'isolated-product-pg\n' : '';
  };
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  await assert.rejects(runAcceptance({ directory, execute }), /probe stops after nested Go test entrypoint/);
  return { directory, stateFile, calls, docker };
}

test('B2 CI: acceptance gives nested Go the dedicated URL and matching socket mount', async t => {
  const f = await acceptanceProbe(t);
  const env = readFileSync(join(f.directory, 'test.env'), 'utf8');
  const line = env.split('\n').find(s => s.startsWith('WEAVEOS_B2_TEST_DATABASE_URL='));
  assert.ok(line, 'nested Go test env must contain the B2 fixture URL');
  const socket = new URL(line.slice(line.indexOf('=') + 1)).searchParams.get('host');
  const go = f.calls.find(c => c.args[0] === 'run' && c.args.at(-1)?.startsWith('go test -race'));
  assert.ok(go.args.includes(`type=bind,src=${socket},dst=${socket},readonly`), 'nested mount must match the URL path exactly');
  assert.match(env, /^WEAVEOS_TEST_DATABASE_URL=.*\/weaveos_ci_test\?/m, 'original product database is preserved');
  assert.match(env, /^WEAVEOS_TEST_ARCHIVE_DATABASE_URL=.*\/weaveos_ci_archive_test\?/m);
  assert.match(env, /^WEAVEOS_TEST_REDIS_URL=redis:\/\/127.0.0.1:6379\/15$/m);
});

test('B2 CI: acceptance failure cleans B2 and preserves original product stop-only lifecycle', async t => {
  const f = await acceptanceProbe(t);
  assert.ok(f.docker.calls.some(c => c.args[0] === 'rm' && c.args.includes('--volumes')), 'new B2 data must be cleaned');
  assert.equal(existsSync(f.stateFile), false);
  assert.ok(f.calls.some(c => c.args[0] === 'compose' && c.args.at(-1) === 'stop'));
  assert.ok(!f.calls.some(c => c.args[0] === 'compose' && c.args.includes('--volumes')), 'existing product volume policy must remain unchanged');
  assert.equal(existsSync(join(f.directory, 'runtime.env')), true, 'private product fixture is retained');
});
