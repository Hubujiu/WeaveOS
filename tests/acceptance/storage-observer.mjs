import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync, unlinkSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { backupDatabase, restoreDatabase, privateFile } from '../../infra/runtime/backup.mjs';
import { recoverRuntime } from '../../infra/runtime/recovery.mjs';

const literal = value => `'${String(value).replaceAll("'", "''")}'`;
const digest = value => createHash('sha256').update(Buffer.from(value, 'base64url')).digest('hex');
const envFile = file => Object.fromEntries(readFileSync(file, 'utf8').trim().split(/\r?\n/).map(line => { const i = line.indexOf('='); return [line.slice(0, i), line.slice(i + 1)]; }));

// Test-side control only. No Docker socket, observer endpoint or fault switch is
// installed in the BFF. All commands require the runner's private local namespace.
export async function open({ baseURL }) {
  assert.ok(['localhost', '127.0.0.1'].includes(new URL(baseURL).hostname), 'loopback target required');
  const project = process.env.WEAVEOS_ACCEPTANCE_PROJECT;
  assert.match(project ?? '', /^weaveos-v010-007-\d+$/);
  const dir = dirname(resolve(process.env.WEAVEOS_ACCEPTANCE_FIXTURES));
  const runtimeFile = resolve(dir, 'runtime.env');
  const runtime = envFile(runtimeFile), pg = new URL(runtime.WEAVEOS_DATABASE_URL);
  assert.equal(pg.hostname, 'postgres');
  assert.match(pg.pathname.slice(1), /^weaveos_[a-zA-Z0-9_]+$/);
  assert.match(pg.username, /^weaveos_[a-zA-Z0-9_]+$/);
  let generation = runtime.WEAVEOS_SESSION_GENERATION, database = pg.pathname.slice(1);
  assert.ok(generation === project || /^recovered_[0-9a-f]{32}$/.test(generation));
  const docker = (args, input) => {
    try { return execFileSync('docker', args, { input, encoding: 'utf8', stdio: 'pipe', timeout: 120000, maxBuffer: 32 * 1024 * 1024 }).trim(); }
    catch { throw new Error('Isolated storage command failed; details withheld'); }
  };
  const containers = Object.fromEntries(['postgres', 'redis', 'bff'].map(service => {
    const name = `${project}-${service}-1`, info = JSON.parse(docker(['inspect', name]))[0];
    assert.equal(info.Config.Labels['com.docker.compose.project'], project);
    assert.equal(info.Config.Labels['com.docker.compose.service'], service);
    assert.ok(!Object.values(info.NetworkSettings.Ports ?? {}).some(Boolean), 'storage/application ports must be private');
    if (service === 'bff') {
      assert.ok(info.Config.Env.includes(`WEAVEOS_PUBLIC_ORIGIN=${new URL(baseURL).origin}`), 'HTTP target must be this isolated BFF origin');
      assert.ok(info.Config.Env.includes(`WEAVEOS_DATABASE_URL=${runtime.WEAVEOS_DATABASE_URL}`), 'observer and BFF must use the same database');
      assert.ok(info.Config.Env.includes(`WEAVEOS_SESSION_GENERATION=${generation}`), 'observer and BFF must use the same session generation');
    }
    return [service, name];
  }));
  const sql = (query, db = database) => docker(['exec', '-i', containers.postgres, 'psql', '-X', '-At', '-v', 'ON_ERROR_STOP=1', '-U', pg.username, '-d', db], query);
  const redis = (...args) => JSON.parse(docker(['exec', containers.redis, 'redis-cli', '--json', ...args.map(String)]));
  const key = auth => {
    assert.match(auth, /^__Host-session=[A-Za-z0-9_-]{43}$/);
    const sid = auth.split('=')[1];
    assert.equal(Buffer.from(sid, 'base64url').toString('base64url'), sid);
    return `ems:auth:session:${generation}:v1:${digest(sid)}`;
  };
  const cleanups = [];
  const cleanup = fn => { let pending = true; const once = async () => { if (pending) { await fn(); pending = false; } }; cleanups.push(once); return once; };
  const count = query => Number(sql(query));
  const compose = (...args) => docker(['compose', '-p', project, '-f', process.env.WEAVEOS_ACCEPTANCE_COMPOSE ?? resolve('infra/acceptance/compose.json'), ...args]);
  // Compose interpolation is restricted to this verified fixture directory.
  process.env.WEAVEOS_ACCEPTANCE_DIR = dir;
  const configure = values => {
    Object.assign(runtime, values);
    writeFileSync(runtimeFile, Object.entries(runtime).map(([k,v]) => `${k}=${v}`).join('\n')+'\n');
  };
  const ready = async () => {
    let healthySince;
    for (let i=0;i<100;i++) {
      try {
        const response = await fetch(new URL('/health/ready',baseURL), {headers:{Connection:'close'},signal:AbortSignal.timeout(1000)});
        await response.arrayBuffer();
        if (response.status === 200) {
          healthySince ??= Date.now();
          // Require stable dependency readiness across ingress's one-second DNS TTL.
          if (Date.now()-healthySince >= 1500) return;
        } else healthySince = undefined;
      } catch { healthySince = undefined; }
      await new Promise(resolve => setTimeout(resolve,100));
    }
    throw new Error('BLOCKED: isolated ingress did not recover');
  };
  const snapshots = new WeakSet();
  return {
    storage: 'isolated-postgresql-and-redis',
    async close() { for (const fn of cleanups.reverse()) await fn(); },
    async sessionPTTL(auth) { return redis('PTTL', key(auth)); },
    async expireSession(auth) { assert.equal(redis('PEXPIRE', key(auth), 0), 1); },
    async setSessionPTTL(auth, ttl) { assert.ok(ttl > 0 && ttl <= 3600000); assert.equal(redis('PEXPIRE', key(auth), ttl), 1); },
    async sessionCreatedAt(auth) { return JSON.parse(redis('GET', key(auth))).created_at_unix_ms; },
    async setSessionCreationAge(auth, age) {
      assert.ok(Number.isSafeInteger(age) && age > 0);
      const record = JSON.parse(redis('GET', key(auth)));
      record.created_at_unix_ms = Date.now() - age;
      assert.equal(redis('SET', key(auth), JSON.stringify(record), 'XX', 'KEEPTTL'), 'OK');
    },
    async setUserStatus(id, status) {
      assert.match(id, /^[0-9a-f-]{36}$/); assert.equal(status, 'disabled');
      assert.equal(sql(`UPDATE auth.users SET status='disabled', auth_version=auth_version+1, updated_at=now() WHERE id=${literal(id)} RETURNING status`).split('\n')[0], 'disabled');
    },
    async disconnectApplication(dependency) {
      assert.ok(['postgresql', 'redis'].includes(dependency));
      if (dependency === 'postgresql') {
        const original = docker(['exec', containers.postgres, 'sh', '-c', 'cat "$PGDATA/pg_hba.conf"']);
        const address = JSON.parse(docker(['inspect', containers.bff]))[0].NetworkSettings.Networks[`${project}_default`].IPAddress;
        assert.match(address, /^\d+\.\d+\.\d+\.\d+$/);
        const write = text => docker(['exec', '-i', containers.postgres, 'sh', '-c', 'cat > "$PGDATA/pg_hba.conf"'], text + '\n');
        const restore = cleanup(async () => { write(original); sql('SELECT pg_reload_conf()'); await new Promise(resolve => setTimeout(resolve, 200)); });
        write(`host all all ${address}/32 reject\n${original}`);
        sql('SELECT pg_reload_conf()');
        await new Promise(resolve => setTimeout(resolve, 200));
        sql(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE client_addr=${literal(address)}::inet AND pid<>pg_backend_pid()`);
        return restore;
      }
      const service = dependency === 'postgresql' ? 'postgres' : 'redis', name = containers[service], network = `${project}_default`;
      const restore = cleanup(async () => {
        docker(['network', 'connect', '--alias', service, network, name]);
        await new Promise(resolve => setTimeout(resolve, 1500));
      });
      docker(['network', 'disconnect', network, name]);
      if (service === 'redis') redis('CLIENT', 'KILL', 'TYPE', 'normal', 'SKIPME', 'yes');
      else sql('SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid<>pg_backend_pid() AND backend_type=\'client backend\'');
      return restore;
    },
    async failNextRegistrationBeforeCommit(account) {
      assert.match(account, /^test-[0-9a-f-]{36}$/);
      const name = `observer_${randomBytes(8).toString('hex')}`;
      const release = cleanup(async () => sql(`DROP TRIGGER IF EXISTS ${name} ON auth.authentication_events; DROP FUNCTION IF EXISTS auth.${name}()`));
      sql(`CREATE FUNCTION auth.${name}() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.event_type='register' AND NEW.outcome='success' AND EXISTS (SELECT 1 FROM auth.users WHERE id=NEW.subject_user_id AND account=${literal(account)}) THEN RAISE EXCEPTION 'isolated commit fault' USING ERRCODE='P0001'; END IF; RETURN NEW; END $$; CREATE CONSTRAINT TRIGGER ${name} AFTER INSERT ON auth.authentication_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION auth.${name}()`);
      return release;
    },
    async countUsersByExactAccount(account) { return count(`SELECT count(*) FROM auth.users WHERE account=${literal(account)}`); },
    async countCredentialsByExactAccount(account) { return count(`SELECT count(*) FROM auth.password_credentials p JOIN auth.users u ON p.user_id=u.id WHERE u.account=${literal(account)}`); },
    async countInvitationUses(code) { return count(`SELECT count(*) FROM auth.invitations WHERE code_hash=decode(${literal(digest(code))},'hex') AND used_by IS NOT NULL`); },
    async readPasswordHash(account) { return sql(`SELECT p.password_hash FROM auth.password_credentials p JOIN auth.users u ON p.user_id=u.id WHERE u.account=${literal(account)}`); },
    async dumpOwnedAuthenticationState(accounts) {
      const list = accounts.map(literal).join(',');
      return sql(`SELECT jsonb_build_object('user',to_jsonb(u),'credential',to_jsonb(p),'invitation',to_jsonb(i),'event',to_jsonb(e)) FROM auth.users u LEFT JOIN auth.password_credentials p ON p.user_id=u.id LEFT JOIN auth.invitations i ON i.used_by=u.id LEFT JOIN auth.authentication_events e ON e.subject_user_id=u.id WHERE u.account IN (${list})`);
    },
    async readLoginEvents({ userId, since }) {
      return JSON.parse(sql(`SELECT coalesce(jsonb_agg(jsonb_build_object('result',outcome,'userId',subject_user_id,'accountIdentifier',account_fingerprint,'ip',host(client_ip),'userAgent',user_agent,'time',occurred_at)),'[]'::jsonb) FROM auth.authentication_events WHERE event_type='login' AND subject_user_id=${literal(userId)} AND occurred_at>=${literal(new Date(since.getTime()-1000).toISOString())}`));
    },
    async readApplicationLogs({ since }) {
      const result = spawnSync('docker',['logs','--since',since.toISOString(),containers.bff],{encoding:'utf8',stdio:'pipe',timeout:10000,maxBuffer:32*1024*1024});
      if (result.status !== 0) throw new Error('Isolated log read failed; details withheld');
      return result.stdout + result.stderr;
    },
    async holdNextRenewal(auth) {
      const name = `${project}-observer-gate`, original = runtime.WEAVEOS_REDIS_URL;
      docker(['run','-d','--name',name,'--network',`${project}_default`,'--network-alias','observer-gate','-p','127.0.0.1::6380','--mount',`type=bind,src=${resolve('tests/acceptance/redis-gate.mjs')},dst=/gate.mjs,readonly`,'node:24.14.0-bookworm-slim','node','/gate.mjs','--serve']);
      cleanup(async () => { configure({WEAVEOS_REDIS_URL:original}); compose('up','-d','--no-deps','bff'); await ready(); docker(['rm','-f',name]); });
      const port = JSON.parse(docker(['inspect',name]))[0].NetworkSettings.Ports['6380/tcp'][0].HostPort;
      const url = `http://127.0.0.1:${port}`;
      configure({WEAVEOS_REDIS_URL:'redis://observer-gate:6379/0'});
      compose('up','-d','--no-deps','bff'); await ready();
      assert.equal((await fetch(url+'/arm',{method:'POST',body:JSON.stringify({key:key(auth)})})).status,200);
      return {
        async waitUntilHeld() {
          for (let i=0;i<100;i++) {
            if ((await (await fetch(url+'/status')).json()).observed) return;
            await new Promise(resolve=>setTimeout(resolve,5));
          }
          throw new Error('BLOCKED: real renewal did not reach transport barrier');
        },
        async release() { assert.equal((await fetch(url+'/release',{method:'POST'})).status,200); },
      };
    },
    async snapshotIsolatedStorage(auth) {
      const suffix = randomBytes(8).toString('hex');
      const backup = {container:containers.postgres,user:pg.username,database,keyFile:resolve(dir,`store-key-${suffix}`),backupFile:resolve(dir,`store-backup-${suffix}`)};
      privateFile(backup.keyFile,randomBytes(32)); backupDatabase(backup);
      cleanup(async () => { unlinkSync(backup.keyFile); unlinkSync(backup.backupFile); });
      const records = [key(auth)].map(name => {
        const dump = execFileSync('docker',['exec',containers.redis,'redis-cli','--raw','DUMP',name],{stdio:'pipe'});
        return {name,ttl:redis('PTTL',name),dump:dump.subarray(0,dump.length-1)};
      });
      const handle = {backup,records,generation,accounts:sql('SELECT coalesce(jsonb_agg(account ORDER BY account),\'[]\') FROM auth.users')};
      snapshots.add(handle); return handle;
    },
    async restoreThroughRecoveryProcedure(handle) {
      assert.ok(snapshots.has(handle));
      const restored = `weaveos_store_${randomBytes(8).toString('hex')}`;
      recoverRuntime({
        generation,
        pause() { compose('stop','bff'); },
        restore() {
          sql(`CREATE DATABASE ${restored}`,'postgres');
          restoreDatabase({...handle.backup,database:restored});
          for (const record of handle.records) {
            assert.ok(record.ttl>0);
            try {
              execFileSync('docker',['exec','-i',containers.redis,'redis-cli','--json','-x','EVAL',"return redis.call('RESTORE',KEYS[1],ARGV[1],ARGV[2],'REPLACE')",'1',record.name,String(record.ttl)],{input:record.dump,stdio:'pipe'});
            } catch { throw new Error('Isolated Redis snapshot restore failed; details withheld'); }
          }
        },
        switchGeneration(next) {
          generation=next; database=restored; pg.pathname='/'+restored;
          configure({WEAVEOS_DATABASE_URL:pg.href,WEAVEOS_SESSION_GENERATION:next});
        },
        resume() { compose('up','-d','--no-deps','bff'); },
      });
      await ready();
    },
    async recoveryEvidence(handle) {
      assert.ok(snapshots.has(handle));
      return {generationChanged:generation!==handle.generation,oldSessionRestored:handle.records.every(r=>redis('EXISTS',r.name)===1),databaseRestored:database!==handle.backup.database && sql('SELECT coalesce(jsonb_agg(account ORDER BY account),\'[]\') FROM auth.users')===handle.accounts};
    },
  };
}
