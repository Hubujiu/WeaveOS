import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync,existsSync} from 'node:fs';import {resolve} from 'node:path';import {summarizeScan,scanImages} from './image-scan.mjs';
test('image evidence preserves unfixed critical and fixable findings for explicit risk disposition',()=>{
 const report={Results:[{Target:'os',Vulnerabilities:[{VulnerabilityID:'CVE-FIXTURE-1',PkgName:'parser',InstalledVersion:'1',Severity:'CRITICAL',Status:'affected'},{VulnerabilityID:'CVE-FIXTURE-2',PkgName:'helper',InstalledVersion:'2',Severity:'HIGH',FixedVersion:'3',Status:'fixed'}]}]};
 assert.deepEqual(summarizeScan(report),[{target:'os',id:'CVE-FIXTURE-1',package:'parser',installed:'1',severity:'CRITICAL',status:'affected',fixed:null},{target:'os',id:'CVE-FIXTURE-2',package:'helper',installed:'2',severity:'HIGH',status:'fixed',fixed:'3'}]);
});
test('actual scanners produce source-associated evidence for both immutable OCI images',()=>{
 const record=JSON.parse(readFileSync(process.env.WEAVEOS_ARTIFACT_RECORD,'utf8')),dir=process.env.WEAVEOS_IMAGE_SCAN_DIR??resolve('.work',`image-evidence-${Date.now()}`);const result=scanImages(record,dir);
 assert.equal(result?.scanStatus,'completed','actual scan must complete and publish a record');assert.equal(result.source,record.commit);
 for(const name of ['bff','web']){assert.equal(result.images[name].digest,record[name].manifestDigest);assert.ok(existsSync(resolve(dir,`${name}.json`)));assert.ok(Array.isArray(result.images[name].findings));}
 const storage=JSON.parse(readFileSync('infra/runtime/compose.json','utf8')).services;
 for(const name of ['postgres','redis']){assert.equal(result.images[name]?.digest,storage[name].image.split('@')[1],'every runtime storage image needs a matching scan');assert.ok(existsSync(resolve(dir,`${name}.json`)));}
 assert.equal(result.riskAcceptance,'pending','executing scanners cannot approve risk');
});
