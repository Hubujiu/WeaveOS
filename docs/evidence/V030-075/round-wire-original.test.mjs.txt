import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
const source=readFileSync(new URL('../../apps/web/src/applications/workflows/roundContract.ts',import.meta.url),'utf8');
const {validRoundPreview:preview,validRoundStartReceipt:receipt}=await import('data:text/javascript;base64,'+Buffer.from(stripTypeScriptTypes(source,{mode:'transform'})).toString('base64'));
// V069 PRD/ADR third slice: authoritative current eligibility and minimum CAS,
// no business values/graph/approvers; accepted is durable intent, not completion.
const id=n=>'aaaaaaaa-aaaa-4aaa-8aaa-'+String(n).padStart(12,'0');
const current={instanceId:id(1),flowId:id(2),roundNumber:2,latestInstanceId:id(1),state:'rejected',definitionVersion:3,workflowRevision:7,schemaVersion:2,recordVersion:9,canRework:true,canResubmit:true,canReview:false,editableFieldIds:[id(20)]};
test('V069 authorized rejected latest round keeps current CAS and field mask',()=>assert.equal(preview(current,id(1)),true));
test('V069 old round is a valid read-only preview',()=>assert.equal(preview({...current,latestInstanceId:id(3),canRework:false,canResubmit:false,editableFieldIds:[]},id(1)),true));
test('V069 completed latest round may authorize review without rework',()=>assert.equal(preview({...current,state:'completed',canRework:false,canResubmit:false,canReview:true,editableFieldIds:[]},id(1)),true));
for(const state of ['starting','active','no_effect'])test('V069 '+state+' has no terminal actions',()=>assert.equal(preview({...current,state,canRework:false,canResubmit:false,editableFieldIds:[]},id(1)),true));
for(const [name,change]of [['other instance',{instanceId:id(99)}],['zero flow',{flowId:'00000000-0000-0000-0000-000000000000'}],['uppercase id',{flowId:id(2).toUpperCase()}],['missing latest',{latestInstanceId:undefined}],['bad state',{state:'accepted'}],['zero round',{roundNumber:0}],['unsafe version',{recordVersion:9007199254740992}],['string revision',{workflowRevision:'7'}],['null schema',{schemaVersion:null}],['fractional definition',{definitionVersion:1.2}],['coerced capability',{canReview:'false'}],['null field mask',{editableFieldIds:null}],['duplicate field',{editableFieldIds:[id(20),id(20)]}],['non-id field',{editableFieldIds:['amount']}],['private graph',{graph:{}}],['business values',{values:{}}],['approvers',{approvers:[id(8)]}],['old writable',{latestInstanceId:id(3)}],['active writable',{state:'active'}],['completed resubmit',{state:'completed'}]])test('V069 preview rejects '+name,()=>assert.equal(preview({...current,...change},id(1)),false));
const accepted={operationId:id(4),flowId:id(2),previousInstanceId:id(1),instanceId:id(5),roundKind:'resubmit',roundNumber:3,status:'accepted'};
test('V069 resubmit accepts only the minimum new-round receipt',()=>assert.equal(receipt(accepted,id(4),id(2),id(1),'resubmit'),true));
test('V069 review is an independent new round',()=>assert.equal(receipt({...accepted,roundKind:'review'},id(4),id(2),id(1),'review'),true));
for(const [name,change]of [['other operation',{operationId:id(9)}],['other flow',{flowId:id(9)}],['other parent',{previousInstanceId:id(9)}],['same instance',{instanceId:id(1)}],['wrong kind',{roundKind:'review'}],['completed',{status:'completed'}],['zero round',{roundNumber:0}],['string round',{roundNumber:'3'}],['unsafe round',{roundNumber:9007199254740992}],['no instance',{instanceId:undefined}],['command',{commandId:id(9)}],['values',{values:{}}]])test('V069 receipt rejects '+name,()=>assert.equal(receipt({...accepted,...change},id(4),id(2),id(1),'resubmit'),false));
