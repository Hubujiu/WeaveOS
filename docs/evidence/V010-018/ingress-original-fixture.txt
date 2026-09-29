import test from 'node:test';
import assert from 'node:assert/strict';
import {request} from 'node:https';
import {execFileSync} from 'node:child_process';

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
test('actual ingress config parses',()=>assert.match(docker('exec',names.nginx,'nginx','-t'),/^/));
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
test('paused and stopped upstream produce identifiable non-cacheable 504/502',async()=>{
 docker('pause',names.bff);
 try {const res=await call('/api/v1/sessions/current');assert.equal(res.status,504);policy(res,{api:true});}
 finally{docker('unpause',names.bff);}
 docker('stop','--time','2',names.bff);
 try {const res=await call('/api/v1/sessions/current');assert.equal(res.status,502);policy(res,{api:true});}
 finally{docker('start',names.bff);}
});
