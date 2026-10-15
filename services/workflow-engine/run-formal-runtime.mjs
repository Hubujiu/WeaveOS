// Root-owned acceptance: actual BFF and WorkflowEngineMain OS processes.
// No deployment, published ports, Docker socket in runner, or live credentials.
import { execFileSync, spawnSync, spawn } from 'node:child_process';
import { mkdirSync, readFileSync, realpathSync, writeFileSync, existsSync, renameSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
process.chdir(root);
if (process.platform !== 'linux') throw Error('Linux isolated fixture required');
const engine = resolve('services/workflow-engine');
const cache = realpathSync(resolve(engine, '.work'));
const dir = resolve('.work', `v041-formal-${Date.now()}-${process.pid}`);
mkdirSync(dir, { recursive: true, mode: 0o700 });
const browserMode = process.env.WEAVEOS_PRODUCT_BROWSER_MODE === '1';
const reportDir = resolve(engine, browserMode ? 'product-browser-reports' : 'formal-runtime-reports');
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
  step('build-tests', go, ['test', '-tags', browserMode ? 'workflowbrowser_integration,workflowruntime_integration,workflowrpc_integration' : 'workflowruntime_integration,workflowrpc_integration', '-c', '-o', resolve(engine, '.work/formal-runtime.test'), './cmd/bff'], { cwd: resolve('services/bff'), env: { ...env, CGO_ENABLED: '0', GOOS: 'linux', GOARCH: 'amd64' } });
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
  if (browserMode) {
    const {browserCommand,runBrowserJourneys} = await import('./product-browser-runner.mjs');
    const {validateFixture,validateReports,validateDefinitionFixture,validateDefinitionReports,validateCompleteReports} = await import('./product-browser-contract.mjs');
    const head=call('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim();
    const image='mcr.microsoft.com/playwright:v1.64.1-noble@sha256:7852dc5a7d72f5fe458a597d690f6c42fbc8e9e26ddfa95f8e6ef552558d9bbf';
    // Download the fixed public browser image before a bounded backend fixture starts.
    step('browser-image','docker',['pull',image]);
    const proofs={};
    for(const journey of ['approval','definition']) {
    const phaseDir=journey==='approval'?reportDir:resolve(reportDir,'definition');mkdirSync(phaseDir,{recursive:true});
    const exchange=resolve(dir,'private-browser-exchange-'+journey);mkdirSync(exchange,{mode:0o700});
    const validate=journey==='approval'?validateFixture:validateDefinitionFixture;
    const testName=journey==='approval'?'TestRootProductBrowserJourney':'TestRootProductDefinitionJourney';
    const config=journey==='approval'?'services/workflow-engine/playwright.product.config.ts':'services/workflow-engine/playwright.definition.config.ts';
    step('browser-discovery',process.execPath,['node_modules/@playwright/test/cli.js','test','--config',config,'--list'],{env:{...env,WEAVEOS_PRODUCT_BROWSER_REPORT:resolve(phaseDir,'baseline.json'),WEAVEOS_PRODUCT_BROWSER_OUTPUT:resolve(phaseDir,'discovery')}});
    const baseline=JSON.parse(readFileSync(resolve(phaseDir,'baseline.json'),'utf8'));
    let results;
    try {
      results=await runBrowserJourneys({
        async start(browser) {
          const args=['exec','-e',`WEAVEOS_TEST_DATABASE_URL=${app.url}`,'-e',`WEAVEOS_TEST_ARCHIVE_DATABASE_URL=${archive.href}`,'-e',`WEAVEOS_TEST_REDIS_URL=unix://${redisSocket}/redis.sock?db=15`,'-e','WEAVEOS_FORMAL_BFF_BINARY=/proof/.work/formal-bff','-e',`WEAVEOS_FORMAL_JAVA_CLASSPATH=${classpath}`,'-e',`WEAVEOS_PRODUCT_BROWSER=${browser}`,'-e',`WEAVEOS_PRODUCT_HEAD=${head}`,'-e',`WEAVEOS_PRODUCT_EXCHANGE_DIR=${exchange}`,'-e',`WEAVEOS_PRODUCT_WEB_ROOT=${resolve('apps/web/dist')}`,runner,'/proof/.work/formal-runtime.test','-test.v=test2json','-test.run','^'+testName+'$','-test.timeout=300s'];
          const child=spawn('docker',args,{cwd:root,env,stdio:['ignore','pipe','pipe']});
          const handle={browser,child,stdout:'',stderr:'',closed:false,secrets:[]};
          for(const key of ['stdout','stderr'])child[key].on('data',chunk=>{handle[key]+=chunk.toString();if(handle[key].length>32*1024*1024)child.kill();});
          handle.exit=new Promise(resolve=>{child.once('error',()=>{handle.closed=true;resolve(1);});child.once('close',code=>{handle.closed=true;resolve(code??1);});});
          return handle;
        },
        async ready(handle) {
          const path=resolve(exchange,handle.browser+'.request.json');
          for(let i=0;i<120;i++){
            if(existsSync(path)){
              let fixture;try{fixture=validate(JSON.parse(readFileSync(path,'utf8')),head,handle.browser);}catch{throw Error('Private browser fixture rejected');}
              handle.secrets=['session','csrf','otherSession','otherCsrf'].map(k=>fixture[k]);return;
            }
            if(handle.closed)throw Error('Actual browser backend fixture exited before ready');
            await delay(250);
          }
          throw Error('Actual browser backend fixture did not become ready');
        },
        async browser(handle) {
          step('browser-'+handle.browser,'docker',browserCommand({root,output:phaseDir,exchange,runner,uid:process.getuid(),gid:process.getgid(),head,browser:handle.browser,journey}));
          return JSON.parse(readFileSync(resolve(phaseDir,handle.browser+'.json'),'utf8'));
        },
        async signal(handle,success) {
          const path=resolve(exchange,handle.browser+'.done.json');
          writeFileSync(path+'.tmp',JSON.stringify({browser:handle.browser,success}),{mode:0o600});renameSync(path+'.tmp',path);
        },
        async finish(handle) {
          const status=await handle.exit;
          let raw=handle.stdout;
          // Native events retain their identities/status. Remove synthetic session tokens if any diagnostic unexpectedly contains one.
          for(const secret of handle.secrets)raw=raw.split(secret).join('[redacted synthetic Session]');
          const converted=step('browser-'+handle.browser+'-test2json',go,['tool','test2json','-t','-p','github.com/Hubujiu/WeaveOS/services/bff/cmd/bff'],{input:raw});
          writeFileSync(resolve(phaseDir,handle.browser+'-go.jsonl'),converted.stdout);
          if(status!==0)throw Error('Actual browser backend assertions failed: '+handle.browser);
          return {head,browser:handle.browser,events:converted.stdout.trim().split('\n').map(line=>JSON.parse(line))};
        },
      });
    } finally {
      // This exact directory was created by this invocation; it is never an artifact source.
      rmSync(exchange,{recursive:true,force:true});
    }
    proofs[journey]={head,baseline,reports:results.map(r=>r.report),backend:results.map(r=>r.backend)};
    const summary=(journey==='approval'?validateReports:validateDefinitionReports)(proofs[journey]);
    writeFileSync(resolve(phaseDir,'summary.json'),JSON.stringify(summary,null,2));console.log(JSON.stringify(summary));
    }
    const complete=validateCompleteReports(proofs);
    writeFileSync(resolve(reportDir,'complete-summary.json'),JSON.stringify(complete,null,2));console.log(JSON.stringify(complete));
  } else {
  const result = spawnSync('docker', ['exec', '-e', `WEAVEOS_TEST_DATABASE_URL=${app.url}`, '-e', `WEAVEOS_TEST_ARCHIVE_DATABASE_URL=${archive.href}`, '-e', `WEAVEOS_TEST_REDIS_URL=unix://${redisSocket}/redis.sock?db=15`, '-e', 'WEAVEOS_FORMAL_BFF_BINARY=/proof/.work/formal-bff', '-e', `WEAVEOS_FORMAL_JAVA_CLASSPATH=${classpath}`, runner, '/proof/.work/formal-runtime.test', '-test.v=test2json', '-test.run', '^TestRootFormalRuntime', '-test.timeout=300s'], { cwd: root, env, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 });
  writeFileSync(resolve(dir, 'test.stdout'), result.stdout || '', { mode: 0o600 });
  writeFileSync(resolve(dir, 'test.stderr'), result.stderr || '', { mode: 0o600 });
  const converted = step('test2json', go, ['tool', 'test2json', '-t', '-p', 'github.com/Hubujiu/WeaveOS/services/bff/cmd/bff'], { input: result.stdout || '' });
  writeFileSync(resolve(reportDir, 'tests.jsonl'), converted.stdout);
  const summary = { head: call('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim(), exit: result.status ?? 1, scope: 'actual BFF and WorkflowEngineMain OS processes, real HTTPS ingress and authenticated loopback gRPC; isolated app/engine databases; no deployment or public new-instance trigger' };
  writeFileSync(resolve(reportDir, 'summary.json'), JSON.stringify(summary, null, 2));
  console.log(JSON.stringify(summary));
  process.exitCode = result.status ?? 1;
  }
} finally {
  for (const name of owned) { try { writeFileSync(resolve(dir, `${name}.log`), call('docker', ['logs', name]), { mode: 0o600 }); } catch {} }
  // Only names created successfully by this invocation are removed; never prune.
  for (const name of owned.reverse()) { try { call('docker', ['rm', '-f', '-v', name]); } catch {} }
  try { call('docker', ['rm', '-f', migration]); } catch {}
  if (networkCreated) { try { call('docker', ['network', 'rm', network]); } catch {} }
  console.log('Formal-runtime isolated evidence: ' + dir);
}
