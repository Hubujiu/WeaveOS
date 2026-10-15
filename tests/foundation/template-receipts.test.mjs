import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// V070 ADR HTTP + Data exact 201 five-key receipt; never infer from the implementation.
const source=await readFile(new URL('../../apps/web/src/applications/templates/receipts.ts',import.meta.url),'utf8');
const {validTemplatePreflight,validTemplateImportReceipt}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const op='aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',app='bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb';
const counts={directories:2,tables:1,fields:11,forms:2,workflows:3,permissionGroups:1};
const preflight={valid:true,counts},receipt={operationId:op,appId:app,policyRevision:1,structureVersion:1,status:'imported'};
test('V070 exact preflight allows empty and complete bounded structure counts without claiming import',()=>{assert.equal(validTemplatePreflight(preflight),true);assert.equal(validTemplatePreflight({valid:true,counts:{directories:0,tables:0,fields:0,forms:0,workflows:0,permissionGroups:0}}),true);assert.equal(validTemplatePreflight({valid:true,counts:{directories:1000,tables:128,fields:25600,forms:256,workflows:128,permissionGroups:128}}),true);});
test('V070 preflight rejects non-success, unknown data, incomplete or nonnumeric counts',()=>{for(const value of [null,[],{...preflight,valid:false},{...preflight,appId:app},{...preflight,counts:null},{...preflight,counts:{...counts,records:2}},{valid:true,counts:{tables:1}},...[-1,1.5,'1',NaN,Infinity,9007199254740992].map(n=>({...preflight,counts:{...counts,tables:n}}))])assert.equal(validTemplatePreflight(value),false);});
test('V070 preflight cannot claim counts above the admitted complete-manifest bounds',()=>{for(const[key,max]of Object.entries({directories:1000,tables:128,fields:25600,forms:256,workflows:128,permissionGroups:128}))assert.equal(validTemplatePreflight({valid:true,counts:{...counts,[key]:max+1}}),false);});
test('V070 import accepts only the original operation and exact initial imported receipt',()=>assert.equal(validTemplateImportReceipt(receipt,op),true));
test('V070 import rejects forged operation, lifecycle, extra values, missing keys and noncanonical IDs',()=>{const missing={...receipt};delete missing.structureVersion;for(const value of [null,[],missing,{...receipt,operationId:app},{...receipt,status:'enabled'},{...receipt,status:'pending'},{...receipt,policyRevision:2},{...receipt,structureVersion:0},{...receipt,ownerUserId:op},...['bad',app.toUpperCase(),'00000000-0000-0000-0000-000000000000'].map(appId=>({...receipt,appId}))])assert.equal(validTemplateImportReceipt(value,op),false);assert.equal(validTemplateImportReceipt(receipt,'bad'),false);});
