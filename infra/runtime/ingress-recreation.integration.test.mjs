import test from 'node:test';
import assert from 'node:assert/strict';
import {runtimeContext} from './context.mjs';
const c=runtimeContext();
async function loginFailure(){
 let response;
 for(let i=0;i<40;i++){
  try{response=await fetch('https://localhost:19443/api/v1/sessions',{method:'POST',headers:{Origin:'https://localhost:19443','Content-Type':'application/json','X-Request-Id':'f'.repeat(32),'X-Forwarded-For':'203.0.113.99'},body:JSON.stringify({account:'absent-ingress-user',password:'Aa1!'}),signal:AbortSignal.timeout(2000)});if(response.status===401)return response;await response.text();}catch{}
  await new Promise(r=>setTimeout(r,200));
 }
 throw Error('Recreated ingress did not recover');
}
test('real Nginx recreation at a different peer IP retains trusted request metadata',async()=>{
 await loginFailure();
 const network=c.project+'_default';
 const address=()=>JSON.parse(c.command('docker',['inspect',c.container('nginx')]).toString())[0].NetworkSettings.Networks[network].IPAddress;
 const old=address(),reservation=c.project+'-old-ingress-address';let reserved=false;
 try{
  c.compose('stop','nginx');c.compose('rm','-f','nginx');
  c.command('docker',['run','-d','--name',reservation,'--network',network,'--ip',old,'debian:bookworm-slim','sleep','120']);reserved=true;
  c.compose('up','-d','--no-deps','nginx');assert.notEqual(address(),old);
  const response=await loginFailure(),body=await response.json(),id=response.headers.get('x-request-id');
  assert.match(id,/^[a-f0-9]{32}$/);assert.notEqual(id,'f'.repeat(32));assert.equal(body.meta.requestId,id);
  assert.equal(c.sql('weaveos_runtime',`SELECT request_id FROM auth.authentication_events WHERE request_id='${id}' AND client_ip <> '203.0.113.99';`),id);
 }finally{
  if(reserved){c.command('docker',['stop',reservation]);c.command('docker',['rm',reservation]);}
  c.compose('up','-d','--no-deps','nginx');await loginFailure();
 }
});
test('actual Nginx directory denial returns 403 with inherited global headers',()=>{
 const probe=c.project+'-403-probe';
 c.command('docker',['run','-d','--name',probe,'--network',c.project+'_default','--mount',`type=bind,src=${c.dir}/tls,dst=/etc/nginx/tls,readonly`,c.artifacts.web.imageID]);
 try{
  c.command('docker',['exec','-u','0',probe,'mkdir','/usr/share/nginx/html/denied']);
  c.command('docker',['exec','-u','0',probe,'chmod','700','/usr/share/nginx/html/denied']);
  const raw=c.command('docker',['exec',probe,'curl','--retry','10','--retry-connrefused','--retry-delay','1','--max-time','3','--cacert','/etc/nginx/tls/cert.pem','-sS','-D','-','-o','/dev/null','https://localhost:19443/denied/']).toString();
  assert.match(raw,/HTTP\/1\.1 403/);assert.match(raw,/X-Request-Id: [a-f0-9]{32}/i);
  assert.match(raw,/X-Content-Type-Options: nosniff/i);
  assert.equal((raw.match(/X-Request-Id:/gi)??[]).length,1);
 }finally{c.command('docker',['stop',probe]);c.command('docker',['rm',probe]);}
});
