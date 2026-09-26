import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { verifyArtifacts } from './artifacts.mjs';
const file=process.env.WEAVEOS_ARTIFACT_RECORD;
if(!file)throw new Error('Actually built OCI record required');
const record=JSON.parse(readFileSync(file,'utf8'));
test('verified source/OCI bytes/Docker image combination can be promoted',()=>{assert.equal(verifyArtifacts(record),true);});
test('changed OCI digest, archive bytes claim or source identity refuses promotion',()=>{
 for(const field of ['manifestDigest','archiveSHA256']){const broken=structuredClone(record);broken.bff[field]='sha256:'+'0'.repeat(64);assert.throws(()=>verifyArtifacts(broken));}
 const wrongSource=structuredClone(record);wrongSource.commit='0'.repeat(40);assert.throws(()=>verifyArtifacts(wrongSource));
});
