import {X509Certificate,createPublicKey} from 'node:crypto';
import {readFileSync,renameSync,unlinkSync} from 'node:fs';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {privateFile} from '../runtime/backup.mjs';
import {acmePlan} from './acme.mjs';

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

// Operational activation is still blocked on Q16 DNS credentials. Never let
// a premature ACME reload hook silently report success without deploying TLS.
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href)throw Error('TLS activation pending DNS credentials and verified operational wiring');
