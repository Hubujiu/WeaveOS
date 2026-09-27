import {X509Certificate,createPublicKey} from 'node:crypto';
import {readFileSync,renameSync,unlinkSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {privateFile} from '../runtime/backup.mjs';
import {acmePlan} from './acme.mjs';
import {serverContext} from './context.mjs';
import {receiveCodes} from '../runtime/monitor.mjs';
export function deployCertificate(c=serverContext()) {
 const certificate=resolve(c.dir,'acme-stage/cert.pem'),keyFile=resolve(c.dir,'acme-stage/key.pem');
 const chain=readFileSync(certificate),key=readFileSync(keyFile);
 try{
  const identity=validateCertificate(chain,key);
  try{c.command('openssl',['verify','-purpose','sslserver','-verify_hostname',identity.domain,'-untrusted',certificate,certificate]);}catch{throw Error('Certificate system trust rejected');}
  replaceCertificateFiles(resolve(c.dir,'tls'),chain,key,()=>{
   c.compose('exec','-T','nginx','nginx','-t');
   c.compose('exec','-T','nginx','nginx','-s','reload');
  });
  writeFileSync(resolve(c.dir,'tls/public-trust-ready'),identity.domain+'\n',{mode:0o600});
  return identity;
 }finally{key.fill(0);}
}

export function validateCertificate(chain,key,now=Date.now()) {
 const leaf=new X509Certificate(chain),domain=acmePlan().domain;
 if(leaf.checkHost(domain,{subject:'never',wildcards:false})!==domain||leaf.ca)throw Error('Certificate identity rejected');
 if(Date.parse(leaf.validFrom)>now||Date.parse(leaf.validTo)-now<3*86400000)throw Error('Certificate lifetime rejected');
 let matches=false;
 try{matches=leaf.publicKey.export({type:'spki',format:'der'}).equals(createPublicKey(key).export({type:'spki',format:'der'}));}catch{}
 if(!matches)throw Error('Certificate private key rejected');
 return {domain,fingerprint:leaf.fingerprint256,expires:leaf.validTo};
}

// The operational caller fixes this directory; tests use real isolated files.
export function replaceCertificateFiles(directory,chain,key,checkAndReload) {
 const cert=resolve(directory,'cert.pem'),privateKey=resolve(directory,'key.pem');
 const stamp=Date.now()+'-'+process.pid;
 const previousCert=cert+'.previous-'+stamp,previousKey=privateKey+'.previous-'+stamp;
 const nextCert=cert+'.next-'+stamp,nextKey=privateKey+'.next-'+stamp;
 // Read before mutating, and retain old bytes for explicit recovery.
 const oldCert=readFileSync(cert),oldKey=readFileSync(privateKey);
 privateFile(previousCert,oldCert);privateFile(previousKey,oldKey);
 privateFile(nextCert,chain);privateFile(nextKey,key);
 try{
  renameSync(nextCert,cert);renameSync(nextKey,privateKey);
  checkAndReload();
 }catch{
  // Recover both files even if only one of the forward renames succeeded.
  renameSync(previousCert,cert);renameSync(previousKey,privateKey);
  try{checkAndReload();}catch{throw Error('Certificate replacement failed; previous files restored, reload also failed');}
  throw Error('Certificate replacement failed; previous certificate restored');
 }finally{
  oldKey.fill(0);
  for(const file of [nextCert,nextKey])try{unlinkSync(file);}catch{}
 }
}

if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
 process.umask(0o077);
 try{
  if(process.argv[2]!=='deploy')throw Error('TLS operation rejected');
  console.log(JSON.stringify({status:'passed',...deployCertificate()}));
 }catch{
  receiveCodes('/opt/weaveos-v010/alerts.jsonl',['CERTIFICATE']);
  console.error('TLS activation failed; previous active certificate retained or restored');process.exitCode=1;
 }
}
