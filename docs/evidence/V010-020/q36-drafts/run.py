#!/usr/bin/env python3
"""Dedicated disposable PG18.6/Redis. No production URLs or migration Down.
Usage: run.py setup | test <go arguments> | stop
Private generated connection data stays in /tmp/weaveos-q36-drafts.
"""
import json, os, pathlib, secrets, subprocess, sys, time
root=pathlib.Path(__file__).resolve().parents[4]
private=pathlib.Path('/tmp/weaveos-q36-drafts')
pg='weaveos-q36-drafts-pg'; redis='weaveos-q36-drafts-redis'
tools=pathlib.Path('/workspace/.weaveos-tools')
def run(args, **kw):
 result=subprocess.run(args,**kw)
 if result.returncode: raise RuntimeError('command failed with exit '+str(result.returncode))
 return result
if sys.argv[1]=='setup':
 private.mkdir(mode=0o700,exist_ok=False)
 password=secrets.token_urlsafe(32)
 envfile=private/'postgres.env';envfile.write_text('POSTGRES_USER=drafts_test\nPOSTGRES_DB=weaveos_drafts_test\nPOSTGRES_PASSWORD='+password+'\n');envfile.chmod(0o600)
 run(['docker','run','-d','--name',pg,'--label','weaveos.task=q36-drafts','--env-file',str(envfile),'-p','127.0.0.1:25436:5432','postgres:18.6'],stdout=subprocess.DEVNULL)
 run(['docker','run','-d','--name',redis,'--label','weaveos.task=q36-drafts','-p','127.0.0.1:26436:6379','sha256:5165c4f6f63ee38719866dd2f2eb38f92f20a9e08beb3d3968f3b66de7b8c301'],stdout=subprocess.DEVNULL)
 for i in range(50):
  if subprocess.run(['docker','exec',pg,'pg_isready','-h','127.0.0.1','-U','drafts_test','-d','weaveos_drafts_test'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode==0: break
  time.sleep(.2)
 cfg={'WEAVEOS_TEST_DATABASE_URL':'postgres://drafts_test:'+password+'@127.0.0.1:25436/weaveos_drafts_test?sslmode=disable','WEAVEOS_TEST_REDIS_URL':'redis://127.0.0.1:26436/15'}
 cfg['WEAVEOS_TEST_ARCHIVE_DATABASE_URL']=cfg['WEAVEOS_TEST_DATABASE_URL'].replace('/weaveos_drafts_test?', '/weaveos_drafts_archive_test?')
 (private/'env.json').write_text(json.dumps(cfg));(private/'env.json').chmod(0o600)
 env=os.environ.copy();env.update(cfg)
 run([str(tools/'gopath/bin/goose'),'-dir',str(root/'db/migrations'),'postgres',cfg['WEAVEOS_TEST_DATABASE_URL'],'up'],env=env)
 with (root/'infra/runtime/roles.sql').open() as f: run(['docker','exec','-i',pg,'psql','-v','ON_ERROR_STOP=1','-U','drafts_test','-d','weaveos_drafts_test'],stdin=f,stdout=subprocess.DEVNULL)
 run(['docker','exec',pg,'createdb','-U','drafts_test','weaveos_drafts_archive_test'])
 run([str(tools/'gopath/bin/goose'),'-dir',str(root/'db/archive-migrations'),'postgres',cfg['WEAVEOS_TEST_ARCHIVE_DATABASE_URL'],'up'],env=env)
 # Test-only exact draft capabilities. Production roles.sql belongs to integrator.
 run(['docker','exec',pg,'psql','-v','ON_ERROR_STOP=1','-U','drafts_test','-d','weaveos_drafts_test','-c','GRANT SELECT, INSERT, DELETE ON personnel.drafts TO auth_app; GRANT UPDATE (payload_json,draft_version,updated_at) ON personnel.drafts TO auth_app;'],stdout=subprocess.DEVNULL)
 run(['docker','exec',pg,'psql','-U','drafts_test','-d','weaveos_drafts_test','-Atc','SELECT version()'])
 run(['docker','exec',redis,'redis-server','--version'])
elif sys.argv[1]=='test':
 env=os.environ.copy();env.update(json.loads((private/'env.json').read_text()));env.update(PATH=str(tools/'go/bin')+':'+env['PATH'],GOMODCACHE=str(tools/'go-mod'),GOCACHE=str(tools/'go-cache'),GOPATH=str(tools/'gopath'))
 sys.exit(subprocess.run(['go']+sys.argv[2:],cwd=root/'services/bff',env=env).returncode)
elif sys.argv[1]=='stop':
 for name in [pg,redis]:
  label=subprocess.check_output(['docker','inspect','--format','{{index .Config.Labels "weaveos.task"}}',name],text=True).strip()
  if label!='q36-drafts': raise RuntimeError('container ownership mismatch')
  run(['docker','rm','-f','-v',name],stdout=subprocess.DEVNULL)
 for f in private.iterdir(): f.unlink()
 private.rmdir()
else: raise RuntimeError('unknown action')
