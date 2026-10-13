import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// Oracle: V070 ADR first slice, P21 browser file admission. No server validation is mocked.
const source=await readFile(new URL('../../apps/web/src/applications/templates/fileInput.ts',import.meta.url),'utf8');
const {decodeTemplateFile}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const bytes=s=>new TextEncoder().encode(s),id='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
const base={format:'weaveos.structure-template',version:1,application:{id,name:'结构模板'},directories:[],tables:[],forms:[],workflows:[],permissionGroups:[]};
const encode=v=>bytes(JSON.stringify(v));
test('V070 local template file admits an empty complete manifest without mutation or generated identities',()=>{const input=encode(base),before=input.slice();assert.deepEqual(decodeTemplateFile(input),base);assert.deepEqual(input,before);});
test('V070 local admission preserves nested configuration, precise decimal strings and punctuation in strings',()=>{const value={...base,tables:[{id,name:'[,}:"\\\n',directoryId:null,position:0,fields:[{id,kind:'number',name:'金额',required:false,default:'123456789012345678901234567890.12',config:{precision:38,scale:2,roundingPlaces:2,roundingMode:'half_up'},presentation:{helpText:null,displayTimeZone:null}}]}]};assert.deepEqual(decodeTemplateFile(encode(value)),value);});
test('V070 repeated keys in different objects are independent and Unicode escaped strings are decoded',()=>{const raw=JSON.stringify({...base,directories:[{id,name:'甲',parentId:null,position:0},{id:'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',name:'乙',parentId:null,position:1}]}).replace('甲','\\u7532');assert.deepEqual(decodeTemplateFile(bytes(raw)),JSON.parse(raw));});
for(const [name,raw] of [
 ['root duplicate',JSON.stringify(base).replace('"version":1','"version":1,"version":1')],
 ['nested duplicate',JSON.stringify(base).replace('"name":"结构模板"','"name":"结构模板","name":"偷换"')],
 ['escaped duplicate',JSON.stringify(base).replace('"version":1','"version":1,"ver\\u0073ion":1')],
 ['nested escaped duplicate',JSON.stringify(base).replace('"name":"结构模板"','"name":"结构模板","na\\u006de":"偷换"')],
 ['trailing value',JSON.stringify(base)+'{}'],['trailing comma',JSON.stringify(base).replace('"version":1','"version":1,')],
 ['comment',JSON.stringify(base)+'/* ignored? */'],['bad escape',JSON.stringify(base).replace('结构模板','\\x41')],
 ['array root','[]'],['null root','null'],['number root','1'],['string root','"template"'],['empty',''],
])test('V070 rejects '+name,()=>assert.equal(decodeTemplateFile(bytes(raw)),null));
for(const key of ['records','attachments','secrets','instances','ownerUserId'])test('V070 rejects forbidden extra root '+key,()=>assert.equal(decodeTemplateFile(encode({...base,[key]:[]})),null));
for(const key of ['directories','tables','forms','workflows','permissionGroups']){
 test('V070 requires collection '+key,()=>{const v={...base};delete v[key];assert.equal(decodeTemplateFile(encode(v)),null);});
 for(const wrong of [null,{},false])test('V070 rejects non-array '+key+' '+JSON.stringify(wrong),()=>assert.equal(decodeTemplateFile(encode({...base,[key]:wrong})),null));
}
for(const [key,value] of [['format','weaveos.structure-template.v2'],['version',2],['version','1'],['application',null],['application',[]]])test('V070 rejects invalid '+key+' '+JSON.stringify(value),()=>assert.equal(decodeTemplateFile(encode({...base,[key]:value})),null));
test('V070 rejects invalid UTF8 instead of substituting a replacement character',()=>{const input=bytes(JSON.stringify(base)),n=input.indexOf(0xe7);assert.ok(n>=0);input[n]=0xff;assert.equal(decodeTemplateFile(input),null);});
test('V070 enforces raw byte size including whitespace before parsing',()=>{const raw=JSON.stringify(base),space=' '.repeat(1048576-bytes(raw).length);assert.deepEqual(decodeTemplateFile(bytes(raw+space)),base);assert.equal(decodeTemplateFile(bytes(raw+space+' ')),null);});
test('V070 caps container depth without counting brackets or braces inside a string',()=>{const v={...base,application:{id,name:'['.repeat(100)}};assert.deepEqual(decodeTemplateFile(encode(v)),v);let nested={};for(let i=0;i<33;i++)nested={child:nested};assert.equal(decodeTemplateFile(encode({...base,application:nested})),null);});
for(const token of ['9007199254740993','-9007199254740993','1e400','1e-400','1.234567890123456789','2e0'])test('V070 integer metadata cannot be silently changed by JSON numeric decoding: '+token,()=>{const raw=JSON.stringify({...base,directories:[{id,name:'目录',parentId:null,position:0}]}).replace('"position":0','"position":'+token);assert.equal(decodeTemplateFile(bytes(raw)),null);});
test('V070 rejects UTF8 BOM as non-JSON rather than stripping input bytes',()=>{const input=encode(base);assert.equal(decodeTemplateFile(new Uint8Array([0xef,0xbb,0xbf,...input])),null);});
