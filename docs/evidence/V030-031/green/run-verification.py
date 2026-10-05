import os,json,subprocess,time,pathlib,sys,platform
root=pathlib.Path('/workspace/WeaveOS-worktrees/V030-031'); out=root/'docs/evidence/V030-031/green'; out.mkdir(exist_ok=True)
env=os.environ.copy();env.update(GOROOT='/workspace/.weaveos-tools/go',GOPATH='/workspace/.weaveos-tools/gopath',GOMODCACHE='/workspace/.weaveos-tools/go-mod',GOCACHE='/workspace/.weaveos-tools/go-cache',GOPROXY='off',WEAVEOS_TEST_DATABASE_URL='postgres://weaveos_test:isolated-codec-only@127.0.0.1:55431/weaveos_codec_test?sslmode=disable',WEAVEOS_TEST_REDIS_URL='redis://127.0.0.1:56331/15')
go='/workspace/.weaveos-tools/go/bin/go'; bff=root/'services/bff'
commands={
'formal-ledger-fence':([go,'test','-race','-count=1','-json','-run','^TestRoot(FormalLedger|CommandFence)','./internal/apprecordservice'],bff),
'new-codecs':([go,'test','-race','-count=1','-json','-run','^TestRootExecution','./internal/flowcommands'],bff),
'flowcommands':([go,'test','-race','-count=1','-json','./internal/flowcommands'],bff),
'vet':([go,'vet','./...'],bff),
'build':([go,'build','-o','/tmp/v030-031-bff','./cmd/bff'],bff),
'governance':(['bash','-c','node --test tests/governance/*.test.mjs tests/foundation/*.test.mjs && node scripts/verify-repo.mjs && node scripts/check-tasks.mjs'],root),
'benchmarks':([go,'test','-run','^$','-bench','^BenchmarkRootExecution','-benchmem','-benchtime=1s','-count=5','./internal/flowcommands'],bff)}
for label in sys.argv[1:]:
 args,cwd=commands[label];start=time.time();p=subprocess.run(args,cwd=cwd,env=env,capture_output=True);elapsed=time.time()-start;(out/(label+'.stdout')).write_bytes(p.stdout);(out/(label+'.stderr')).write_bytes(p.stderr)
 result={'command':args,'cwd':str(cwd),'exit_code':p.returncode,'elapsed_seconds':elapsed,'utc_started':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime(start))}
 if '-json' in args:
  events=[json.loads(x) for x in p.stdout.decode().splitlines()];tests=[e for e in events if e.get('Action') in ['pass','fail','skip'] and e.get('Test') and '/' not in e['Test']];result['top_level_tests']={status:sum(e['Action']==status for e in tests) for status in ['pass','fail','skip']}
 (out/(label+'.json')).write_text(json.dumps(result,indent=2)+'\n');print(label,json.dumps(result),flush=True)
 if p.returncode:print(p.stderr.decode(),p.stdout.decode()[-3000:],flush=True);sys.exit(p.returncode)
