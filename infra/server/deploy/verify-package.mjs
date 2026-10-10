// PR acceptance verifies packaging or the original deployment policy's refusal.
// This never changes package.mjs, the receiver, or main delivery authorization.
import {readFileSync,existsSync} from 'node:fs';
import {spawnSync} from 'node:child_process';
import {resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
export function verifyPackageResult(result,compatible){
 if(typeof compatible!=='boolean'||result.error||result.signal)throw Error('Packaging verification did not complete');
 if(compatible){if(result.status!==0)throw Error('Compatible artifact packaging failed');return 'packaged';}
 if(result.status!==1||!/^Error: Release identity\/compatibility rejected\r?$/m.test(result.stderr??''))throw Error('Non-compatible artifact was not rejected by original deployment policy');
 return 'promotion-blocked';
}
if(process.argv[1]&&import.meta.url===pathToFileURL(resolve(process.argv[1])).href){
 const [input,output]=process.argv.slice(2);if(!input||!output)throw Error('Usage: verify-package.mjs BUILD.json output-directory');
 const {backwardCompatible}=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));
 const result=spawnSync(process.execPath,['infra/server/deploy/package.mjs',input,output],{encoding:'utf8',timeout:300000,maxBuffer:4*1024*1024});
 const state=verifyPackageResult(result,backwardCompatible);
 if(state==='promotion-blocked'&&(existsSync(resolve(output,'release.bin'))||existsSync(resolve(output,'manifest.json'))))throw Error('Rejected release left a promotable bundle');
 if(state==='packaged'&&(!existsSync(resolve(output,'release.bin'))||!existsSync(resolve(output,'manifest.json'))))throw Error('Successful packaging omitted the release bundle');
 console.log(JSON.stringify({status:state,backwardCompatible}));
}
