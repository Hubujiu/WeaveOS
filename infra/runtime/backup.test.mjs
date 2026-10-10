import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { randomBytes, createHash } from 'node:crypto';
import { resolve } from 'node:path';
import { backupDatabase, restoreDatabase, privateFile } from './backup.mjs';
const container=process.env.WEAVEOS_BACKUP_TEST_CONTAINER;
if(!container?.startsWith('weaveos-v010-'))throw new Error('Isolated PostgreSQL18 container required');
const dir=resolve('.work/backup-tests',String(Date.now()));mkdirSync(dir,{recursive:true});
const user=process.env.WEAVEOS_BACKUP_TEST_USER??'weaveos_test';
if(!/^weaveos_[a-zA-Z0-9_]+$/.test(user))throw new Error('Isolated backup test identity required');
const sql=(database,text)=>execFileSync('docker',['exec','-i',container,'psql','-X','-A','-t','-v','ON_ERROR_STOP=1','-v','VERBOSITY=verbose','-U',user,'-d',database],{input:text,stdio:['pipe','pipe','pipe'],encoding:'utf8'});
const suffix=Date.now(),source=`weaveos_backup_source_${suffix}`,target=`weaveos_backup_target_${suffix}`;
sql('postgres',`CREATE DATABASE ${source}; CREATE DATABASE ${target};`);
// Synthetic source independent of backup implementation. Restore must preserve
// status, credential representation and consumed invitation state, not just counts.
sql(source,"CREATE SCHEMA auth; CREATE TABLE auth.recovery_fixture (id integer PRIMARY KEY,status text,password_hash text,invitation_consumed boolean); INSERT INTO auth.recovery_fixture VALUES (1,'disabled','synthetic-one-way-hash',true);");
const keyFile=resolve(dir,'key'),backupFile=resolve(dir,'snapshot.enc');writeFileSync(keyFile,randomBytes(32),{mode:0o600,flag:'wx'});
const options={container,user,database:source,keyFile,backupFile};
test('real encrypted logical backup restores into an isolated empty database',()=>{
 backupDatabase(options);
 assert.ok(existsSync(backupFile),'encrypted backup must actually be persisted');
 assert.equal(readFileSync(backupFile).includes(Buffer.from('synthetic-one-way-hash')),false);
 restoreDatabase({...options,database:target});
 const restored=sql(target,'SELECT status,password_hash,invitation_consumed FROM auth.recovery_fixture;');
 assert.equal(restored.trim(),'disabled|synthetic-one-way-hash|t','restore must retain the complete original row');
});
test('missing source produces a failure and no successful backup file',()=>{
 const failedFile=resolve(dir,'must-not-exist.enc'),alertFile=resolve(dir,'backup-alerts.jsonl');
 assert.throws(()=>backupDatabase({...options,database:'weaveos_missing_backup_source',backupFile:failedFile,alertFile}));
 assert.equal(existsSync(failedFile),false,'failed dump must not publish success artifact');
 assert.deepEqual(JSON.parse(readFileSync(alertFile,'utf8')).codes,['BACKUP'],'real backup failure must reach the local receiver');
});
test('wrong-key restore fails before any database mutation',()=>{
 const wrongKey=resolve(dir,'wrong-key');writeFileSync(wrongKey,randomBytes(32),{mode:0o600,flag:'wx'});
 const untouched=`weaveos_backup_untouched_${suffix}`;sql('postgres',`CREATE DATABASE ${untouched};`);
 assert.throws(()=>restoreDatabase({...options,database:untouched,keyFile:wrongKey}));
 assert.equal(sql(untouched,"SELECT to_regclass('auth.recovery_fixture') IS NULL;").trim(),'t');
});

