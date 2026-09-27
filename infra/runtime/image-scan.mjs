import {execFileSync} from 'node:child_process';import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';import {createHash} from 'node:crypto';import {resolve} from 'node:path';import {verifyArtifacts} from './artifacts.mjs';
export function summarizeScan(report){
 if(!Array.isArray(report.Results)||report.Results.length===0)throw new Error('Image scan has no inventory');
 return report.Results.flatMap(r=>(r.Vulnerabilities??[]).map(v=>({target:r.Target,id:v.VulnerabilityID,package:v.PkgName,installed:v.InstalledVersion,severity:v.Severity,status:v.Status,fixed:v.FixedVersion??null})));
}
export function scanImages(record,directory){
 verifyArtifacts(record);mkdirSync(directory,{recursive:true});
 const temp=resolve('.work',`scan-unpacked-${Date.now()}`),images={};
 const tool='aquasec/trivy:0.74.0@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969';
 const config=readFileSync('infra/runtime/compose.json'),storage=JSON.parse(config).services;
 for(const name of ['bff','web','postgres','redis']){
  const unpacked=resolve(temp,name);mkdirSync(unpacked,{recursive:true});let archive=record[name]?.archive,digest=record[name]?.manifestDigest;
  if(!archive){
   const reference=storage[name].image;if(!/^[a-z]+:[0-9.]+@sha256:[0-9a-f]{64}$/.test(reference))throw new Error('Storage image must be pinned before scanning');
   digest=reference.split('@')[1];archive=resolve(temp,`${name}.tar`);
   execFileSync('docker',['pull',reference],{stdio:'pipe',timeout:180000});execFileSync('docker',['save','--output',archive,reference],{stdio:'pipe',timeout:180000});
  }
  execFileSync('tar',['-xf',archive,'-C',unpacked],{stdio:'pipe'});
  // This collects complete evidence, including unfixed findings. Only the
  // separate user MAN-RISK release gate can accept the reported local risk.
  const raw=execFileSync('docker',['run','--rm','--mount',`type=bind,src=${unpacked},dst=/oci,readonly`,'--mount','type=volume,src=weaveos-v010-trivy-cache,dst=/root/.cache/trivy',tool,'image','--input','/oci','--scanners','vuln','--format','json','--quiet','--no-progress'],{encoding:'utf8',stdio:'pipe',timeout:360000,maxBuffer:32*1024*1024});
  const report=JSON.parse(raw),findings=summarizeScan(report);writeFileSync(resolve(directory,`${name}.json`),raw,{flag:'wx'});
  images[name]={digest,findings};
 }
 const result={scanStatus:'completed',riskAcceptance:'pending',source:record.commit,runtimeConfigSHA256:createHash('sha256').update(config).digest('hex'),tool,at:new Date().toISOString(),images};writeFileSync(resolve(directory,'summary.json'),JSON.stringify(result,null,2),{flag:'wx'});return result;
}
