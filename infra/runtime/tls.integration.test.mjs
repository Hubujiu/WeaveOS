import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync,writeFileSync,statSync} from 'node:fs';import {resolve} from 'node:path';import {X509Certificate} from 'node:crypto';import {execFileSync} from 'node:child_process';import https from 'node:https';
import {runtimeContext} from './context.mjs';import {privateFile} from './backup.mjs';
const c=runtimeContext(),key=resolve(c.dir,'tls/key.pem'),cert=resolve(c.dir,'tls/cert.pem');
async function trustedReady(ca){
 let status;for(let i=0;i<30;i++){try{status=await new Promise((resolve,reject)=>{const r=https.get('https://localhost:19443/health/ready',{ca,agent:false,timeout:2000},response=>{response.resume();resolve(response.statusCode);});r.on('error',reject);r.on('timeout',()=>r.destroy(new Error('TLS timeout')));});if(status===200)return status;}catch{}await new Promise(r=>setTimeout(r,100));}return status;
}
test('actual TLS private key is restricted to its local owner',()=>{
 if(process.platform==='win32')assert.equal(execFileSync('icacls',[key],{encoding:'utf8'}).includes('(I)'),false,'TLS key must not inherit directory readers');
 else assert.equal(statSync(key).mode&0o077,0,'TLS key must not grant group/world access');
});
test('renewed local TLS certificate is loaded by Nginx and serves HTTPS with explicit trust',async()=>{
 const stamp=Date.now(),oldKey=readFileSync(key),oldCert=readFileSync(cert),nextKey=resolve(c.dir,`tls/renew-key-${stamp}.pem`),nextCert=resolve(c.dir,`tls/renew-cert-${stamp}.pem`);
 privateFile(nextKey,Buffer.alloc(0));
 const openssl=process.platform==='win32'?'C:/Program Files/Git/usr/bin/openssl.exe':'openssl';
 execFileSync(openssl,['req','-x509','-newkey','rsa:2048','-nodes','-days','7','-keyout',nextKey,'-out',nextCert,'-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1'],{stdio:'pipe'});
 try{
  const replacement=readFileSync(nextCert);assert.notEqual(new X509Certificate(replacement).fingerprint256,new X509Certificate(oldCert).fingerprint256);
  writeFileSync(key,readFileSync(nextKey));writeFileSync(cert,replacement);c.compose('exec','-T','nginx','nginx','-t');c.compose('exec','-T','nginx','nginx','-s','reload');
  assert.equal(await trustedReady(replacement),200,'new certificate must actually complete a trusted HTTPS request');
 }finally{writeFileSync(key,oldKey);writeFileSync(cert,oldCert);oldKey.fill(0);c.compose('restart','nginx');assert.equal(await trustedReady(oldCert),200,'test cleanup must restore the original trusted TLS endpoint');}
});