test('restricted backup preserves migration ledger sequence and remains unable to advance it',()=>{
 const live=`weaveos_backup_ledger_${suffix}`,restored=`weaveos_backup_ledger_restored_${suffix}`;
 sql('postgres',`CREATE DATABASE ${live}; CREATE DATABASE ${restored};`);
 sql(live,readFileSync('db/migrations/00001_auth.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00002_personnel.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00003_query_drafts.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00004_query_revision_writers.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00005_table_presets.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00006_apps_policy.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00007_app_structure.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00008_app_records.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00009_record_save_history.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00010_member_source_label_width.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00011_record_reference_defaults.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00012_actual_reference_defaults.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00013_record_command_fence.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00014_workflow_command_ledger.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00015_workflow_catalog.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00016_workflow_management_operations.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00017_workflow_publications.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00018_workflow_execution_projection.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00019_workflow_execution_recovery.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00020_workflow_evidence.sql','utf8').split('-- +goose Down')[0]);
 sql(live,readFileSync('db/migrations/00021_workflow_task_operations.sql','utf8').split('-- +goose Down')[0]);
 // Later schema expansions include LOCK TABLE; keep each migration atomic.
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00022_workflow_lifecycle_operations.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00023_workflow_record_order_index.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00024_workflow_trigger_configuration.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00025_workflow_manual_start_operations.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00026_workflow_personal_inbox_index.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00027_workflow_independent_journal.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00028_workflow_publication_history.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00029_workflow_deletions.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00030_workflow_rounds.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00031_workflow_round_operations.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00032_application_template_import.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00033_application_table_presets.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00034_record_lifecycle.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,'BEGIN;\n'+readFileSync('db/migrations/00035_structure_deletion.sql','utf8').split('-- +goose Down')[0]+'\nCOMMIT;');
 sql(live,"SELECT setval('applications.record_command_fence_epoch_seq',41,true);");
 sql(live,`INSERT INTO auth.users(id,account) VALUES('77777777-7777-4777-8777-777777777777','preset-backup-synthetic');
 INSERT INTO personnel.table_presets(id,owner_id,view_key,name,slot,filter_json,hidden_column_ids,schema_version,version,created_at,updated_at)
 VALUES('88888888-8888-4888-8888-888888888888','77777777-7777-4777-8777-777777777777','members','持久方案😀',20,'{"children":[{"field":"account","operator":"eq","value":"A"}],"operator":"and"}','["identities"]',1,7,'2026-10-01T00:00:00Z','2026-10-02T00:00:00Z');`);
 // Synthetic valid protocol metadata tests persistence, not user authorization.
 const recoveryID=n=>n.toString(16).padStart(8,'0')+'-0000-4000-8000-'+n.toString(16).padStart(12,'0');
 const payload=Buffer.from('5756465041590001'+'01'.repeat(32)+'0000000000','hex');
 const command={ProtocolVersion:2,CommandID:recoveryID(901),AppID:recoveryID(902),TableID:recoveryID(903),RecordID:recoveryID(904),InstanceID:recoveryID(905),TaskID:'',ActorID:'77777777-7777-4777-8777-777777777777',Action:'withdraw',RecordVersion:3,FenceEpoch:41,TaskEpoch:0,ExpectedSequence:7,PayloadHash:[...createHash('sha256').update(payload).digest()],ViewID:recoveryID(906),FlowID:recoveryID(907),VersionID:recoveryID(908),TargetNodeID:'',DefinitionVersion:2,SchemaVersion:4};
 const envelope=[Buffer.from('575646434d440002','hex')];
 for(const key of ['CommandID','AppID','TableID','ViewID','RecordID','FlowID','VersionID','InstanceID','TaskID','ActorID','Action','TargetNodeID']){
  const value=Buffer.from(command[key],'utf8'),length=Buffer.alloc(4);length.writeUInt32BE(value.length);envelope.push(length,value);
 }
 for(const key of ['DefinitionVersion','SchemaVersion','RecordVersion','FenceEpoch','TaskEpoch','ExpectedSequence']){
  const value=Buffer.alloc(8);value.writeBigUInt64BE(BigInt(command[key]));envelope.push(value);
 }
 envelope.push(Buffer.from(command.PayloadHash));
 const commandHash=createHash('sha256').update(Buffer.concat(envelope)).digest('hex');
 sql(live,`INSERT INTO applications.workflow_commands(command_id,command_json,command_hash,state,execution_payload)
 VALUES('${command.CommandID}',$fixture$${JSON.stringify(command)}$fixture$::jsonb,decode('${commandHash}','hex'),'pending',decode('${payload.toString('hex')}','hex'));
 INSERT INTO applications.workflow_dispatch(command_id,protocol_version,next_attempt_at,attempts,lease_token,lease_until,last_error)
 VALUES('${command.CommandID}',2,'2026-10-05T17:00:45Z',3,'${recoveryID(909)}','2026-10-05T17:00:45Z','DEPENDENCY_UNAVAILABLE');`);
 // Root's independent golden bytes, not output generated by the store under test.
 const golden=JSON.parse(readFileSync('services/bff/internal/workflowevidence/testdata/root-evidence-vectors.json','utf8'));
 const fieldBody=Buffer.concat([Buffer.from('57564645464c0001','hex'),Buffer.from(golden.fieldJSON)]);
 const manifestBody=Buffer.concat([Buffer.from('575646454d460001','hex'),Buffer.from(golden.manifestJSON)]);
 assert.equal(createHash('sha256').update(fieldBody).digest('hex'),'e7d8bf7caf8d2d9732d1133133d77182750a91942c6855fed907b2ce70defc86');
 assert.equal(createHash('sha256').update(manifestBody).digest('hex'),'826bda2b5039c347c39e9ba6f7ff50dff9871f8890cf83e81b87c833e658fd4c');
 const h=JSON.parse(golden.manifestJSON).header,field=JSON.parse(golden.fieldJSON).definition;
 sql(live,`INSERT INTO auth.users(id,account) VALUES('${h.createdBy}','evidence-backup-synthetic');
 INSERT INTO applications.apps(id,name,owner_user_id) VALUES('${h.appId}','evidence backup','${h.createdBy}');
 INSERT INTO applications.logical_tables(id,app_id,name,position,schema_version) VALUES('${h.tableId}','${h.appId}','evidence table',0,${h.schemaVersion});
 INSERT INTO applications.form_views(id,app_id,table_id,name,position) VALUES('${h.viewId}','${h.appId}','${h.tableId}','evidence view',0);
 INSERT INTO applications.workflow_evidence_blobs(app_id,field_id,content_hash,body) VALUES('${h.appId}','${field.id}',decode('${golden.fieldSHA256}','hex'),decode('${fieldBody.toString('hex')}','hex'));
 INSERT INTO applications.workflow_evidence_documents(app_id,evidence_hash,table_id,view_id,record_id,created_by,schema_version,record_version,field_count,body)
 VALUES('${h.appId}',decode('${golden.manifestSHA256}','hex'),'${h.tableId}','${h.viewId}','${h.recordId}','${h.createdBy}',${h.schemaVersion},${h.recordVersion},1,decode('${manifestBody.toString('hex')}','hex'));
 INSERT INTO applications.workflow_evidence_members(app_id,evidence_hash,field_id,content_hash) VALUES('${h.appId}',decode('${golden.manifestSHA256}','hex'),'${field.id}',decode('${golden.fieldSHA256}','hex'));`);
 // Synthetic opaque receipt tests backup bytes and CHECKs only. Real acceptance
 // and command binding are covered by Root's PG business-service tests.
 const actionOperation=recoveryID(910);
 const actionReceipt={operationId:actionOperation,commandId:recoveryID(911),instanceId:recoveryID(912),status:'pending'};
 sql(live,`INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location)
 VALUES('${h.createdBy}','${actionOperation}','${h.appId}','workflow.task.agree',decode('${'17'.repeat(32)}','hex'),$receipt$${JSON.stringify(actionReceipt)}$receipt$::jsonb,202,'/api/v1/application-workflow-operations/${actionOperation}');`);
 // Independent synthetic private configuration; restore must retain original
 // canonical text, immutable owner/view identity and the minimum mutation receipt.
 const privatePresetId=recoveryID(930),privateOperation=recoveryID(931);
 const privateState=JSON.stringify({name:'应用私人方案😀',filter:null,sort:null,hiddenColumnIds:[],columnOrder:['createdAt'],columnWidths:{createdAt:180}});
 const privateReceipt={operationId:privateOperation,id:privatePresetId,version:7};
 sql(live,`INSERT INTO applications.table_presets(id,owner_user_id,app_id,view_id,name,slot,definition_json,field_kinds,version,created_at,updated_at)
 VALUES('${privatePresetId}','${h.createdBy}','${h.appId}','${h.viewId}','应用私人方案😀',20,$preset$${privateState}$preset$,'{}',7,'2026-10-01T00:00:00Z','2026-10-02T00:00:00Z');
 INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location)
 VALUES('${h.createdBy}','${privateOperation}','${h.appId}','preset.update',decode('${'19'.repeat(32)}','hex'),$receipt$${JSON.stringify(privateReceipt)}$receipt$,200,'');`);
 // Synthetic retained lifecycle metadata; actual typed-record transitions are
 // independently exercised through real service and HTTPS tests in V072.
 const lifecycleOperation=recoveryID(940);
 sql(live,`INSERT INTO applications.record_lifecycle(app_id,table_id,record_id,deleted,record_version,changed_by,changed_at)
 VALUES('${h.appId}','${h.tableId}','${h.recordId}',true,2,'${h.createdBy}','2026-10-10T00:00:00Z');
 INSERT INTO applications.record_lifecycle_events(app_id,table_id,record_id,view_id,actor_user_id,operation_id,action,before_record_version,after_record_version,occurred_at)
 VALUES('${h.appId}','${h.tableId}','${h.recordId}','${h.viewId}','${h.createdBy}','${lifecycleOperation}','delete',1,2,'2026-10-10T00:00:00Z');`);
 // A real finite transition on a separate empty app preserves its identity,
 // event and six-key operation result without touching the populated shared app.
 const structureDeletionApp=recoveryID(950),structureDeletionOperation=recoveryID(951);
 sql(live,`INSERT INTO applications.apps(id,name,owner_user_id) VALUES('${structureDeletionApp}','Retained empty app','${h.createdBy}');
 SELECT applications.register_catalog_entry('${structureDeletionApp}');
 WITH changed AS (SELECT applications.delete_structure_resource('${structureDeletionApp}','application','${structureDeletionApp}','${h.createdBy}','${structureDeletionOperation}',0,1) AS result)
 INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location)
 SELECT '${h.createdBy}','${structureDeletionOperation}','${structureDeletionApp}','application.delete',decode('${'23'.repeat(32)}','hex'),result,200,'' FROM changed;`);
 const structureDeletionQuery="SELECT to_jsonb(a) FROM applications.apps a ORDER BY id; SELECT to_jsonb(d) FROM applications.directories d ORDER BY id; SELECT to_jsonb(t) FROM applications.logical_tables t ORDER BY id; SELECT to_jsonb(v) FROM applications.form_views v ORDER BY id; SELECT to_jsonb(e) FROM applications.structure_deletions e ORDER BY app_id,resource_kind,resource_id;";
 const lifecycleQuery="SELECT to_jsonb(l) FROM applications.record_lifecycle l ORDER BY app_id,table_id,record_id; SELECT to_jsonb(e) FROM applications.record_lifecycle_events e ORDER BY id;";
 const privateQuery="SELECT id,owner_user_id,app_id,view_id,name,slot,encode(convert_to(definition_json,'UTF8'),'hex'),field_kinds,version,created_at,updated_at FROM applications.table_presets ORDER BY id;";
 sql(live,"CREATE TABLE public.goose_db_version(id serial PRIMARY KEY,version_id bigint); INSERT INTO public.goose_db_version(version_id) VALUES(0),(1);");
 sql(live,readFileSync('infra/runtime/roles.sql','utf8'));
 sql('postgres',"DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='weaveos_backup_probe') THEN CREATE ROLE weaveos_backup_probe LOGIN; END IF; END $$; GRANT auth_backup TO weaveos_backup_probe;");
 const encrypted=resolve(dir,'ledger.enc');
 backupDatabase({...options,user:'weaveos_backup_probe',database:live,backupFile:encrypted});
 restoreDatabase({...options,database:restored,backupFile:encrypted});
 const operationQuery="SELECT actor_user_id,operation_id,app_id,operation_kind,encode(fingerprint,'hex'),result_json,http_status,location,created_at FROM applications.operations ORDER BY actor_user_id,operation_id;";
 assert.equal(sql(restored,operationQuery),sql(live,operationQuery),'pending operation receipt bytes must survive restricted backup');
 assert.ok(sql(restored,operationQuery).includes(actionOperation));
 assert.equal(sql(restored,privateQuery),sql(live,privateQuery),'private configuration and immutable scope survive restricted backup exactly');
 assert.equal(sql(restored,lifecycleQuery),sql(live,lifecycleQuery),'retained deletion and permanent lifecycle events survive restricted backup exactly');
 assert.equal(sql(restored,structureDeletionQuery),sql(live,structureDeletionQuery),'retained structure metadata and deletion events survive restricted backup exactly');
 assert.ok(sql(restored,operationQuery).includes(structureDeletionOperation),'closed structure deletion receipt survives restore');
 assert.ok(sql(restored,privateQuery).includes(Buffer.from(privateState).toString('hex')),'actual canonical private settings survive restore');
 assert.ok(sql(restored,operationQuery).includes(privateOperation),'actual minimum private receipt survives restore');
 // PostgreSQL may deparse equivalent casts differently after pg_restore.
 // Validate catalog flags and actual behavior, not rendered expression text.
 const checks="SELECT count(*)=3 AND bool_and(contype='c' AND convalidated AND conenforced AND NOT condeferrable) AS guards_valid FROM pg_constraint WHERE conrelid='applications.operations'::regclass AND conname IN ('ck_operation_kind','ck_operation_result','ck_workflow_task_operation_result');";
 const normalKinds=['application.create','group.create','group.update','members.replace','grants.replace','directory.create','directory.update','table.create','table.update','form.create','form.update','definition.save','record.create','record.edit','draft.create','draft.update','draft.discard','workflow.definition.save','workflow.enable','workflow.close'];
 const state=(kind='workflow.task.agree',body=actionReceipt,status=202,location=`/api/v1/application-workflow-operations/${actionOperation}`)=>({kind,body,status,location});
 const literal=value=>value===null?'NULL':"'"+String(value).replaceAll("'","''")+"'";
 const update=({kind,body,status,location})=>`UPDATE applications.operations SET operation_kind=${literal(kind)},result_json=${body===null?'NULL':literal(JSON.stringify(body))+'::jsonb'},http_status=${status===null?'NULL':status},location=${literal(location)} WHERE operation_id='${actionOperation}';`;
 // Existing RecordMutationResult and DraftOperationResult are closed contracts.
 // Legal old kinds must carry their own receipt shape, not an empty object.
 const recordResult={operationId:actionOperation,id:recoveryID(921),recordVersion:1,schemaVersion:4,createdAt:'2026-10-06T00:00:00Z',updatedAt:'2026-10-06T00:00:00Z'};
 const draftResult={operationId:actionOperation,id:recoveryID(922),draftVersion:1};
 const legacyResult=kind=>kind.startsWith('record.')?recordResult:kind.startsWith('draft.')?draftResult:{};
 const valid=[...normalKinds.map(kind=>state(kind,legacyResult(kind),kind==='draft.discard'?204:kind.endsWith('.create')?201:200,'')),state(),state('workflow.task.reject'),state('workflow.task.agree',null,null,null)];
 const invalid=[
  state('unknown.kind',null,null,null),
  state('application.create',{},202,''),
  state('workflow.task.agree',actionReceipt,200),
  state('workflow.task.reject',{...actionReceipt,status:'success'}),
  state('workflow.task.agree',{...actionReceipt,extra:'forbidden'}),
  state('workflow.task.agree',{...actionReceipt,operationId:recoveryID(920)}),
  state('workflow.task.agree',actionReceipt,202,'/wrong'),
  state('workflow.task.agree',null,202,null),
  state('workflow.task.agree',actionReceipt,null),
  state('workflow.task.agree',actionReceipt,202,null),
  state('workflow.task.agree',[]),
 ];
 for(const key of ['operationId','commandId','instanceId','status']){
  const missing={...actionReceipt};delete missing[key];invalid.push(state('workflow.task.agree',missing));
  invalid.push(state('workflow.task.agree',{...actionReceipt,[key]:42}));
 }
 for(const key of ['commandId','instanceId']){
  invalid.push(state('workflow.task.agree',{...actionReceipt,[key]:'00000000-0000-0000-0000-000000000000'}));
  invalid.push(state('workflow.task.agree',{...actionReceipt,[key]:'not-a-uuid'}));
 }
 for(const database of [live,restored]){
  assert.equal(sql(database,checks).trim(),'t','all three CHECK guards must exist, be validated and enforced');
  for(const row of valid)sql(database,`BEGIN; ${update(row)} ROLLBACK;`);
  for(const [index,row] of invalid.entries()){
   // Only 23514 proves a CHECK guard rejected the mutation. Syntax, connection
   // and unrelated constraint failures propagate and fail this test.
   sql(database,`DO $guard$ DECLARE violated text; BEGIN BEGIN ${update(row)} RAISE EXCEPTION 'invalid receipt ${index} accepted' USING ERRCODE='P0001'; EXCEPTION WHEN check_violation THEN GET STACKED DIAGNOSTICS violated=CONSTRAINT_NAME; IF violated NOT IN ('ck_operation_kind','ck_operation_result','ck_workflow_task_operation_result') THEN RAISE; END IF; END; END $guard$;`);
  }
  assert.equal(sql(database,operationQuery),sql(live,operationQuery),'guard probes must preserve original pending receipt');
 }
 const recoveryColumns="c.command_id,c.command_json,encode(c.command_hash,'hex'),c.state,c.receipt_json,c.created_at,encode(c.execution_payload,'hex'),d.protocol_version,d.created_at,d.next_attempt_at,d.attempts,d.lease_token,d.lease_until,d.last_error";
 const recoveryQuery='SELECT '+recoveryColumns+' FROM applications.workflow_commands c JOIN applications.workflow_dispatch d USING(command_id);';
 const recovered=sql(restored,recoveryQuery);
 assert.equal(recovered,sql(live,recoveryQuery),'backup must preserve original accepted payload, identity, result state and pending retry metadata exactly');
 assert.ok(recovered.includes(payload.toString('hex'))&&recovered.includes(commandHash)&&recovered.includes(command.CommandID),'independent original wire evidence must actually be present after restore');
 const evidenceQueries=[
  "SELECT app_id,field_id,encode(content_hash,'hex'),encode(body,'hex'),created_at FROM applications.workflow_evidence_blobs ORDER BY app_id,field_id,content_hash;",
  "SELECT app_id,encode(evidence_hash,'hex'),table_id,view_id,record_id,created_by,schema_version,record_version,field_count,encode(body,'hex'),created_at FROM applications.workflow_evidence_documents ORDER BY app_id,evidence_hash;",
  "SELECT app_id,encode(evidence_hash,'hex'),field_id,encode(content_hash,'hex') FROM applications.workflow_evidence_members ORDER BY app_id,evidence_hash,field_id;"
 ];
 for(const query of evidenceQueries)assert.equal(sql(restored,query),sql(live,query),'restricted backup must preserve immutable evidence bytes, metadata and membership exactly');
 assert.ok(sql(restored,evidenceQueries[0]).includes(fieldBody.toString('hex')),'actual original field bytes must survive restore');
 assert.ok(sql(restored,evidenceQueries[1]).includes(manifestBody.toString('hex')),'actual original manifest bytes must survive restore');
 const guardedQueries=[...evidenceQueries,recoveryQuery,operationQuery,privateQuery,"SELECT last_value,is_called FROM public.goose_db_version_id_seq;","SELECT last_value,is_called FROM applications.record_command_fence_epoch_seq;"];
 const guardedBefore=guardedQueries.map(query=>sql(live,query));
 const denied=(statement,message)=>assert.throws(()=>sql(live,statement),error=>{
  assert.equal(error.status,3,'psql must reach a SQL error rather than fail to connect');
  assert.match(String(error.stderr),/ERROR:\s+42501:/,'only insufficient_privilege proves role denial');
  return true;
 },message);
 for(const table of ['workflow_evidence_blobs','workflow_evidence_documents','workflow_evidence_members']){
  denied(`SET ROLE weaveos_backup_probe; DELETE FROM applications.${table};`,'backup cannot delete evidence');
  denied(`SET ROLE weaveos_backup_probe; UPDATE applications.${table} SET app_id=app_id;`,'backup cannot mutate evidence');
 }
 denied("SET ROLE weaveos_backup_probe; UPDATE applications.workflow_commands SET execution_payload=NULL;",'backup must not rewrite accepted execution inputs');
 denied("SET ROLE weaveos_backup_probe; UPDATE applications.workflow_dispatch SET attempts=0;",'backup must not alter retry or lease metadata');
 assert.equal(sql(restored,"SELECT nextval('public.goose_db_version_id_seq');").trim(),'3','restored sequence must continue after original ledger rows');
 assert.equal(sql(restored,"SELECT nextval('applications.record_command_fence_epoch_seq');").trim(),'42','restored fence epoch must continue after original allocations');
 denied("SET ROLE weaveos_backup_probe; SELECT nextval('applications.record_command_fence_epoch_seq');",'backup role must not allocate fence epochs');
 denied("SET ROLE weaveos_backup_probe; SELECT setval('applications.record_command_fence_epoch_seq',1,true);",'backup role must not reset fence epochs');
 const presetColumns="id,owner_id,view_key,name,slot,filter_json,hidden_column_ids,schema_version,version,created_at,updated_at";
 assert.equal(sql(restored,'SELECT '+presetColumns+' FROM personnel.table_presets;'),sql(live,'SELECT '+presetColumns+' FROM personnel.table_presets;'),'restricted backup must preserve complete named configuration, Unicode, AST, ownership, slot, CAS and timestamps');
 assert.equal(sql(restored,"SELECT name,slot,version FROM personnel.table_presets WHERE id='88888888-8888-4888-8888-888888888888';").trim(),'持久方案😀|20|7','synthetic independent preset survives encrypted backup/restore');
 denied("SET ROLE weaveos_backup_probe; SELECT nextval('public.goose_db_version_id_seq');",'read-only backup must not allocate sequence values');
 assert.deepEqual(guardedQueries.map(query=>sql(live,query)),guardedBefore,'denied backup operations must preserve exact data, evidence and sequence state');
});
