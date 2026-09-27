import assert from 'node:assert/strict';
import { createConnection } from 'node:net';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';

// Test-only observer; reads the runner's private configuration and uses native Redis
// expiration. Never connects to the deployed service or emits keys/credentials.
export async function open(options = {}) {
  let { redisURL, generation } = options;
  if (!redisURL) {
    assert.ok(process.env.WEAVEOS_ACCEPTANCE_FIXTURES, 'isolated fixture path required');
    const values = Object.fromEntries(readFileSync(join(dirname(process.env.WEAVEOS_ACCEPTANCE_FIXTURES), 'runtime.env'), 'utf8')
      .trim().split(/\r?\n/).map(line => { const i = line.indexOf('='); return [line.slice(0, i), line.slice(i + 1)]; }));
    redisURL = values.WEAVEOS_REDIS_URL;
    generation = values.WEAVEOS_SESSION_GENERATION;
  }
  const url = new URL(redisURL);
  assert.ok(url.protocol === 'redis:' && ['redis', 'localhost', '127.0.0.1'].includes(url.hostname), 'isolated Redis required');
  assert.ok(/^(weaveos-v010-00[789]-[A-Za-z0-9_-]+|recovered_[0-9a-f]{32})$/.test(generation), 'runner-owned generation required');
  return {
    storage: 'isolated-redis',
    async expireSession(cookie) {
      assert.ok(/^__Host-session=[A-Za-z0-9_-]{43}$/.test(cookie), 'canonical test session required');
      const sid = cookie.slice('__Host-session='.length), raw = Buffer.from(sid, 'base64url');
      assert.ok(raw.toString('base64url') === sid, 'canonical session encoding required');
      const key = `ems:auth:session:${generation}:v1:${createHash('sha256').update(raw).digest('hex')}`;
      const commands = [
        ...(url.password ? [['AUTH', decodeURIComponent(url.password)]] : []),
        ['SELECT', url.pathname.slice(1) || '0'], ['EXISTS', key], ['PEXPIRE', key, '0'], ['PTTL', key],
      ];
      const replies = await new Promise((resolve, reject) => {
        const socket = createConnection({ host: url.hostname, port: Number(url.port || 6379) });
        let pending = '', values = [];
        const fail = () => { socket.destroy(); reject(new Error('isolated Redis observation failed; details withheld')); };
        socket.setTimeout(5000, fail).on('error', fail);
        socket.on('connect', () => socket.write(commands.map(args => `*${args.length}\r\n` + args.map(arg => `$${Buffer.byteLength(arg)}\r\n${arg}\r\n`).join('')).join('')));
        socket.on('data', chunk => {
          pending += chunk.toString();
          let end;
          while ((end = pending.indexOf('\r\n')) >= 0) {
            const line = pending.slice(0, end); pending = pending.slice(end + 2);
            if (!['+', ':'].includes(line[0])) { fail(); return; }
            values.push(line[0] === ':' ? Number(line.slice(1)) : line.slice(1));
            if (values.length === commands.length) { socket.destroy(); resolve(values); }
          }
        });
      });
      assert.deepEqual(replies.slice(-3), [1, 1, -2], 'existing session must expire natively');
    },
    async close() {},
  };
}
