"""Real isolated Goose upgrade/down acceptance; never use a production DSN."""
import json,os,pathlib,subprocess,sys,urllib.parse
base=urllib.parse.urlsplit(os.environ['WEAVEOS_TEST_DATABASE_URL'])
assert base.scheme=='postgres' and base.hostname=='127.0.0.1' and base.username=='weaveos_test' and base.path=='/weaveos_ci_test'
r=pathlib.Path(__file__).resolve().parents[2];out=pathlib.Path(sys.argv[1]);out.mkdir(parents=True,exist_ok=True)
def run(args):return subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
def sql(url,q):
 x=run(['psql',url,'-XAt','-v','ON_ERROR_STOP=1','-c',q]);assert x.returncode==0,x.stdout;return x.stdout.strip()
def goose(url,*args):return run(['goose','-dir',str(r/'db/migrations'),'postgres',url,*args])
def version(url):return sql(url,'SELECT version_id FROM public.goose_db_version ORDER BY id DESC LIMIT 1')
u='00000000-0000-4000-8000-000000000001';a='00000000-0000-4000-8000-000000000002';d='00000000-0000-4000-8000-000000000003';t='00000000-0000-4000-8000-000000000004';v='00000000-0000-4000-8000-000000000005';op='00000000-0000-4000-8000-000000000006'
seed=f"INSERT INTO auth.users(id,account) VALUES('{u}','structure-cli');INSERT INTO applications.apps(id,name,owner_user_id) VALUES('{a}','CLI','{u}');SELECT applications.register_catalog_entry('{a}');"
constraint="SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid='applications.operations'::regclass AND conname='ck_operation_kind'"
functions=['apply_schema_change(uuid,uuid,uuid,text,jsonb,jsonb)','apply_option_mapping(uuid,uuid,uuid,uuid,text,jsonb,jsonb)','apply_record_change(uuid,uuid,uuid,uuid,uuid,text,bigint,bigint,jsonb)','acquire_record_command_fence(uuid,uuid,uuid,uuid,uuid,bigint,bigint)','change_record_lifecycle(uuid,uuid,uuid,uuid,uuid,uuid,bigint,bigint,boolean)']
def snapshot(url):
 return {name:sql(url,f"SELECT COALESCE(jsonb_agg(j ORDER BY j::text),'[]') FROM (SELECT to_jsonb(t) j FROM applications.{name} t) q") for name in ['apps','directories','logical_tables','form_views','operations','structure_deletions']}
results=[]
for case in ['empty','application','directory','table','form','operation-only','marker-only']:
 try:
  name='weaveos_v073_cli_'+case.replace('-','_');url=urllib.parse.urlunsplit(base._replace(path='/'+name))
  sql(urllib.parse.urlunsplit(base._replace(path='/postgres')),'CREATE DATABASE '+name)
  x=goose(url,'up-to','34');assert x.returncode==0,x.stdout
  before={f:sql(url,f"SELECT pg_get_functiondef('applications.{f}'::regprocedure)") for f in functions};old=sql(url,constraint)
  x=goose(url,'up-to','35');(out/(case+'-up.txt')).write_text(x.stdout);assert x.returncode==0,x.stdout
  assert version(url)=='35'
  x=run(['psql',url,'-v','ON_ERROR_STOP=1','-f',str(r/'infra/runtime/roles.sql')]);assert x.returncode==0,x.stdout
  if case!='empty':
   sql(url,seed)
   target=a;resource=1
   if case=='directory':target=d;resource=0;sql(url,f"INSERT INTO applications.directories(id,app_id,name,position) VALUES('{d}','{a}','directory',0)")
   if case in ['table','form']:
    sql(url,f"INSERT INTO applications.logical_tables(id,app_id,name,position) VALUES('{t}','{a}','table',0)");target=t;resource=0
   if case=='form':sql(url,f"INSERT INTO applications.form_views(id,app_id,table_id,name,position) VALUES('{v}','{a}','{t}','view',0)");target=v
   if case in ['application','directory','table','form']:sql(url,f"SET ROLE auth_app;SELECT applications.delete_structure_resource('{a}','{case}','{target}','{u}','{op}',0,{resource})")
   elif case=='marker-only':sql(url,f"UPDATE applications.apps SET deleted_at=now() WHERE id='{a}'")
   else:
    receipt=json.dumps(dict(operationId=op,appId=a,resourceKind='application',id=a,structureVersion=1,deleted=True))
    sql(url,f"INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES('{u}','{op}','{a}','application.delete',decode(repeat('ab',32),'hex'),'{receipt}',200,'')")
  retained=snapshot(url);x=goose(url,'down');(out/(case+'-down.txt')).write_text(x.stdout)
  if case=='empty':
   assert x.returncode==0,x.stdout;assert version(url)=='34'
   assert sql(url,constraint)==old
   for f in functions:
    assert sql(url,f"SELECT pg_get_functiondef('applications.{f}'::regprocedure)")==before[f],f
    assert sql(url,f"SELECT has_function_privilege('auth_app','applications.{f}','EXECUTE')")=='t',f
   assert sql(url,"SELECT to_regclass('applications.structure_deletions') IS NULL")=='t'
   assert sql(url,"SELECT count(*) FROM pg_attribute WHERE attrelid IN ('applications.apps'::regclass,'applications.directories'::regclass,'applications.logical_tables'::regclass,'applications.form_views'::regclass) AND attname='deleted_at' AND NOT attisdropped")=='0'
   x=goose(url,'up-to','35');assert x.returncode==0 and version(url)=='35',x.stdout
  else:
   assert x.returncode!=0 and '55000' in x.stdout,x.stdout
   assert version(url)=='35' and snapshot(url)==retained,'refused Down changed durable identity'
  results.append(dict(case=case,passed=True))
 except Exception as e:results.append(dict(case=case,passed=False,error=str(e)))
(out/'results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,indent=2));sys.exit(0 if all(x['passed'] for x in results) else 1)
