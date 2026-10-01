import test from 'node:test';
import assert from 'node:assert/strict';
import {request} from 'node:https';
import {readFileSync} from 'node:fs';
import {execFileSync,spawnSync} from 'node:child_process';
import {renderNginx} from '../../../../infra/runtime/nginx.mjs';
// Oracle: frozen query-post contract and existing Go raw/canonical boundary tests.
// Uses real HTTPS ingress; never changes nginx config or existing drafts.
const base=new URL(process.env.WEAVEOS_WEB_URL??'');
assert.ok(base.protocol==='https:'&&['localhost','127.0.0.1'].includes(base.hostname));
const credentials=JSON.parse(readFileSync(process.env.WEAVEOS_ACCEPTANCE_FIXTURES,'utf8')).admin;
const nginx=process.env.WEAVEOS_Q36_NGINX;
assert.match(nginx??'',/^weaveos-q36-b2-[0-9]+-nginx-1$/);
const cookies=new Map();
function call(path,body,method='POST'){
 return new Promise((resolve,reject)=>{
  const headers={Origin:base.origin,'Content-Type':'application/json',...(cookies.size?{Cookie:[...cookies].map(([k,v])=>k+'='+v).join('; ')}:{}),...(cookies.has('__Host-csrf')?{'X-CSRF-Token':cookies.get('__Host-csrf')}:{}),...(body!==undefined?{'Content-Length':Buffer.byteLength(body)}:{})};
  const req=request(new URL('/api/v1/'+path,base),{method,headers,rejectUnauthorized:false},res=>{
   for(const value of res.headers['set-cookie']??[]){const pair=value.split(';')[0],at=pair.indexOf('=');cookies.set(pair.slice(0,at),pair.slice(at+1));}
   let raw='';res.setEncoding('utf8');res.on('data',x=>raw+=x);res.on('end',()=>{
    try{assert.match(res.headers['x-request-id']??'',/^[a-f0-9]{32}$/);assert.equal(res.rawHeaders.filter(x=>x.toLowerCase()==='x-request-id').length,1);assert.equal(res.headers['cache-control'],'no-store');assert.equal(res.headers['x-content-type-options'],'nosniff');resolve({status:res.statusCode,body:raw?JSON.parse(raw):undefined});}catch(e){reject(e);}
   });
  });req.on('error',reject);req.setTimeout(12000,()=>req.destroy(Error('isolated ingress deadline')));req.end(body);
 });
}
function expectResponse(res,status,code='OK'){assert.equal(res.status,status);if(status!==204)assert.equal(res.body.code,code);}
function pad(body,size){assert.ok(Buffer.byteLength(body)<=size);return body+' '.repeat(size-Buffer.byteLength(body));}
function filter(size){
 const empty={children:[{field:'account',operator:'eq',value:''}],operator:'and'};
 const room=size-Buffer.byteLength(JSON.stringify(empty));empty.children[0].value='中'.repeat(Math.floor(room/3))+'x'.repeat(room%3);
 const result=JSON.stringify(empty);assert.equal(Buffer.byteLength(result),size);return result;
}
function payload(size){
 // Sorted map keys match the frozen canonical form; unique codes are each <=160 characters.
 const codes=Array.from({length:401},(_,i)=>String(i).padStart(3,'0')+'中'.repeat(52)+'x');
 const empty={description:'',name:'',permissionCodes:[...codes,'']};
 const room=size-Buffer.byteLength(JSON.stringify(empty));assert.ok(room>=0&&room<=160);empty.permissionCodes[401]='z'.repeat(room);
 const result=JSON.stringify(empty);assert.equal(Buffer.byteLength(result),size);return result;
}
const invalid='COMMON_INVALID_ARGUMENT';
test.before(async()=>{expectResponse(await call('sessions',JSON.stringify(credentials)),201);});
test('fixed actual Nginx config/version unchanged and small canonical Chinese query crosses the old URI-sized budget',async()=>{
 const version=spawnSync('docker',['exec',nginx,'nginx','-v'],{encoding:'utf8'});assert.equal(version.status,0);assert.match(version.stderr,/nginx\/1\.30\.5/);
 // Compose may reference a version tag; verify the running image ID against the exact pinned digest, without pulling.
 const running=execFileSync('docker',['inspect','--format','{{.Image}}',nginx],{encoding:'utf8'}).trim();
 const pinned=execFileSync('docker',['image','inspect','nginx:1.30.5@sha256:b972f831f200b19ef0767938224f9711e74cd783718738cd7405d5cabf75c442','--format','{{.Id}}'],{encoding:'utf8'}).trim();assert.equal(running,pinned);
 const conf=execFileSync('docker',['exec',nginx,'cat','/etc/nginx/nginx.conf'],{encoding:'utf8'});assert.ok(!conf.includes('client_max_body_size'));
 assert.equal(conf,renderNginx().replace('listen 19443 ssl;','listen '+base.port+' ssl;'));
 expectResponse(await call('personnel/members/search',JSON.stringify({filter:{operator:'and',children:[{field:'account',operator:'eq',value:'中文'.repeat(1800)}]}})),200);
});
test('both POST query routes independently accept raw 64KiB and reject 64KiB+1',async()=>{
 for(const view of ['members','events']){const exact=pad('{}',65536);expectResponse(await call('personnel/'+view+'/search',exact),200);expectResponse(await call('personnel/'+view+'/search',exact+' '),400,invalid);}
});
test('filter canonical 16KiB is inclusive independently of query raw spelling and whitespace',async()=>{
 const exact=filter(16384),over=filter(16385);
 expectResponse(await call('personnel/members/search','{"filter":'+exact+'}'),200);
 expectResponse(await call('personnel/members/search','{"filter":'+over+'}'),400,invalid);
 const escaped=exact.replace(/中/g,'\\u4e2d');assert.ok(Buffer.byteLength(escaped)<65536&&Buffer.byteLength(escaped)>16384);
 expectResponse(await call('personnel/members/search',pad('{"filter":'+escaped+'}',65536)),200);
});
test('draft create/update raw 128KiB is inclusive and rejected oversized writes retain the saved version',async()=>{
 const body='{"kind":"department","targetId":null,"baseVersion":null,"payload":{"name":"入口专项中文","parentId":null}}';
 const created=await call('personnel/drafts',pad(body,131072));expectResponse(created,201);const id=created.body.data.id;let version=1;
 try{
  const before=(await call('personnel/drafts',undefined,'GET')).body.data.items.length;
  expectResponse(await call('personnel/drafts',pad(body,131073)),400,invalid);
  assert.equal((await call('personnel/drafts',undefined,'GET')).body.data.items.length,before);
  const input={version,payload:{name:'128KiB 更新中文',parentId:null}};expectResponse(await call('personnel/drafts/'+id,pad(JSON.stringify(input),131072),'PUT'),200);version=2;
  expectResponse(await call('personnel/drafts/'+id,pad(JSON.stringify({...input,version,payload:{name:'禁止保存',parentId:null}}),131073),'PUT'),400,invalid);
  const retained=await call('personnel/drafts/'+id,undefined,'GET');expectResponse(retained,200);assert.equal(retained.body.data.version,version);assert.equal(retained.body.data.payload.name,'128KiB 更新中文');
 }finally{expectResponse(await call('personnel/drafts/'+id+'?version='+version,undefined,'DELETE'),204);}
});
test('draft canonical Chinese payload 64KiB accepts its wrapper and rejects +1 below the raw cap',async()=>{
 const exact=payload(65536),over=payload(65537),wrapper=p=>' {"kind":"template","targetId":null,"baseVersion":null,"payload":'+p+'}';
 assert.ok(Buffer.byteLength(wrapper(over))<131072);const created=await call('personnel/drafts',wrapper(exact));expectResponse(created,201);const id=created.body.data.id;let version=1;
 try{
  const update=p=>'{"version":'+version+',"payload":'+p+'}';expectResponse(await call('personnel/drafts/'+id,update(exact),'PUT'),200);version=2;
  expectResponse(await call('personnel/drafts/'+id,update(over),'PUT'),400,invalid);
  expectResponse(await call('personnel/drafts',wrapper(over)),400,invalid);
  const retained=await call('personnel/drafts/'+id,undefined,'GET');expectResponse(retained,200);assert.equal(retained.body.data.version,version);assert.deepEqual(retained.body.data.payload,JSON.parse(exact));
 }finally{expectResponse(await call('personnel/drafts/'+id+'?version='+version,undefined,'DELETE'),204);}
});
