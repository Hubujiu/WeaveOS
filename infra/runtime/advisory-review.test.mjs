import test from 'node:test';
import assert from 'node:assert/strict';
import {unresolvedAdvisories} from './advisory-review.mjs';
const id='GO-2026-6443';
const proof=()=>({modulePath:'google.golang.org/grpc',version:'v1.84.0',sum:'h1:soMyaPJ8pAak5PIQ0DGBUir0XRo2fRoMqhNWMLlLxO0=',goModSum:'h1:ljCht0DrxQrXBDRTZp52Qxh3Ffk8CdYm2sj4O2QN2C0=',replacement:false,transportSHA256:'6ff2da17e276ba22a782dffe467ec32b664895d12faccfbb5fc1ec40d59dcf6c',regression:{exitCode:0,passed:1,failed:0,skipped:0}});
test('reviewed grpc advisory requires the exact verified release and fresh transport regression',()=>{
 const ids=[id,'GO-2026-unrelated'];const evidence=proof();const before=structuredClone(evidence);
 assert.deepEqual(unresolvedAdvisories(ids,evidence),['GO-2026-unrelated']);
 assert.deepEqual(ids,[id,'GO-2026-unrelated']);assert.deepEqual(evidence,before);
});
test('missing evidence never suppresses an advisory',()=>{for(const p of [undefined,null,{},false])assert.deepEqual(unresolvedAdvisories([id],p),[id]);});
test('other module version sum replacement or source never inherits the review',()=>{
 const changes={modulePath:'another/module',version:'v1.84.1',sum:'h1:tampered',goModSum:'h1:tampered',replacement:true,transportSHA256:'0'.repeat(64)};
 for(const [field,value] of Object.entries(changes)){const p=proof();p[field]=value;assert.deepEqual(unresolvedAdvisories([id],p),[id],field);}
 for(const field of Object.keys(proof())){const p=proof();delete p[field];assert.deepEqual(unresolvedAdvisories([id],p),[id],`missing ${field}`);}
});
test('failed skipped absent duplicate or nonzero-exit regression cannot justify the review',()=>{
 for(const regression of [null,{}, {exitCode:1,passed:1,failed:0,skipped:0},{exitCode:0,passed:0,failed:0,skipped:0},{exitCode:0,passed:2,failed:0,skipped:0},{exitCode:0,passed:1,failed:1,skipped:0},{exitCode:0,passed:1,failed:0,skipped:1},{exitCode:0,passed:'1',failed:0,skipped:0}])assert.deepEqual(unresolvedAdvisories([id],{...proof(),regression}),[id]);
});
test('the review does not create a blanket version or vulnerability exemption',()=>{
 for(const version of ['v1.83.2','v1.85.0','v1.84.0-dev','1.84.0',''])assert.deepEqual(unresolvedAdvisories([id],{...proof(),version}),[id]);
 for(const ids of [[],['GO-2026-5932'],['GO-2026-6444'],['GO-2026-6443-extra']])assert.deepEqual(unresolvedAdvisories(ids,proof()),ids);
});
