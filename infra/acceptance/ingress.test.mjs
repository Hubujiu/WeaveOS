import test from 'node:test';
import assert from 'node:assert/strict';
import {request} from 'node:https';
import {execFileSync} from 'node:child_process';
import {resolve} from 'node:path';

// Oracle: PRD FR02/03, AC04–06. This suite requires real Nginx + Go + PG.
const base=new URL(process.env.WEAVEOS_API_URL??'https://localhost:19443');
const names=JSON.parse(process.env.WEAVEOS_INGRESS_CONTAINERS??'null');
if(!names?.nginx||!names?.bff||!names?.postgres)throw Error('Explicit isolated ingress container identities required');
const docker=(...args)=>execFileSync('docker',args,{encoding:'utf8',stdio:['pipe','pipe','pipe'],timeout:30000});
function call(path,{method='GET',body='',headers={}}={}){
 return new Promise((resolve,reject)=>{
  const req=request(new URL(path,base),{method,headers},res=>{let text='';res.setEncoding('utf8');res.on('data',x=>text+=x);res.on('end',()=>resolve({status:res.statusCode,headers:res.headers,raw:res.rawHeaders,text}));});
  req.on('error',reject);req.setTimeout(12000,()=>req.destroy(Error('HTTP deadline exceeded')));req.end(body);
 });
}
function policy(res,{api=false}={}){
 assert.match(res.headers['x-request-id']??'',/^[a-f0-9]{32}$/);
 assert.equal(res.raw.filter(x=>x.toLowerCase()==='x-request-id').length,1);
 assert.equal(res.headers['x-content-type-options'],'nosniff');
 assert.equal(res.raw.filter(x=>x.toLowerCase()==='x-content-type-options').length,1);
 if(api)assert.equal(res.headers['cache-control'],'no-store');
}
test('actual ingress config parses and becomes ready',async()=>{
 docker('exec',names.nginx,'nginx','-t');
 const deadline=Date.now()+20000;let ready=false;
 while(Date.now()<deadline){try{ready=(await call('/health/ready')).status===200;}catch{}if(ready)break;await new Promise(r=>setTimeout(r,200));}
 assert.ok(ready,'real ingress did not become ready');
});
test('API ingress/header/JSON/audit/access log correlate and external IDs cannot be forged',async()=>{
 const forged='f'.repeat(32);
 const res=await call('/api/v1/sessions?invitationCode=DO_NOT_LOG_INVITATION',{method:'POST',body:JSON.stringify({account:'unknown-ingress-probe',password:'Aa1!'}),headers:{'Content-Type':'application/json','Origin':base.origin,'X-Request-Id':forged,'X-Forwarded-For':'203.0.113.99','Cookie':'unrelated=DO_NOT_LOG_COOKIE','X-CSRF-Token':'DO_NOT_LOG_CSRF'}});
 assert.equal(res.status,401);policy(res,{api:true});
 const id=res.headers['x-request-id'];assert.notEqual(id,forged);assert.equal(JSON.parse(res.text).meta.requestId,id);
 const row=docker('exec',names.postgres,'psql','-U','weaveos_test','-d',process.env.WEAVEOS_INGRESS_DATABASE??'weaveos_ci_test','-tAc',`SELECT request_id FROM auth.authentication_events WHERE request_id='${id}' AND client_ip <> '203.0.113.99'`);
 assert.equal(row.trim(),id);
 const logs=docker('logs',names.nginx);
 assert.ok(logs.includes(id),'ingress access log missing correlation');
 for(const secret of ['DO_NOT_LOG_INVITATION','DO_NOT_LOG_COOKIE','DO_NOT_LOG_CSRF','Aa1!'])assert.ok(!logs.includes(secret),'ingress log exposed request secrets');
});
test('static HTML and assets have ingress metadata and separate cache policy',async()=>{
 const html=await call('/login');assert.equal(html.status,200);policy(html);assert.notEqual(html.headers['cache-control'],'no-store');
 const asset=await call('/assets/missing.js');assert.equal(asset.status,404);policy(asset);assert.notEqual(asset.headers['cache-control'],'no-store');
});
test('proxy-generated 413 preserves headers and API no-store without a BFF response',async()=>{
 const res=await call('/api/v1/sessions',{method:'POST',body:'x'.repeat(2*1024*1024),headers:{'Content-Type':'application/json'}});
 assert.equal(res.status,413);policy(res,{api:true});
});
test('paused upstream produces identifiable non-cacheable 504',async()=>{
 docker('pause',names.bff);
 try {const res=await call('/api/v1/sessions/current');assert.equal(res.status,504);policy(res,{api:true});}
 finally{docker('unpause',names.bff);}
});

test('real upstream TCP reset produces identifiable non-cacheable 502',async()=>{
 // A stopped Docker endpoint can time out (504), depending on bridge routing.
 // This fixture keeps a live address and explicitly resets an accepted socket.
 const group=`weaveos-v010-018-gateway-${Date.now()}`;
 const tls=JSON.parse(docker('inspect','--format','{{json .Mounts}}',names.nginx)).find(m=>m.Destination==='/etc/nginx/tls')?.Source;
 assert.ok(tls,'owned test TLS mount required');
 const containers=[];docker('network','create',group);
 try{
  docker('run','-d','--name',group+'-upstream','--network',group,'--network-alias','bff','mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27','node','-e',"require('net').createServer(socket=>{console.log('reset');socket.destroy()}).listen(8080,'0.0.0.0',()=>console.log('ready'))");containers.push(group+'-upstream');
  for(let i=0;i<30&&!docker('logs',group+'-upstream').includes('ready');i++)await new Promise(r=>setTimeout(r,100));
  assert.ok(docker('logs',group+'-upstream').includes('ready'));
  docker('run','-d','--name',group+'-nginx','--network',group,'--mount',`type=bind,src=${tls},dst=/etc/nginx/tls,readonly`,'--mount',`type=bind,src=${resolve('infra/acceptance/nginx.conf')},dst=/etc/nginx/nginx.conf,readonly`,'nginx:1.30.5@sha256:b972f831f200b19ef0767938224f9711e74cd783718738cd7405d5cabf75c442');containers.push(group+'-nginx');
  let raw='';
  for(let i=0;i<30;i++){
   try{raw=docker('exec',group+'-nginx','curl','--max-time','3','--cacert','/etc/nginx/tls/cert.pem','-sS','-D','-','-o','/dev/null','https://localhost:19443/api/v1/sessions/current');break;}catch{}
   await new Promise(r=>setTimeout(r,100));
  }
  assert.match(raw,/HTTP\/1\.1 502/);
  assert.ok(docker('logs',group+'-upstream').includes('reset'),'fixture must actually reset the accepted connection');
  const pairs=raw.split('\r\n').filter(line=>line.includes(':')).map(line=>{const colon=line.indexOf(':');return [line.slice(0,colon),line.slice(colon+1).trim()];});
  policy({headers:Object.fromEntries(pairs.map(([k,v])=>[k.toLowerCase(),v])),raw:pairs.flat()},{api:true});
 }finally{
  for(const name of containers.reverse()){docker('stop',name);docker('rm',name);}
  docker('network','rm',group);
 }
});
