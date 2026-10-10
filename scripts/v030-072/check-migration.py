"""Independent real Goose acceptance on newly created loopback test databases only."""
import json, os, pathlib, subprocess, sys, urllib.parse
base=urllib.parse.urlsplit(os.environ['WEAVEOS_TEST_DATABASE_URL'])
assert base.scheme=='postgres' and base.hostname=='127.0.0.1' and base.username=='weaveos_test' and base.path=='/weaveos_ci_test'
repo=pathlib.Path(__file__).resolve().parents[2]
out=pathlib.Path(sys.argv[1]).resolve();out.mkdir(parents=True,exist_ok=True)
def run(args):
 return subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
def sql(url,q):
 r=run(['psql',url,'-XAt','-v','ON_ERROR_STOP=1','-c',q])
 if r.returncode: raise AssertionError(r.stdout)
 return r.stdout.strip()
def goose(url,*args):
 return run(['goose','-dir',str(repo/'db/migrations'),'postgres',url,*args])
def version(url):
 return sql(url,'SELECT version_id FROM public.goose_db_version ORDER BY id DESC LIMIT 1')
a='00000000-0000-4000-8000-000000000001';u='00000000-0000-4000-8000-000000000002';t='00000000-0000-4000-8000-000000000003';v='00000000-0000-4000-8000-000000000004';p='00000000-0000-4000-8000-000000000005';op='00000000-0000-4000-8000-000000000006'
seed=f"INSERT INTO auth.users(id,account) VALUES('{u}','record-lifecycle-cli'); INSERT INTO applications.apps(id,name,owner_user_id) VALUES('{a}','CLI','{u}'); INSERT INTO applications.logical_tables(id,app_id,name,position) VALUES('{t}','{a}','CLI',0); INSERT INTO applications.form_views(id,app_id,table_id,name,position) VALUES('{v}','{a}','{t}','CLI',0);"
constraint="SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='applications.operations'::regclass AND conname='ck_operation_kind'"
results=[]
for case in ['empty','state','delete','restore']:
 name='weaveos_v072_cli_'+case
 url=urllib.parse.urlunsplit(base._replace(path='/'+name))
 try:
  sql(urllib.parse.urlunsplit(base._replace(path='/postgres')),'CREATE DATABASE '+name)
  before=goose(url,'up-to','33');assert before.returncode==0,before.stdout
  prior=sql(url,constraint)
  function_before=sql(url,"SELECT pg_get_functiondef('applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb)'::regprocedure)")
  up=goose(url,'up');(out/(case+'-up.txt')).write_text(up.stdout);assert up.returncode==0,up.stdout
  assert version(url)=='34' and sql(url,"SELECT to_regclass('applications.record_lifecycle') IS NOT NULL")=='t'
  if case!='empty':
   sql(url,seed)
   if case=='state':
    sql(url,f"INSERT INTO applications.record_lifecycle(app_id,table_id,record_id,deleted,record_version,changed_by,changed_at) VALUES('{a}','{t}','{p}',true,2,'{u}',now());")
   else:
    receipt=json.dumps(dict(operationId=op,id=p,recordVersion=2,schemaVersion=1,deleted=case=='delete'),separators=(',',':'))
    sql(url,f"INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES('{u}','{op}','{a}','record.{case}',decode(repeat('ab',32),'hex'),'{receipt}',200,'');")
  snapshot=sql(url,"SELECT row_to_json(x) FROM (SELECT (SELECT jsonb_agg(to_jsonb(p)) FROM applications.record_lifecycle p) AS presets,(SELECT jsonb_agg(to_jsonb(o)) FROM applications.operations o) AS operations) x")
  down=goose(url,'down');(out/(case+'-down.txt')).write_text(down.stdout)
  if case=='empty':
   assert down.returncode==0,down.stdout
   assert version(url)=='33' and sql(url,"SELECT to_regclass('applications.record_lifecycle') IS NULL")=='t','empty Down left private relation or wrong version'
   assert sql(url,constraint)==prior,'old finite operation kind guard changed'
   assert sql(url,"SELECT pg_get_functiondef('applications.apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb)'::regprocedure)")==function_before,'old DML body changed'
   assert sql(url,"SELECT to_regclass('applications.record_lifecycle_events') IS NULL")=='t'
   up=goose(url,'up');(out/(case+'-reup.txt')).write_text(up.stdout);assert up.returncode==0 and version(url)=='34',up.stdout
  else:
   assert down.returncode!=0 and '55000' in down.stdout,'nonempty Down did not refuse with 55000'
   assert version(url)=='34','refused Down changed Goose version'
   assert sql(url,"SELECT row_to_json(x) FROM (SELECT (SELECT jsonb_agg(to_jsonb(p)) FROM applications.record_lifecycle p) AS presets,(SELECT jsonb_agg(to_jsonb(o)) FROM applications.operations o) AS operations) x")==snapshot,'refused Down changed durable rows'
  results.append(dict(case=case,passed=True))
 except Exception as e:
  results.append(dict(case=case,passed=False,error=str(e)))
(out/'results.json').write_text(json.dumps(results,indent=2))
print(json.dumps(results,indent=2));sys.exit(0 if all(x['passed'] for x in results) else 1)
