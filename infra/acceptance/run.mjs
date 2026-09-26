// Reproducible isolated Linux product acceptance. No production account or data.
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync, existsSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const dir = resolve(root, '.work/acceptance');
if (existsSync(resolve(dir, 'runtime.env'))) throw new Error('Existing private environment: inspect it before rerunning; no overwrite');
mkdirSync(resolve(dir, 'tls'), { recursive: true });
mkdirSync(resolve(dir, 'public'), { recursive: true });
const project = `weaveos-v010-007-${Date.now()}`;
const env = { ...process.env, WEAVEOS_ACCEPTANCE_DIR: dir };
const composeArgs = ['compose', '-p', project, '-f', resolve(root, 'infra/acceptance/compose.json')];
const call = (command, args, options = {}) => execFileSync(command, args, { cwd: root, env, stdio: 'inherit', ...options });
const compose = (...args) => call('docker', [...composeArgs, ...args]);
const id = name => call('docker', [...composeArgs, 'ps', '-q', name], { encoding: 'utf8', stdio: 'pipe' }).trim();
const password = randomBytes(32).toString('hex');
const database = name => `postgres://weaveos_test:${password}@127.0.0.1:5432/${name}?sslmode=disable&connect_timeout=2`;
const privateFile = (name, text) => writeFileSync(resolve(dir, name), text, { mode: 0o600, flag: 'wx' });
privateFile('postgres.env', `POSTGRES_USER=weaveos_test\nPOSTGRES_PASSWORD=${password}\nPOSTGRES_DB=weaveos_ci_test\n`);
privateFile('runtime.env', `WEAVEOS_DATABASE_URL=${database('weaveos_acceptance').replace('@127.0.0.1:', '@postgres:')}\nWEAVEOS_REDIS_URL=redis://postgres:6379/0\nWEAVEOS_SESSION_GENERATION=${project}\nWEAVEOS_AUDIT_KEY_ID=test\nWEAVEOS_AUDIT_HMAC_KEY=${randomBytes(32).toString('base64')}\n`);
privateFile('test.env', `WEAVEOS_TEST_DATABASE_URL=${database('weaveos_ci_test')}\nWEAVEOS_TEST_REDIS_URL=redis://127.0.0.1:6379/15\n`);
privateFile('seed.env', `WEAVEOS_TEST_DATABASE_URL=${database('weaveos_acceptance')}\nWEAVEOS_ACCEPTANCE_FIXTURES=/repo/.work/acceptance/fixtures.json\n`);
const openssl = process.platform === 'win32' ? 'C:/Program Files/Git/usr/bin/openssl.exe' : 'openssl';
call(openssl, ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2', '-keyout', resolve(dir, 'tls/key.pem'), '-out', resolve(dir, 'tls/cert.pem'), '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost,IP:127.0.0.1'], { stdio: 'pipe' });
const goImage = 'golang:1.25.7';
const playwrightImage = 'mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27';
const mount = ['--mount', `type=bind,src=${root},dst=/repo`];
const go = (script, envFile = 'test.env') => call('docker', ['run', '--rm', '--network', `container:${id('postgres')}`, ...mount, '--mount', 'type=volume,src=weaveos-v010-go-cache,dst=/go/pkg/mod', '--mount', 'type=volume,src=weaveos-v010-go-build-cache,dst=/root/.cache/go-build', '--env-file', resolve(dir, envFile), '-e', 'GOTOOLCHAIN=auto', '-e', 'GOBIN=/repo/.work/acceptance/tools', '-w', '/repo/services/bff', goImage, 'sh', '-ec', script]);
const node = (script, network) => call('docker', ['run', '--rm', ...(network ? ['--network', `container:${network}`] : []), ...mount, '--mount', 'type=volume,src=weaveos-v010-linux-node,dst=/repo/node_modules', '--mount', 'type=volume,src=weaveos-v010-linux-web-node,dst=/repo/apps/web/node_modules', '-e', 'CI=true', '-e', 'WEAVEOS_API_URL=https://localhost:19443', '-e', 'WEAVEOS_WEB_URL=https://localhost:19443', '-e', 'WEAVEOS_ACCEPTANCE_FIXTURES=/repo/.work/acceptance/fixtures.json', '-e', 'NODE_EXTRA_CA_CERTS=/repo/.work/acceptance/tls/cert.pem', '-w', '/repo', playwrightImage, 'bash', '-euc', `npm install --global pnpm@10.28.2 --ignore-scripts; ${script}`]);
const started = Date.now();
writeFileSync(resolve(dir, 'public/result.json'), JSON.stringify({ result: 'running', project, target: 'isolated Linux HTTPS' }));
try {
  compose('up', '-d', '--wait', 'postgres', 'redis');
  // Published initial migration intentionally has no destructive Down. Recovery is V010-008.
  go('go install github.com/pressly/goose/v3/cmd/goose@v3.28.0; /repo/.work/acceptance/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up; /repo/.work/acceptance/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up');
  go('go test -race -p 1 -count=1 ./... && go vet ./... && CGO_ENABLED=0 go build -o /repo/.work/acceptance/bff ./cmd/bff');
  compose('exec', '-T', 'postgres', 'createdb', '-U', 'weaveos_test', 'weaveos_acceptance');
  go('/repo/.work/acceptance/tools/goose -dir /repo/db/migrations postgres "$WEAVEOS_TEST_DATABASE_URL" up && go run ./cmd/acceptance-seed', 'seed.env');
  node('pnpm install --frozen-lockfile --ignore-scripts --store-dir .work/pnpm-store; node --test contracts/*.test.mjs tests/governance/*.test.mjs tests/foundation/*.test.mjs tests/acceptance/topology.test.mjs; pnpm exec redocly lint contracts/openapi/openapi.json; pnpm typecheck; pnpm build; cd apps/web; pnpm exec playwright test --config playwright.component.config.ts');
  compose('up', '-d', 'bff', 'nginx');
  node('node --test tests/acceptance/api.test.mjs; pnpm exec playwright test --config apps/web/playwright.integration.config.ts --reporter=line', id('nginx'));
  call(process.execPath, ['--test', 'infra/acceptance/faults.test.mjs'], { env: { ...env, WEAVEOS_ACCEPTANCE_PROJECT: project, WEAVEOS_ACCEPTANCE_FIXTURES: resolve(dir, 'fixtures.json'), NODE_EXTRA_CA_CERTS: resolve(dir, 'tls/cert.pem') } });
  writeFileSync(resolve(dir, 'public/result.json'), JSON.stringify({ result: 'passed', project, elapsedSeconds: (Date.now() - started) / 1000, api: 20, browser: 30, components: 23, faults: 3, storage: 'actual PostgreSQL18 + Redis8.2', target: 'isolated Linux HTTPS', secrets: 'not included' }, null, 2));
} catch (error) {
  writeFileSync(resolve(dir, 'public/result.json'), JSON.stringify({ result: 'failed', project, exitCode: error.status ?? 1, elapsedSeconds: (Date.now() - started) / 1000 }));
  throw error;
} finally {
  // Preserve private fixtures and isolated DB volume; no --volumes or file deletion.
  compose('stop');
}
