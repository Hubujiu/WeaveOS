// Transparent RESP transport for a single isolated test command. Never supplies
// authentication results: every reply comes from the real Redis server.
import net from 'node:net';
import http from 'node:http';

export async function startGate({ upstreamHost, upstreamPort = 6379, port = 6379, controlPort = 6380 }) {
  let target, observed = false, held;
  const sockets = new Set();
  const server = net.createServer(client => {
    const upstream = net.connect(upstreamPort, upstreamHost);
    sockets.add(client); sockets.add(upstream);
    let pending = Buffer.alloc(0);
    const close = () => { client.destroy(); upstream.destroy(); sockets.delete(client); sockets.delete(upstream); };
    client.on('error', close).on('close', close); upstream.on('error', close).on('close', close);
    upstream.pipe(client);
    client.on('data', chunk => {
      pending = Buffer.concat([pending, chunk]);
      while (pending.length) {
        const parsed = command(pending);
        if (!parsed) return;
        const frame = pending.subarray(0, parsed.end); pending = pending.subarray(parsed.end);
        if (target && ['EVAL', 'EVALSHA'].includes(parsed.args[0].toUpperCase()) && parsed.args[3] === target) {
          observed = true;
          target = undefined;
          held = () => upstream.write(frame);
          continue;
        }
        upstream.write(frame);
      }
    });
  });
  const control = http.createServer(async (req, res) => {
    if (req.method === 'POST' && req.url === '/arm') {
      let body = ''; for await (const chunk of req) { body += chunk; if (body.length > 1024) { res.writeHead(413).end(); return; } }
      const key = JSON.parse(body).key;
      if (typeof key !== 'string' || !/^ems:auth:session:(weaveos-v010-007-\d+|recovered_[0-9a-f]{32}):v1:[0-9a-f]{64}$/.test(key) || held) { res.writeHead(400).end(); return; }
      target = key; observed = false;
    } else if (req.method === 'POST' && req.url === '/release') { held?.(); held = undefined; target = undefined; }
    else if (req.method !== 'GET' || req.url !== '/status') { res.writeHead(404).end(); return; }
    res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify({ observed }));
  });
  await Promise.all([new Promise(resolve => server.listen(port, '0.0.0.0', resolve)), new Promise(resolve => control.listen(controlPort, '0.0.0.0', resolve))]);
  return { port: server.address().port, controlPort: control.address().port, async close() { held?.(); for (const socket of sockets) socket.destroy(); await Promise.all([new Promise(resolve => server.close(resolve)), new Promise(resolve => control.close(resolve))]); } };
}

function command(bytes) {
  let end = bytes.indexOf('\r\n'); if (end < 0) return;
  if (bytes[0] !== 42) throw new Error('RESP array required');
  const count = Number(bytes.subarray(1, end)); let offset = end + 2; const args = [];
  for (let i = 0; i < count; i++) {
    end = bytes.indexOf('\r\n', offset); if (end < 0) return;
    if (bytes[offset] !== 36) throw new Error('RESP bulk argument required');
    const size = Number(bytes.subarray(offset + 1, end));
    if (bytes.length < end + 2 + size + 2) return;
    args.push(bytes.subarray(end + 2, end + 2 + size).toString()); offset = end + 2 + size + 2;
  }
  return { args, end: offset };
}
if (process.argv.includes('--serve')) await startGate({ upstreamHost: 'redis' });
