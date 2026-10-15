import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {stripTypeScriptTypes} from 'node:module';
// V037 P3 plus V075 P4: execution status, never generic accepted=success.
const source=await readFile(new URL('../../apps/web/src/applications/workflows/taskActionContract.ts',import.meta.url),'utf8');
const {validWorkflowOperation}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
const id=n=>'77777777-7777-4777-8777-'+String(n).padStart(12,'0');
const expected={operationId:id(1),instanceId:id(3)};
const pending={operationId:id(1),commandId:id(2),instanceId:id(3),status:'pending'};
const success={...pending,status:'success',instanceState:'completed',sequence:1};
const noEffect={...pending,status:'no_effect',instanceState:'unchanged',reason:'task_inactive',sequence:0};
for(const [name,value,status,method] of [
 ['accepted pending',pending,202,'POST'],['read pending',pending,200,'GET'],['replay success',success,200,'POST'],['read success',success,200,'GET'],['replay no effect',noEffect,200,'POST'],['read no effect zero sequence',noEffect,200,'GET'],
 ['success active',{...success,instanceState:'active'},200,'GET'],['success rejected',{...success,instanceState:'rejected'},200,'GET'],
])test('workflow operation accepts '+name,()=>assert.equal(validWorkflowOperation(value,expected,status,method),true));
test('workflow operation preserves first accepted command binding',()=>assert.equal(validWorkflowOperation(success,{...expected,commandId:id(2)},200,'GET'),true));
for(const [name,value,status,method] of [
 ['pending POST200',pending,200,'POST'],['pending GET202',pending,202,'GET'],['success POST202',success,202,'POST'],['success wrong HTTP',success,201,'GET'],
 ['extra private values',{...success,values:{secret:'hidden'}},200,'GET'],['pending terminal fields',{...pending,sequence:1},202,'POST'],
 ['wrong operation',{...success,operationId:id(99)},200,'GET'],['wrong instance',{...success,instanceId:id(99)},200,'GET'],
 ['zero command',{...success,commandId:'00000000-0000-0000-0000-000000000000'},200,'GET'],['bad command',{...success,commandId:'../command'},200,'GET'],
 ['missing command',{...success,commandId:undefined},200,'GET'],['non execution confirmed',{...pending,status:'confirmed'},200,'GET'],
 ['pending fake success state',{...pending,instanceState:'completed'},202,'POST'],['success zero sequence',{...success,sequence:0},200,'GET'],
 ['success fractional sequence',{...success,sequence:1.2},200,'GET'],['unsafe sequence',{...success,sequence:9007199254740992},200,'GET'],
 ['missing sequence',Object.fromEntries(Object.entries(success).filter(([k])=>k!=='sequence')),200,'GET'],['success unknown state',{...success,instanceState:'finished'},200,'GET'],
 ['no effect missing reason',Object.fromEntries(Object.entries(noEffect).filter(([k])=>k!=='reason')),200,'GET'],['no effect arbitrary reason',{...noEffect,reason:'SQL internal detail'},200,'GET'],
 ['no effect negative sequence',{...noEffect,sequence:-1},200,'GET'],['no effect changed state',{...noEffect,instanceState:'completed'},200,'GET'],
 ['success no effect reason',{...success,reason:'cancelled'},200,'GET'],['null',null,200,'GET'],['array',[],200,'GET'],
])test('workflow operation rejects '+name,()=>assert.equal(validWorkflowOperation(value,expected,status,method),false));
test('workflow operation rejects a changed command after acceptance',()=>assert.equal(validWorkflowOperation(success,{...expected,commandId:id(99)},200,'GET'),false));
