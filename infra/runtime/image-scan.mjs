import {execFileSync} from 'node:child_process';import {mkdirSync,writeFileSync} from 'node:fs';import {resolve} from 'node:path';import {verifyArtifacts} from './artifacts.mjs';
export function summarizeScan(report){
 if(!Array.isArray(report.Results)||report.Results.length===0)throw new Error('Image scan has no inventory');
 return report.Results.flatMap(r=>(r.Vulnerabilities??[]).map(v=>({target:r.Target,id:v.VulnerabilityID,package:v.PkgName,installed:v.InstalledVersion,severity:v.Severity,status:v.Status,fixed:v.FixedVersion??null})));
}
export function scanImages(record,directory){
 verifyArtifacts(record);mkdirSync(directory,{recursive:true});
 const temp=resolve('.work',`scan-unpacked-${Date.now()}`),images={};
 const tool='aquasec/trivy:0.74.0@sha256:62b1e65e8869bc4b4c6aa4fa2b21595256c7c2f6018a9d9ad61caf87187c1969';
 for(const name of ['bff','web']){
  const unpacked=resolve(temp,name);mkdirSync(unpacked,{recursive:true});execFileSync('tar',['-xf',record[name].archive,'-C',unpacked],{stdio:'pipe'});
  // This collects complete evidence, including unfixed findings. Only the
  // separate user MAN-RISK release gate can accept the reported local risk.
  const raw=execFileSync('docker',['run','--rm','--mount',`type=bind,src=${unpacked},dst=/oci,readonly`,'--mount','type=volume,src=weaveos-v010-trivy-cache,dst=/root/.cache/trivy',tool,'image','--input','/oci','--scanners','vuln','--format','json','--quiet','--no-progress'],{encoding:'utf8',stdio:'pipe',timeout:360000,maxBuffer:32*1024*1024});
  const report=JSON.parse(raw),findings=summarizeScan(report);writeFileSync(resolve(directory,`${name}.json`),raw,{flag:'wx'});
  images[name]={digest:record[name].manifestDigest,findings};
 }
 const result={scanStatus:'completed',riskAcceptance:'pending',source:record.commit,tool,at:new Date().toISOString(),images};writeFileSync(resolve(directory,'summary.json'),JSON.stringify(result,null,2),{flag:'wx'});return result;
}
