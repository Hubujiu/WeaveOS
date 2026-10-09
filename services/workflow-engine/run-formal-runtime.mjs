// Root-owned acceptance: actual BFF and WorkflowEngineMain OS processes.
// No deployment, published ports, Docker socket in runner, or live credentials.
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdirSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
process.chdir(root);
if (process.platform !== 'linux') throw Error('Linux isolated fixture required');
const engine = resolve('services/workflow-engine');
const cache = realpathSync(resolve(engine, '.work'));
const dir = resolve('.work', `v041-formal-${Date.now()}-${process.pid}`);
mkdirSync(dir, { recursive: true, mode: 0o700 });
const reportDir = resolve(engine, 'formal-runtime-reports');
mkdirSync(reportDir, { recursive: true });
const go = process.env.WEAVEOS_FORMAL_GO || 'go';
if (!process.env.WEAVEOS_FORMAL_GOOSE) throw Error('Explicit pinned Goose executable required');
const goose = realpathSync(process.env.WEAVEOS_FORMAL_GOOSE);
const env = { ...process.env, GOTOOLCHAIN: 'local', GOFLAGS: '-buildvcs=false' };
const call = (cmd, args, options = {}) => execFileSync(cmd, args, { cwd: root, env, stdio: 'pipe', maxBuffer: 32 * 1024 * 1024, ...options });
const step = (name, cmd, args, options = {}) => {
  const result = spawnSync(cmd, args, { cwd: root, env, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024, ...options });
  writeFileSync(resolve(dir, `${name}.stdout`), result.stdout || '', { mode: 0o600 });
  writeFileSync(resolve(dir, `${name}.stderr`), result.stderr || '', { mode: 0o600 });
  if (result.status !== 0) throw Error(`${name} failed; isolated evidence retained`);
  return result;
};
const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const prefix = `weaveos-v041-formal-${Date.now()}-${process.pid}`;
const network = `${prefix}-net`, enginePG = `${prefix}-engine-pg`, redis = `${prefix}-redis`;
const runner = `${prefix}-runner`, migration = `${prefix}-migration`;
const pgImage = 'docker.io/library/postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722';
const mavenImage = 'mirror.gcr.io/library/maven@sha256:fa7aa19829157d299ff05f631b51697a388dcd2f6955e84249ecc652015f217b';
const redisImage = 'redis:8.2.10@sha256:164c759a0c342ee69d08fc99219382b0fd682181465c0df2e0e6911f4c85d73c';
const owned = [];
let app, networkCreated = false;
async function waitFor(probe, description) {
  for (let i = 0; i < 120; i++) { try { if (probe()) return; } catch {} await delay(250); }
  throw Error(`${description} did not become ready`);
}
try {
  if (!call(go, ['version'], { encoding: 'utf8' }).includes('go1.27.2 ')) throw Error('Pinned Go compiler required');
  step('build-bff', go, ['build', '-o', resolve(engine, '.work/formal-bff'), './cmd/bff'], { cwd: resolve('services/bff'), env: { ...env, CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' } });
  step('build-tests', go, ['test', '-tags', 'workflowruntime_integration,workflowrpc_integration', '-c', '-o', resolve(engine, '.work/formal-runtime.test'), './cmd/bff'], { cwd: resolve('services/bff'), env: { ...env, CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' } });
  const { startB2TestDatabase } = await import(pathToFileURL(resolve('infra/acceptance/run.mjs')).href);
  app = await startB2TestDatabase({ stateFile: resolve(dir, 'app-state.json') });
  owned.push(app.container);
  await waitFor(() => call('docker', ['exec', app.container, 'sh', '-ec', 'test "$(cat /proc/1/comm)" = postgres; psql -X -U weaveos_b2_test -d weaveos_b2_isolated_test -tAc "SELECT 1"'], { encoding: 'utf8' }).trim() === '1', 'application PostgreSQL');
  const archive = new URL(app.url); archive.pathname = '/weaveos_formal_archive';
  call('docker', ['exec', app.container, 'createdb', '-U', 'weaveos_b2_test', archive.pathname.slice(1)]);
  for (const [folder, dsn, name] of [['db/archive-migrations', archive.href, 'archive'], ['db/migrations', app.url, 'app']]) step(`migrate-${name}`, goose, ['-dir', resolve(folder), 'postgres', dsn, 'up']);
  step('app-roles', 'docker', ['exec', '-i', app.container, 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'weaveos_b2_test', '-d', 'weaveos_b2_isolated_test'], { input: readFileSync('infra/runtime/roles.sql', 'utf8') });

  const redisSocket = resolve(dir, 'redis'); mkdirSync(redisSocket, { mode: 0o700 });
  call('docker', ['run', '-d', '--name', redis, '--network', 'none', '--user', `${process.getuid()}:${process.getgid()}`, '-v', `${redisSocket}:/socket`, '--workdir', '/socket', '--entrypoint', 'redis-server', redisImage, '--port', '0', '--unixsocket', '/socket/redis.sock', '--unixsocketperm', '700', '--save', '', '--appendonly', 'no']);
  owned.push(redis);
  await waitFor(() => call('docker', ['exec', redis, 'redis-cli', '-s', '/socket/redis.sock', 'ping'], { encoding: 'utf8' }).trim() === 'PONG', 'isolated Redis');

  call('docker', ['network', 'create', '--internal', '--label', 'weaveos.package=V030-041', network]); networkCreated = true;
  call('docker', ['run', '-d', '--name', enginePG, '--network', network, '--network-alias', 'b3-postgres', '--label', 'weaveos.package=V030-041', '--tmpfs', '/var/lib/postgresql:rw', '-e', 'POSTGRES_DB=b3_flowable_fixture', '-e', 'POSTGRES_USER=b3_fixture', '-e', 'POSTGRES_PASSWORD=b3_fixture_only', pgImage]);
  owned.push(enginePG);
  const owner = args => call('docker', ['exec', '-i', enginePG, 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'b3_fixture', '-d', 'b3_flowable_fixture', ...args], { encoding: 'utf8' });
  await waitFor(() => call('docker', ['exec', enginePG, 'sh', '-ec', 'test "$(cat /proc/1/comm)" = postgres; psql -X -U b3_fixture -d b3_flowable_fixture -tAc "SELECT 1"'], { encoding: 'utf8' }).trim() === '1', 'engine PostgreSQL');
  owner(['-c', 'CREATE SCHEMA workflow AUTHORIZATION b3_fixture;']);
  step('engine-migration', 'docker', ['run', '--rm', '--name', migration, '--network', network, '--user', `${process.getuid()}:${process.getgid()}`, '-v', `${goose}:/goose:ro`, '-v', `${engine}/schema/migrations:/migrations:ro`, '--entrypoint', '/goose', mavenImage, '-dir', '/migrations', '-table', 'workflow.goose_db_version', 'postgres', 'postgres://b3_fixture:b3_fixture_only@b3-postgres:5432/b3_flowable_fixture?sslmode=disable&search_path=workflow', 'up']);
  step('engine-role', 'docker', ['exec', '-i', enginePG, 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'b3_fixture', '-d', 'b3_flowable_fixture'], { input: readFileSync(resolve(engine, 'schema/formal-runtime-fixture-role.sql'), 'utf8') });
  if (owner(['-tAc', "SELECT count(*) FROM pg_tables WHERE schemaname='workflow' AND left(tablename,4) IN ('act_','flw_')"]).trim() !== '32') throw Error('native schema not explicitly installed');

  step('runtime-compile', 'docker', ['run', '--rm', '--user', `${process.getuid()}:${process.getgid()}`, '--network', network, '-v', `${engine}:/proof`, '-v', `${cache}:${cache}`, '-v', `${engine}/.work/m2:/m2`, '-w', '/proof', '--entrypoint', 'mvn', mavenImage, '-B', '-ntp', '-o', '-s', 'maven-settings.xml', '-Duser.home=/tmp', '-Dmaven.repo.local=/m2', 'compile', 'org.apache.maven.plugins:maven-dependency-plugin:3.9.0:build-classpath', '-Dmdep.outputFile=.work/formal-classpath', '-Dmdep.includeScope=runtime']);
  const classpath = '/proof/target/classes:' + readFileSync(resolve(engine, '.work/formal-classpath'), 'utf8').trim();
  call('docker', ['run', '-d', '--name', runner, '--user', `${process.getuid()}:${process.getgid()}`, '--network', network, '--label', 'weaveos.package=V030-041', '--memory', '2g', '-v', `${engine}:/proof`, '-v', `${cache}:${cache}`, '-v', `${engine}/.work/m2:/m2:ro`, '-v', `${root}:${root}`, '-v', `${app.socket}:${app.socket}`, '-w', resolve('services/bff/cmd/bff'), '--entrypoint', 'sleep', mavenImage, 'infinity']);
  owned.push(runner);
  const isolation = { networkInternal: call('docker', ['network', 'inspect', '--format', '{{.Internal}}', network], { encoding: 'utf8' }).trim(), containers: [] };
  if (isolation.networkInternal !== 'true') throw Error('test network is not internal');
  for (const name of owned) {
    const info = JSON.parse(call('docker', ['inspect', name], { encoding: 'utf8' }))[0];
    if (Object.keys(info.HostConfig.PortBindings || {}).length || info.Mounts.some(m => m.Source === '/var/run/docker.sock')) throw Error('fixture isolation violated');
    isolation.containers.push({ name, network: info.HostConfig.NetworkMode, ports: info.HostConfig.PortBindings });
  }
  writeFileSync(resolve(reportDir, 'isolation.json'), JSON.stringify(isolation, null, 2));
  const result = spawnSync('docker', ['exec', '-e', `WEAVEOS_TEST_DATABASE_URL=${app.url}`, '-e', `WEAVEOS_TEST_ARCHIVE_DATABASE_URL=${archive.href}`, '-e', `WEAVEOS_TEST_REDIS_URL=unix://${redisSocket}/redis.sock?db=15`, '-e', 'WEAVEOS_FORMAL_BFF_BINARY=/proof/.work/formal-bff', '-e', `WEAVEOS_FORMAL_JAVA_CLASSPATH=${classpath}`, runner, '/proof/.work/formal-runtime.test', '-test.v=test2json', '-test.run', '^TestRootFormalRuntime', '-test.timeout=300s'], { cwd: root, env, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
  writeFileSync(resolve(dir, 'test.stdout'), result.stdout || '', { mode: 0o600 });
  writeFileSync(resolve(dir, 'test.stderr'), result.stderr || '', { mode: 0o600 });
  const converted = step('test2json', go, ['tool', 'test2json', '-t', '-p', 'github.com/Hubujiu/WeaveOS/services/bff/cmd/bff'], { input: result.stdout || '' });
  writeFileSync(resolve(reportDir, 'tests.jsonl'), converted.stdout);
  const summary = { head: call('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(), exit: result.status ?? 1, scope: 'actual BFF and WorkflowEngineMain OS processes, real HTTPS ingress and authenticated loopback gRPC; isolated app/engine databases; no deployment or public new-instance trigger' };
  writeFileSync(resolve(reportDir, 'summary.json'), JSON.stringify(summary, null, 2));
  console.log(JSON.stringify(summary));
  process.exitCode = result.status ?? 1;
} finally {
  for (const name of owned) { try { writeFileSync(resolve(dir, `${name}.log`), call('docker', ['logs', name]), { mode: 0o600 }); } catch {} }
  // Only names created successfully by this invocation are removed; never prune.
  for (const name of owned.reverse()) { try { call('docker', ['rm', '-f', '-v', name]); } catch {} }
  try { call('docker', ['rm', '-f', migration]); } catch {}
  if (networkCreated) { try { call('docker', ['network', 'rm', network]); } catch {} }
  console.log('Formal-runtime isolated evidence: ' + dir);
}
