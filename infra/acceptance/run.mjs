// Reproducible isolated Linux product acceptance. No production account or data.
import {dependencyMounts} from '../ci/dependency-cache.mjs';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync, existsSync, readFileSync, mkdtempSync, chmodSync, lstatSync, realpathSync, rmSync, unlinkSync, appendFileSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { resolve, dirname, basename, isAbsolute, join, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import {countAPIReport,countBrowserReport} from './result-counts.mjs';
import {loadComponentArtifacts} from '../../scripts/verify-component-artifacts.mjs';

export async function runAcceptance(options = {}) {
const root = fileURLToPath(new URL('../../', import.meta.url));
const dir = options.directory ?? resolve(root, '.work/acceptance');
const dependencyCacheRoot = process.env.WEAVEOS_DEPENDENCY_CACHE_DIR;
const goCacheMounts = dependencyCacheRoot === undefined ? ['--mount', 'type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod', '--mount', 'type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build'] : dependencyMounts(dependencyCacheRoot, 'go');
const nodeCacheMounts = dependencyCacheRoot === undefined ? [] : dependencyMounts(dependencyCacheRoot, 'node');
if (dependencyCacheRoot !== undefined) for (const name of ['go-mod', 'go-build', 'pnpm-store']) mkdirSync(resolve(dependencyCacheRoot, name), {recursive: true});
let componentSummary;
if (process.env.WEAVEOS_COMPONENT_SHARD_DIR !== undefined) {
  if (!process.env.WEAVEOS_COMPONENT_SHARD_DIR) throw new Error('Component shard directory must be explicit and nonempty');
  const shardDirectory = realpathSync(resolve(process.env.WEAVEOS_COMPONENT_SHARD_DIR));
  const privateDirectory = resolve(dir);
  if (shardDirectory === privateDirectory || shardDirectory.startsWith(privateDirectory + sep) || privateDirectory.startsWith(shardDirectory + sep)) throw new Error('Component reports must be isolated from the private acceptance environment');
  const expectedSha = execFileSync('git', ['rev-parse', 'HEAD'], {cwd: root, encoding: 'utf8'}).trim();
  const allowedSkips = JSON.parse(readFileSync(resolve(root, 'scripts/component-skips.json'), 'utf8'));
  componentSummary = loadComponentArtifacts({directory: shardDirectory, expectedSha, allowedSkips});
}
if (existsSync(resolve(dir, 'runtime.env'))) throw new Error('Existing private environment: inspect it before rerunning; no overwrite');
mkdirSync(resolve(dir, 'tls'), { recursive: true });
mkdirSync(resolve(dir, 'public'), { recursive: true });
const project = `weaveos-v010-007-${Date.now()}`;
const env = { ...process.env, WEAVEOS_ACCEPTANCE_DIR: dir };
const composeArgs = ['compose', '-p', project, '-f', resolve(root, 'infra/acceptance/compose.json')];
const execute = options.execute ?? execFileSync;
const b2StateFile = resolve(dir, 'b2-test-database.json');
let b2Fixture;
const call = (command, args, options = {}) => execute(command, args, { cwd: root, env, stdio: 'inherit', ...options });
const compose = (...args) => call('docker', [...composeArgs, ...args]);
const id = name => call('docker', [...composeArgs, 'ps', '-q', name], { encoding: 'utf8', stdio: 'pipe' }).trim();
const password = randomBytes(32).toString('hex');
const database = name => `postgres://weaveos_test:${password}@127.0.0.1:5432/${name}?sslmode=disable&connect_timeout=2`;
const privateFile = (name, text) => writeFileSync(resolve(dir, name), text, { mode: 0o600, flag: 'wx' });
privateFile('postgres.env', `POSTGRES_USER=weaveos_test\nPOSTGRES_PASSWORD=${password}\nPOSTGRES_DB=weaveos_ci_test\n`);
privateFile('runtime.env', `WEAVEOS_DATABASE_URL=${database('weaveos_acceptance').replace('@127.0.0.1:', '@postgres:')}\nWEAVEOS_REDIS_URL=redis://redis:6379/0\nWEAVEOS_SESSION_GENERATION=${project}\nWEAVEOS_AUDIT_KEY_ID=test\nWEAVEOS_AUDIT_HMAC_KEY=${randomBytes(32).toString('base64')}\nWEAVEOS_DEFINITION_HMAC_KEY=${randomBytes(32).toString('base64')}\nWEAVEOS_DEFINITION_KEY_ID=test\nWEAVEOS_SCHEMA_LOCK_TIMEOUT_MS=1000\nWEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS=5000\n`);
privateFile('seed.env', `WEAVEOS_TEST_DATABASE_URL=${database('weaveos_acceptance')}\nWEAVEOS_ACCEPTANCE_FIXTURES=/repo/.work/acceptance/fixtures.json\n`);
const openssl = process.platform === 'win32' ? 'C:/Program Files/Git/usr/bin/openssl.exe' : 'openssl';
call(openssl, ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2', '-keyout', resolve(dir, 'tls/key.pem'), '-out', resolve(dir, 'tls/cert.pem'), '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost,IP:127.0.0.1'], { stdio: 'pipe' });
const goImage = 'golang:1.27.1@sha256:3680233e3204827fbdc66088528ae6d4b3d034f51d03a99d454f6de034888244';
const playwrightImage = 'mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27';
const mount = ['--mount', `type=bind,src=${root},dst=/repo`];
const go = (script, envFile = 'test.env') => call('docker', ['run', '--rm', '--network', `container:${id('postgres')}`, ...mount, ...(envFile === 'test.env' ? ['--mount', `type=bind,src=${b2Fixture.socket},dst=${b2Fixture.socket},readonly`] : []), ...goCacheMounts, '--env-file', resolve(dir, envFile), '-e', 'GOTOOLCHAIN=local', '-e', 'GOFLAGS=-buildvcs=false', '-e', 'GOBIN=/repo/.work/acceptance/tools', '-w', '/repo/services/bff', goImage, 'sh', '-ec', script]);
const node = (script, network) => call('docker', ['run', '--rm', '--init', '--shm-size=1g', ...(network ? ['--network', `container:${network}`] : []), ...mount, ...nodeCacheMounts, '--mount', 'type=volume,src=weaveos-v010-linux-node,dst=/repo/node_modules', '--mount', 'type=volume,src=weaveos-v010-linux-web-node,dst=/repo/apps/web/node_modules', '-e', 'CI=true', '-e', 'WEAVEOS_API_URL=https://localhost:19443', '-e', 'WEAVEOS_WEB_URL=https://localhost:19443', '-e', 'WEAVEOS_ACCEPTANCE_FIXTURES=/repo/.work/acceptance/fixtures.json', '-e', 'NODE_EXTRA_CA_CERTS=/repo/.work/acceptance/tls/cert.pem', '-w', '/repo', playwrightImage, 'bash', '-euc', `npm install --global pnpm@10.28.2 --ignore-scripts; ${script}`]);
const started = Date.now();
writeFileSync(resolve(dir, 'public/result.json'), JSON.stringify({ result: 'running', project, target: 'isolated Linux HTTPS' }));
try {
  b2Fixture = await startB2TestDatabase({ stateFile: b2StateFile, execute });
  privateFile('test.env', `WEAVEOS_TEST_DATABASE_URL=${database('weaveos_ci_test')}\nWEAVEOS_TEST_ARCHIVE_DATABASE_URL=${database('weaveos_ci_archive_test')}\nWEAVEOS_TEST_REDIS_URL=redis://127.0.0.1:6379/15\nWEAVEOS_B2_TEST_DATABASE_URL=${b2Fixture.url}\n`);
  compose('up', '-d', '--wait', 'postgres', 'redis', 'test-redis');
  compose('exec', '-T', 'postgres', 'createdb', '-U', 'weaveos_test', 'weaveos_ci_archive_test');
  // Published initial migration intentionally has no destructive Down. Recovery is V010-008.
  go('go install github.com/pressly/goose/v3/cmd/goose@v3.28.0');
  go('/repo/.work/acceptance/tools/goose -dir /repo/db/archive-migrations postgres "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" up; /repo/.work/acceptance/tools/goose -dir /repo/db/archive-migrations postgres "$WEAVEOS_TEST_ARCHIVE_DATABASE_URL" up');
  go('/repo/.work/acceptance/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up; /repo/.work/acceptance/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up');
  go('go test -race -p 1 -count=1 ./... && go vet ./... && CGO_ENABLED=0 go build -o /repo/.work/acceptance/bff ./cmd/bff');
  compose('stop', 'test-redis');
  compose('exec', '-T', 'postgres', 'createdb', '-U', 'weaveos_test', 'weaveos_acceptance');
  go('/repo/.work/acceptance/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up && go run ./cmd/acceptance-seed', 'seed.env');
  if (typeof process.getuid === 'function') {
    // Seed runs as container root; preserve0600 while assigning the sole authorized host reader.
    call('docker', ['run', '--rm', ...mount, goImage, 'chown', `${process.getuid()}:${process.getgid()}`, '/repo/.work/acceptance/fixtures.json']);
  }
  const frontendChecks = 'pnpm install --frozen-lockfile --ignore-scripts --store-dir .work/pnpm-store; node --test contracts/*.test.mjs tests/governance/*.test.mjs tests/foundation/*.test.mjs tests/acceptance/topology.test.mjs; pnpm exec redocly lint contracts/openapi/openapi.json; pnpm typecheck; pnpm build';
  if (componentSummary) {
    node(frontendChecks);
    writeFileSync(resolve(dir, 'components.json'), JSON.stringify(componentSummary, null, 2) + '\n');
  } else {
    node(frontendChecks + '; cd apps/web; PLAYWRIGHT_JSON_OUTPUT_NAME=/repo/.work/acceptance/components.json pnpm exec playwright test --config playwright.component.config.ts --reporter=line,json');
  }
  compose('up', '-d', 'bff', 'nginx');
  call(process.execPath, ['--test', 'infra/acceptance/ingress.test.mjs'], { env: { ...env,
    WEAVEOS_API_URL: 'https://localhost:19443',
    WEAVEOS_INGRESS_CONTAINERS: JSON.stringify({ nginx: id('nginx'), bff: id('bff'), postgres: id('postgres') }),
    WEAVEOS_INGRESS_DATABASE: 'weaveos_acceptance', NODE_EXTRA_CA_CERTS: resolve(dir, 'tls/cert.pem') } });
  node('node --test --test-reporter=spec --test-reporter-destination=stdout --test-reporter=tap --test-reporter-destination=/repo/.work/acceptance/api.tap tests/acceptance/api.test.mjs; PLAYWRIGHT_JSON_OUTPUT_NAME=/repo/.work/acceptance/browser.json pnpm exec playwright test --config apps/web/playwright.integration.config.ts --reporter=line,json', id('nginx'));
  call(process.execPath, ['--test', 'infra/acceptance/faults.test.mjs'], { env: { ...env, WEAVEOS_ACCEPTANCE_PROJECT: project, WEAVEOS_ACCEPTANCE_FIXTURES: resolve(dir, 'fixtures.json'), NODE_EXTRA_CA_CERTS: resolve(dir, 'tls/cert.pem') } });
  const storeEnv = { ...env, WEAVEOS_API_URL: 'https://localhost:19443', WEAVEOS_ACCEPTANCE_PROJECT: project, WEAVEOS_ACCEPTANCE_FIXTURES: resolve(dir, 'fixtures.json'), WEAVEOS_ACCEPTANCE_OBSERVER: resolve(root, 'tests/acceptance/storage-observer.mjs'), NODE_EXTRA_CA_CERTS: resolve(dir, 'tls/cert.pem') };
  // Serial fault/control qualification precedes product assertions on the same real stack.
  call(process.execPath, ['--test', '--test-concurrency=1', 'tests/acceptance/storage-observer.test.mjs', 'tests/acceptance/redis-gate.test.mjs'], { env: storeEnv });
  call(process.execPath, ['--test', '--test-concurrency=1', 'tests/acceptance/integration.test.mjs'], { env: storeEnv });
  writeFileSync(resolve(dir, 'public/result.json'), JSON.stringify({ result: 'passed', project, elapsedSeconds: (Date.now() - started) / 1000, api:countAPIReport(readFileSync(resolve(dir,'api.tap'),'utf8')), browser:countBrowserReport(JSON.parse(readFileSync(resolve(dir,'browser.json'),'utf8'))), components: JSON.parse(readFileSync(resolve(dir, 'components.json'), 'utf8')).stats.expected, faults: 3, storageCases:19, storageControls:8, storage: 'actual PostgreSQL18 + Redis8.2', target: 'isolated Linux HTTPS', secrets: 'not included' }, null, 2));
} catch (error) {
  writeFileSync(resolve(dir, 'public/result.json'), JSON.stringify({ result: 'failed', project, exitCode: error.status ?? 1, elapsedSeconds: (Date.now() - started) / 1000 }));
  throw error;
} finally {
  // Preserve original private product fixtures/DB volume. The additional B2
  // fixture has its own checked identity and disposable data/socket lifecycle.
  try { compose('stop'); }
  finally { await stopB2TestDatabase({ stateFile: b2StateFile, execute }); }
}
}

const b2Image = 'postgres:18.6@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722';
const b2OwnerLabel = 'com.weaveos.b2-fixture.owner';
const b2KindLabel = 'com.weaveos.test-fixture';
const b2User = 'weaveos_b2_test';
const b2Database = 'weaveos_b2_isolated_test';
const optionalStat = path => {
  try { return lstatSync(path); }
  catch (error) { if (error.code === 'ENOENT') return null; throw error; }
};
const fixtureCall = (execute, args, timeout = 30000) => execute('docker', args, { encoding: 'utf8', stdio: 'pipe', timeout });

function readB2State(stateFile) {
  if (typeof stateFile !== 'string' || !isAbsolute(stateFile)) throw new Error('B2 fixture requires an absolute state path');
  const file = optionalStat(stateFile);
  if (!file) return null;
  if (!file.isFile() || file.isSymbolicLink() || file.uid !== process.getuid() || (file.mode & 0o777) !== 0o600) throw new Error('B2 fixture state is not an owned private regular file');
  const state = JSON.parse(readFileSync(stateFile, 'utf8'));
  if (state.version !== 1 || typeof state.owner !== 'string' || !/^[a-f0-9]{32}$/.test(state.owner) || state.container !== 'weaveos-b2-ci-' + state.owner || state.uid !== process.getuid() || state.gid !== process.getgid() || typeof state.root !== 'string' || dirname(state.root) !== '/tmp' || !/^weaveos-b2-[a-zA-Z0-9-]+$/.test(basename(state.root)) || resolve(state.root) !== state.root || state.socket !== join(state.root, 'socket')) throw new Error('B2 fixture identity/path mismatch');
  const root = optionalStat(state.root), socket = optionalStat(state.socket);
  if (root && (!root.isDirectory() || root.isSymbolicLink() || root.uid !== state.uid || (root.mode & 0o777) !== 0o700 || realpathSync(state.root) !== state.root)) throw new Error('B2 fixture root is not its owned private directory');
  if (socket && (!socket.isDirectory() || socket.isSymbolicLink() || realpathSync(state.socket) !== state.socket)) throw new Error('B2 fixture socket path is not a direct directory');
  return state;
}

function inspectB2Container(state, execute) {
  let content;
  try { content = fixtureCall(execute, ['inspect', state.container]); }
  catch (error) {
    if (error.status === 1 && new RegExp('No such (?:object|container): ' + state.container + '(?:\\s|$)').test(String(error.stderr))) return null;
    throw error;
  }
  const info = JSON.parse(content)[0];
  if (!info || info.Config?.Image !== b2Image || info.Config?.Labels?.[b2KindLabel] !== 'b2-ci' || info.Config?.Labels?.[b2OwnerLabel] !== state.owner || info.HostConfig?.NetworkMode !== 'none' || Object.keys(info.HostConfig.PortBindings ?? {}).length !== 0 || !info.Config.Env?.includes('POSTGRES_USER=' + b2User) || !info.Config.Env?.includes('POSTGRES_DB=' + b2Database) || !info.Mounts?.some(m => m.Type === 'bind' && m.Source === state.socket && m.Destination === '/var/run/postgresql')) throw new Error('Refusing a B2 fixture container with different ownership/isolation');
  return info;
}

export async function startB2TestDatabase({ stateFile, execute = execFileSync } = {}) {
  if (typeof process.getuid !== 'function' || typeof process.getgid !== 'function') throw new Error('B2 socket fixture requires the Linux test runner');
  if (typeof stateFile !== 'string' || !isAbsolute(stateFile)) throw new Error('B2 fixture requires an absolute state path');
  if (optionalStat(stateFile)) throw new Error('Existing B2 fixture state: inspect before rerunning; no overwrite');
  const root = mkdtempSync('/tmp/weaveos-b2-ci-');
  const owner = randomBytes(16).toString('hex');
  const state = { version: 1, root, socket: join(root, 'socket'), owner, container: 'weaveos-b2-ci-' + owner, uid: process.getuid(), gid: process.getgid() };
  try {
    mkdirSync(state.socket, { mode: 0o777 }); chmodSync(state.socket, 0o777);
    writeFileSync(stateFile, JSON.stringify(state), { mode: 0o600, flag: 'wx' });
  } catch (error) { rmSync(root, { recursive: true }); throw error; }
  try {
    fixtureCall(execute, ['run', '--detach', '--name', state.container, '--network', 'none', '--label', b2KindLabel + '=b2-ci', '--label', b2OwnerLabel + '=' + owner, '--mount', `type=bind,src=${state.socket},dst=/var/run/postgresql`, '--env', 'POSTGRES_USER=' + b2User, '--env', 'POSTGRES_DB=' + b2Database, '--env', 'POSTGRES_HOST_AUTH_METHOD=trust', b2Image, '-c', 'listen_addresses=', '-c', 'unix_socket_directories=/var/run/postgresql'], 180000);
    const deadline = Date.now() + 30000;
    while (true) {
      try { fixtureCall(execute, ['exec', state.container, 'pg_isready', '-U', b2User, '-d', b2Database], 5000); break; }
      catch (error) {
        if (Date.now() >= deadline) throw new Error('Dedicated B2 PostgreSQL did not become ready', { cause: error });
        await new Promise(resolve => setTimeout(resolve, 250));
      }
    }
    if (!inspectB2Container(state, execute)?.State?.Running) throw new Error('Dedicated B2 PostgreSQL stopped before use');
    return { ...state, url: `postgres:///${b2Database}?host=${encodeURIComponent(state.socket)}&user=${b2User}&sslmode=disable` };
  } catch (error) {
    try { await stopB2TestDatabase({ stateFile, execute }); }
    catch (cleanup) { throw new AggregateError([error, cleanup], 'B2 fixture setup failed; owned recovery state retained'); }
    throw error;
  }
}

export async function stopB2TestDatabase({ stateFile, execute = execFileSync } = {}) {
  const state = readB2State(stateFile);
  if (!state) return;
  const info = inspectB2Container(state, execute);
  if (info) {
    if (info.State?.Running) fixtureCall(execute, ['stop', '--time', '5', state.container], 15000);
    fixtureCall(execute, ['rm', '--force', '--volumes', state.container]);
  }
  // PostgreSQL owns its socket directory. Repair only this validated private
  // bind mount; --rm removes this short-lived helper and its anonymous volume.
  const socket = optionalStat(state.socket);
  if (socket && socket.uid !== state.uid) fixtureCall(execute, ['run', '--rm', '--network', 'none', '--user', '0:0', '--label', b2KindLabel + '=b2-ci-cleanup', '--label', b2OwnerLabel + '=' + state.owner, '--entrypoint', 'chown', '--mount', `type=bind,src=${state.socket},dst=/private-fixture`, b2Image, '-R', `${state.uid}:${state.gid}`, '/private-fixture']);
  if (optionalStat(state.root)) rmSync(state.root, { recursive: true });
  unlinkSync(stateFile);
}

export async function runB2FixtureCommand(args, { execute = execFileSync } = {}) {
  if (Array.isArray(args) && args.length === 3 && args[0] === 'b2-fixture-start') {
    const [, environmentFile, stateFile] = args;
    const state = await startB2TestDatabase({ stateFile, execute });
    try { appendFileSync(environmentFile, `\nWEAVEOS_B2_TEST_DATABASE_URL=${state.url}\n`, { mode: 0o600 }); }
    catch (error) {
      try { await stopB2TestDatabase({ stateFile, execute }); }
      catch (cleanup) { throw new AggregateError([error, cleanup], 'B2 environment export failed; owned recovery state retained'); }
      throw error;
    }
    return;
  }
  if (Array.isArray(args) && args.length === 2 && args[0] === 'b2-fixture-stop') return stopB2TestDatabase({ stateFile: args[1], execute });
  throw new Error('Usage: b2-fixture-start <GITHUB_ENV> <state-file> | b2-fixture-stop <state-file>');
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  if (process.argv.length > 2) await runB2FixtureCommand(process.argv.slice(2));
  else await runAcceptance();
}
