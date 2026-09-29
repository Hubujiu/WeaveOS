import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {renderNginx} from '../../infra/runtime/nginx.mjs';

test('FR06 common rules render local and public explicit environments',()=>{
 const local=renderNginx();
 const remote=renderNginx({domain:'weave.hubujiu.site',redirect:true});
 assert.match(local,/server_name localhost;/);
 assert.match(remote,/return 308 https:\/\/weave\.hubujiu\.site\$request_uri;/);
 for(const source of [local,remote]){
  assert.match(source,/proxy_set_header X-Request-Id \$request_id;/);
  assert.match(source,/proxy_hide_header X-Request-Id;/);
  assert.match(source,/proxy_hide_header X-Content-Type-Options;/);
  assert.match(source,/add_header Cache-Control "no-store" always;/);
  assert.doesNotMatch(source,/\$request"|\$request_uri.*status|\$http_cookie|\$args|includeSubDomains|preload/);
 }
 assert.equal(readFileSync(new URL('../../infra/acceptance/nginx.conf',import.meta.url),'utf8').replaceAll('\r\n','\n'),local);
 assert.equal(readFileSync(new URL('../../infra/server/public-nginx.conf',import.meta.url),'utf8').replaceAll('\r\n','\n'),remote);
});
test('FR06 explicit parameters reject config injection and invalid HSTS scope',()=>{
 for(const domain of ['evil; include /secret;','a\nb','https://example.test','*.example.test','a/../b',''])assert.throws(()=>renderNginx({domain}));
 for(const hstsMaxAge of [-1,1.5,Infinity,'31536000'])assert.throws(()=>renderNginx({hstsMaxAge}));
 assert.match(renderNginx({hstsMaxAge:300}),/Strict-Transport-Security "max-age=300" always;/);
});
