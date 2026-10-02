import assert from 'node:assert/strict';
import {readFileSync,readdirSync,createReadStream,mkdirSync,writeFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {hash,canonical,approvalMap,validateEnvelope,receiveBundle} from '../../../../../infra/server/deploy/bundle.mjs';
import {validateCompatibility} from '../../../../../infra/server/deploy/policy.mjs';
import {validateInstalledPersonnelRoles} from '../../../../../infra/server/deploy/personnel-upgrade.mjs';

const before=JSON.parse(readFileSync(new URL('./compatibility-before.json',import.meta.url),'utf8'));
const config=JSON.parse(readFileSync('infra/server/deploy/compatibility.json','utf8'));
const fixed=JSON.parse(readFileSync(new URL('./independent-migration-digests.json',import.meta.url),'utf8'));
const approved=approvalMap(config.migrations),old=approvalMap(before.migrations);
assert.equal(config.backwardCompatible,before.backwardCompatible,'compatibility semantics remain unchanged');
assert.deepEqual(config.config,before.config,'configuration pins remain unchanged');
assert.deepEqual(Object.keys(approved).sort(),[...Object.keys(old),...Object.keys(fixed)].sort(),'only two reviewed additions allowed');
for(const[path,digest]of Object.entries({...old,...fixed}))assert.equal(approved[path],digest,'exact fixed approval '+path);
const migrations={};
for(const dir of ['migrations','archive-migrations'])for(const file of readdirSync('db/'+dir).filter(file=>file.endsWith('.sql'))){
 const path=dir+'/'+file;migrations[path]=hash(canonical(readFileSync('db/'+path,'utf8')));
}
for(const[name,path]of [['compose.json','infra/runtime/compose.json'],['nginx.conf','infra/acceptance/nginx.conf']])assert.equal(hash(canonical(readFileSync(path,'utf8'))),config.config[name]);
validateInstalledPersonnelRoles(readFileSync('infra/runtime/roles.sql','utf8'));
const current={runId:1,migrations:old};
const release={commit:'7afc8a51660a97ec73c40ee5d3202cf0c7f0f225',runId:2,migrations,approved,backwardCompatible:config.backwardCompatible};
assert.equal(validateCompatibility(current,release),true,'approved expansion preserves all published migrations');
assert.throws(()=>validateCompatibility(current,{...release,migrations:{...migrations,'migrations/00005_unknown.sql':'a'.repeat(64)}}),/Migration not approved/);
assert.throws(()=>validateCompatibility(current,{...release,migrations:{...migrations,'migrations/00003_query_drafts.sql':'b'.repeat(64)}}),/Migration not approved/);
assert.throws(()=>validateCompatibility(current,{...release,migrations:{...migrations,'migrations/00001_auth.sql':'c'.repeat(64)}}),/Published migration changed or removed/);
assert.throws(()=>validateCompatibility(current,{...release,backwardCompatible:false}),/identity\/compatibility rejected/);
console.log('Reviewed six migration/config/receiver pins match; approved expansion accepted; unknown, tampered Q36, changed published migration and false compatibility rejected.');

const[manifestFile,buildFile,receiveDir]=process.argv.slice(2);
if(manifestFile||buildFile||receiveDir){
 if(!manifestFile||!buildFile||!receiveDir)throw Error('Actual package manifest, accepted BUILD.json and fresh receive directory required');
 const manifest=JSON.parse(readFileSync(manifestFile,'utf8')),build=JSON.parse(readFileSync(buildFile,'utf8'));
 assert.equal(validateEnvelope(manifest),true);
 assert.equal(manifest.commit,build.commit,'actual package uses accepted OCI source identity');
 assert.deepEqual(manifest.migrations,migrations);
 assert.deepEqual(manifest.approved,approved);
 for(const name of ['bff','web']){
  assert.equal(manifest.images[name].ociDigest,build[name].manifestDigest);
  assert.equal(manifest.images[name].ociSHA256,build[name].archiveSHA256);
  assert.equal(hash(readFileSync(resolve(buildFile,'..',build[name].archive))),build[name].archiveSHA256,'accepted OCI archive unchanged');
 }
 const unknown=structuredClone(manifest);unknown.migrations['migrations/00005_unknown.sql']='d'.repeat(64);
 assert.throws(()=>validateEnvelope(unknown),/Migration not approved/);
 const changed=structuredClone(manifest);changed.files['migrations/00004_query_revision_writers.sql']+='\nSELECT 1;';
 assert.throws(()=>validateEnvelope(changed),/content hash mismatch/);
 mkdirSync(receiveDir);
 const received=await receiveBundle(createReadStream(resolve(manifestFile,'..','release.bin')),receiveDir);
 assert.deepEqual(received,manifest,'real binary envelope and both image byte streams verified');
 const report={source:build.commit,classification:'historical accepted OCI input; not current Q36 artifact acceptance',migrations,images:manifest.images,unknownMigrationRejected:true,tamperedMigrationRejected:true,acceptedOCIUnchanged:true,actualBinaryRoundTrip:true};
 writeFileSync(resolve(receiveDir,'verification.json'),JSON.stringify(report,null,2)+'\n');
 console.log('Actual package binary round trip and original accepted OCI digests passed without rebuilding images; accepted source='+build.commit+'.');
}
