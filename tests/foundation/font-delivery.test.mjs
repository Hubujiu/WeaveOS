import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,existsSync} from 'node:fs';
import {createHash} from 'node:crypto';
const root=new URL('../../apps/web/src/',import.meta.url);
const original=readFileSync(new URL('../../docs/evidence/V030-075/noto-v41-official.css.snapshot',import.meta.url),'utf8');
test('approved Noto family and weights have no external render-blocking stylesheet',()=>{
 const style=readFileSync(new URL('style.css',root),'utf8');
 assert.doesNotMatch(style,/@import\s+(?:url\()?['"]?https?:/i);
 assert.match(style,/@import\s+['"]\.\/assets\/fonts\/noto-sans-sc\.css['"]/);
});
test('all original official font faces and unicode ranges survive same-origin packaging',()=>{
 const path=new URL('assets/fonts/noto-sans-sc.css',root);
 assert.equal(existsSync(path),true,'same-origin font stylesheet exists');
 const css=readFileSync(path,'utf8');
 assert.doesNotMatch(css,/https?:\/\//);
 const normalize=s=>s.replace(/url\([^)]+\)/g,'url(FONT)');
 assert.equal(normalize(css),normalize(original));
 const urls=[...css.matchAll(/url\(([^)]+)\)/g)].map(m=>m[1]);
 const source=[...original.matchAll(/url\(([^)]+)\)/g)].map(m=>m[1]);
 const manifest=JSON.parse(readFileSync(new URL('assets/fonts/sources.json',root),'utf8'));
 assert.equal(new Set(urls).size,101);assert.equal(urls.length,303);
 assert.equal(manifest.files.length,101);
 for(let i=0;i<urls.length;i++){
  const relative=urls[i];assert.match(relative,/^\.\/noto-sc-v41-[a-f0-9]{16}\.woff2$/);
  const row=manifest.files.find(r=>r.file===relative.slice(2));assert.ok(row);
  assert.equal(row.url,source[i]);
  const bytes=readFileSync(new URL('assets/fonts/'+row.file,root));
  assert.equal(bytes.subarray(0,4).toString(),'wOF2');
  assert.equal(bytes.length,row.bytes);
  assert.equal(createHash('sha256').update(bytes).digest('hex'),row.sha256);
 }
 assert.match(readFileSync(new URL('assets/fonts/OFL.txt',root),'utf8'),/SIL OPEN FONT LICENSE Version 1.1/);
});

test('distributed font license is retained in the built public assets',()=>{
 const published=new URL('../../apps/web/public/fonts/OFL.txt',import.meta.url);
 assert.equal(existsSync(published),true);assert.equal(readFileSync(published,'utf8'),readFileSync(new URL('assets/fonts/OFL.txt',root),'utf8'));
});
