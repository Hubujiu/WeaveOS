import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { packageImages } from './artifacts.mjs';
const root=resolve('.'),commit=execFileSync('git',['rev-parse','HEAD'],{encoding:'utf8'}).trim();
test('immutable BFF/web OCI artifacts associate exact source and independently verified manifest bytes',()=>{
 const record=packageImages({root,commit,binaryDir:resolve('.work/acceptance'),commandDir:resolve('.work/cli'),webDir:resolve('apps/web/dist'),outputDir:resolve('.work/oci-test',String(Date.now()))});
 assert.equal(record.commit,commit,'build record must identify exact source');
 for(const name of ['bff','web']){
  const artifact=record[name];assert.match(artifact.manifestDigest,/^sha256:[0-9a-f]{64}$/);
  const index=JSON.parse(execFileSync('tar',['-xOf',artifact.archive,'index.json'],{encoding:'utf8'}));
  const descriptor=index.manifests[0];assert.equal(descriptor.digest,artifact.manifestDigest);
  const bytes=execFileSync('tar',['-xOf',artifact.archive,`blobs/sha256/${descriptor.digest.slice(7)}`]);
  assert.equal(`sha256:${createHash('sha256').update(bytes).digest('hex')}`,artifact.manifestDigest);
  const image=JSON.parse(execFileSync('docker',['image','inspect',artifact.imageID],{encoding:'utf8'}))[0];
  assert.equal(image.Id,artifact.imageID);assert.equal(image.Config.Labels['org.opencontainers.image.revision'],commit);
 }
 assert.equal(JSON.parse(readFileSync(record.recordFile,'utf8')).commit,commit);
});
