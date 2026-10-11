"""Supplemental real Goose CLI acceptance; never accept a production DSN."""
import json,os,pathlib,subprocess,sys,urllib.parse
base=urllib.parse.urlsplit(os.environ['WEAVEOS_TEST_DATABASE_URL'])
assert base.scheme=='postgres' and base.hostname=='127.0.0.1' and base.username=='weaveos_test' and base.path=='/weaveos_ci_test'
r=pathlib.Path(__file__).resolve().parents[2];out=pathlib.Path(sys.argv[1]);out.mkdir(parents=True,exist_ok=True)
def run(args):return subprocess.run(args,text=True,stdout=subprocess.PIPE,stderr=subprocess.STDOUT)
def sql(url,q):
 x=run(['psql',url,'-XAt','-v','ON_ERROR_STOP=1','-c',q]);assert x.returncode==0,x.stdout;return x.stdout.strip()
def goose(url,*args):return run(['goose','-dir',str(r/'db/migrations'),'postgres',url,*args])
def version(url):return sql(url,'SELECT version_id FROM public.goose_db_version ORDER BY id DESC LIMIT 1')
constraints="SELECT string_agg(conname||':'||pg_get_constraintdef(oid),'|' ORDER BY conname) FROM pg_constraint WHERE conrelid='applications.operations'::regclass"
u='00000000-0000-4000-8000-000000000001';a='00000000-0000-4000-8000-000000000002';o='00000000-0000-4000-8000-000000000003';resource='00000000-0000-4000-8000-000000000004'
results=[]
for kind in ['empty','application','directory','table','form']:
 name='weaveos_v074_cli_'+kind;url=urllib.parse.urlunsplit(base._replace(path='/'+name));admin=urllib.parse.urlunsplit(base._replace(path='/postgres'))
 sql(admin,'CREATE DATABASE '+name)
 try:
  x=goose(url,'up-to','35');assert x.returncode==0,x.stdout;before=sql(url,constraints)
  x=goose(url,'up-to','36');(out/(kind+'-up.txt')).write_text(x.stdout);assert x.returncode==0 and version(url)=='36',x.stdout
  if kind!='empty':
   id=a if kind=='application' else resource;location='/api/v1/applications/'+a+('' if kind=='application' else '/'+{'directory':'directories','table':'tables','form':'forms'}[kind]+'/'+id)
   receipt=json.dumps(dict(operationId=o,appId=a,resourceKind=kind,id=id,structureVersion=1,policyRevision=1,status='copied'))
   sql(url,f"INSERT INTO auth.users(id,account) VALUES('{u}','copy-cli');INSERT INTO applications.apps(id,name,owner_user_id) VALUES('{a}','CLI','{u}');INSERT INTO applications.operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint,result_json,http_status,location) VALUES('{u}','{o}','{a}','structure.copy',decode(repeat('ab',32),'hex'),'{receipt}',201,'{location}');")
  retained=sql(url,"SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY operation_id),'[]') FROM applications.operations o")
  x=goose(url,'down');(out/(kind+'-down.txt')).write_text(x.stdout)
  if kind=='empty':
   assert x.returncode==0 and version(url)=='35',x.stdout
   assert sql(url,constraints)==before,'old CHECK changed'
   x=goose(url,'up-to','36');assert x.returncode==0 and version(url)=='36',x.stdout
  else:
   assert x.returncode!=0 and '55000' in x.stdout,x.stdout
   assert version(url)=='36' and sql(url,"SELECT COALESCE(jsonb_agg(to_jsonb(o) ORDER BY operation_id),'[]') FROM applications.operations o")==retained,'failed Down changed ledger/data'
  results.append(dict(kind=kind,passed=True))
 except Exception as e:results.append(dict(kind=kind,passed=False,error=str(e)))
 finally:sql(admin,'DROP DATABASE '+name)
(out/'results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results));sys.exit(0 if all(v['passed'] for v in results) else 1)
