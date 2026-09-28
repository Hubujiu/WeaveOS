// Deployment smoke only: creates/revokes one existing administrator session.
// Never prints credentials, response bodies or Cookie values. No registration or reset.
import { request } from 'node:https';
import { readFileSync } from 'node:fs';
const origin = 'https://weave.hubujiu.site';
const credentials = JSON.parse(readFileSync(process.argv[2], 'utf8'));
function check(condition, label) { if (!condition) throw Error(label); }
function call(path, method = 'GET', headers = {}, data) {
  return new Promise((resolve, reject) => {
    // This Windows host has a VPN fake-IP resolver; connect to the actual public IP.
    const req = request(new URL(path, origin), { method, headers, family: 4,
      lookup: (_host, _options, callback) => callback(null, '43.133.34.48', 4),
      rejectUnauthorized: true, timeout: 15000 }, response => {
      let body = ''; response.setEncoding('utf8');
      response.on('data', part => { body += part; });
      response.on('end', () => resolve({ status: response.statusCode, headers: response.headers, body }));
    });
    req.on('timeout', () => req.destroy(Error('timeout'))); req.on('error', reject);
    if (data) req.write(JSON.stringify(data)); req.end();
  });
}
const summary = { at: new Date().toISOString(), origin, connection: 'public IPv4, default CA and hostname verification; no SSH tunnel', checks: [] };
let cookie, csrf, revoked = false;
try {
  for (const path of ['/login', '/register', '/health/live', '/health/ready']) {
    const response = await call(path); check(response.status === 200, `${path} must return 200`);
    summary.checks.push({ path, status: response.status });
  }
  const page = await call('/login');
  const asset = page.body.match(/src="(\/assets\/[^\"]+\.js)"/)?.[1];
  check(asset, 'page must reference built JS'); check((await call(asset)).status === 200, 'JS asset must load');
  const anonymous = await call('/api/v1/sessions/current'); check(anonymous.status === 401, 'anonymous must be denied');
  const login = await call('/api/v1/sessions', 'POST', { Origin: origin, 'Content-Type': 'application/json' }, { account: credentials.account, password: credentials.password });
  check(login.status === 201, `login status ${login.status}`);
  const cookies = login.headers['set-cookie'] ?? [];
  cookie = cookies.map(value => value.split(';')[0]).join('; ');
  csrf = cookies.find(value => value.startsWith('__Host-csrf='))?.split(';')[0].slice('__Host-csrf='.length);
  const session = cookies.find(value => value.startsWith('__Host-session=')) ?? '';
  check(/; Secure/i.test(session) && /; HttpOnly/i.test(session) && /SameSite=Lax/i.test(session) && /Path=\//i.test(session) && !/Domain=/i.test(session), 'session attributes');
  check(login.headers['cache-control']?.includes('no-store'), 'auth must not cache');
  check((await call('/api/v1/sessions/current', 'GET', { Cookie: cookie })).status === 200, 'identity must restore');
  const crossOrigin = await call('/api/v1/sessions/current', 'DELETE', { Cookie: cookie, Origin: 'https://untrusted.example', 'X-CSRF-Token': csrf });
  check(crossOrigin.status === 403, 'cross-origin mutation must fail');
  const logout = await call('/api/v1/sessions/current', 'DELETE', { Cookie: cookie, Origin: origin, 'X-CSRF-Token': csrf });
  check(logout.status === 204, 'logout must succeed'); revoked = true;
  check((await call('/api/v1/sessions/current', 'GET', { Cookie: cookie })).status === 401, 'old cookie must fail after logout');
  Object.assign(summary, { assets: 'passed', anonymous: 401, login: 201, cookieAttributes: 'passed', authCache: 'no-store', identity: 200, crossOrigin: 403, logout: 204, revokedSession: 401, result: 'passed' });
  console.log(JSON.stringify(summary, null, 2));
} finally {
  if (cookie && csrf && !revoked) await call('/api/v1/sessions/current', 'DELETE', { Cookie: cookie, Origin: origin, 'X-CSRF-Token': csrf }).catch(() => {});
  credentials.password = ''; cookie = ''; csrf = '';
}
