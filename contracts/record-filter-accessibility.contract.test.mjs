import test from 'node:test';import assert from 'node:assert/strict';import {readFileSync,existsSync} from 'node:fs';import {createRequire} from 'node:module';import ts from 'typescript';
// Render the actual React accessibility contract in the dependency-installed
// contract suite, without a browser, network or substituted hooks.
const require=createRequire(new URL('../apps/web/package.json',import.meta.url)),React=require('react'),{renderToStaticMarkup}=require('react-dom/server'),cache=new Map();
function load(url){if(cache.has(url.href))return cache.get(url.href);const m={exports:{}};cache.set(url.href,m.exports);const code=ts.transpileModule(readFileSync(url,'utf8'),{compilerOptions:{jsx:ts.JsxEmit.ReactJSX,module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText;const local=id=>{if(!id.startsWith('.'))return require(id);for(const ext of ['.ts','.tsx']){const child=new URL(id+ext,url);if(existsSync(child))return load(child);}throw Error('Missing local module '+id);};new Function('require','module','exports',code)(local,m,m.exports);cache.set(url.href,m.exports);return m.exports;}
const {RecordFilterReference}=load(new URL('../apps/web/src/applications/records/RecordFilterReference.tsx',import.meta.url));
const id=n=>'abababab-abab-4bab-8bab-'+String(n).padStart(12,'0'),noop=()=>{};
const props={actorId:id(1),appId:id(2),viewId:id(3),field:{id:id(4),name:'成员',kind:'member',required:false,presentation:{helpText:null,displayTimeZone:null}},value:null,onChange:noop,onUnauthorized:noop,onIdentityMismatch:noop};
test('P26 rendered reference controls have single-token unique aria-controls across repeated field conditions',()=>{
 const html=renderToStaticMarkup(React.createElement(React.Fragment,null,...['条件 1.1','条件 1.2'].map(label=>React.createElement(RecordFilterReference,{...props,label,key:label}))));
 const ids=[...html.matchAll(/aria-controls="([^"]+)"/g)].map(m=>m[1]);assert.equal(ids.length,2);for(const value of ids)assert.doesNotMatch(value,/\s/,'aria-controls must reference one real picker ID, not split a human label into tokens');assert.equal(new Set(ids).size,2);
});
