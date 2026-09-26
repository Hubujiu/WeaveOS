import test from 'node:test';import assert from 'node:assert/strict';import {resolve} from 'node:path';import {runtimeContext} from './context.mjs';
const c=runtimeContext();
test('Nginx follows an actual BFF address change through Docker DNS without gateway restart',async()=>{
 const network=c.project+'_default',bff=c.container('bff'),probe=c.project+'-dns-probe';
 const info=JSON.parse(c.command('docker',['network','inspect',network]).toString())[0];assert.equal(info.Internal,true);
 const original=Object.values(info.Containers).find(x=>x.Name===bff).IPv4Address.split('/')[0];
 const [base,prefixText]=info.IPAM.Config[0].Subnet.split('/'),prefix=Number(prefixText);assert.ok(prefix>=16&&prefix<=29,'fixture needs an owned IPv4 network with spare addresses');
 const address=base.split('.').reduce((n,v)=>n*256+Number(v),0)+2**(32-prefix)-3;
 const next=[24,16,8,0].map(bits=>(address>>>bits)&255).join('.');assert.equal(Object.values(info.Containers).some(x=>x.IPv4Address.split('/')[0]===next),false);
 const nginx='nginx:1.30.5@sha256:b972f831f200b19ef0767938224f9711e74cd783718738cd7405d5cabf75c442';let changed=false;
 c.command('docker',['run','-d','--name',probe,'--network',network,'--mount',`type=bind,src=${resolve('infra/acceptance/nginx.conf')},dst=/etc/nginx/nginx.conf,readonly`,'--mount',`type=bind,src=${resolve(c.dir,'tls')},dst=/etc/nginx/tls,readonly`,nginx]);
 const healthy=()=>{try{return c.command('docker',['exec',probe,'curl','--max-time','2','--cacert','/etc/nginx/tls/cert.pem','-s','-o','/dev/null','-w','%{http_code}','https://localhost:19443/health/ready']).toString()==='200';}catch{return false;}};
 try{
  for(let i=0;i<20&&!healthy();i++)await new Promise(r=>setTimeout(r,100));assert.equal(healthy(),true,'original backend must be healthy');
  c.command('docker',['network','disconnect',network,bff]);changed=true;c.command('docker',['network','connect','--alias','bff','--ip',next,network,bff]);
  for(let i=0;i<40&&!healthy();i++)await new Promise(r=>setTimeout(r,100));assert.equal(healthy(),true,'existing gateway must resolve the new service address');
 }finally{
  if(changed){try{c.command('docker',['network','disconnect',network,bff]);}catch{}c.command('docker',['network','connect','--alias','bff','--ip',original,network,bff]);}
  c.command('docker',['stop',probe]);c.command('docker',['rm',probe]);
 }
});
