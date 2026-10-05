import os, pathlib, hashlib, json, subprocess, time, platform, shutil
root=pathlib.Path('/workspace/WeaveOS-worktrees/V030-031'); out=pathlib.Path('/tmp/v030-031-red')
files=['docs/evidence/V030-031/root-vector-generator.py','services/bff/internal/flowcommands/execution_codec.go','services/bff/internal/flowcommands/root_execution_codec_test.go','services/bff/internal/flowcommands/testdata/root-execution-vectors.json']
hashes={}
for name in files:
 data=(root/name).read_bytes(); hashes[name]=hashlib.sha256(data).hexdigest(); dest=out/'original'/name; dest.parent.mkdir(parents=True,exist_ok=True); dest.write_bytes(data)
(out/'source-hashes-before.json').write_text(json.dumps(hashes,indent=2)+'\n')
vectors=json.loads((root/files[-1]).read_text())
assert len(vectors)==8
for name,v in vectors.items():
 data=bytes.fromhex(v['hex']); assert len(data)==v['size'] and hashlib.sha256(data).hexdigest()==v['sha256'],name
(out/'vector-verification.json').write_text(json.dumps({'count':len(vectors),'entries':{k:v['size'] for k,v in vectors.items()},'all_size_and_sha256_match':True},indent=2)+'\n')
env=os.environ.copy(); env.update(GOROOT='/workspace/.weaveos-tools/go',GOPATH='/workspace/.weaveos-tools/gopath',GOMODCACHE='/workspace/.weaveos-tools/go-mod',GOCACHE='/workspace/.weaveos-tools/go-cache',GOPROXY='off')
go='/workspace/.weaveos-tools/go/bin/go'; cwd=root/'services/bff'; runs=[]
for label,args in [('compile',[go,'test','-c','-o',str(out/'flowcommands.test'),'./internal/flowcommands']),('list',[go,'test','-list','^TestRootExecution','./internal/flowcommands']),('red',[go,'test','-json','-count=1','-run','^TestRootExecution','./internal/flowcommands'])]:
 start=time.time(); p=subprocess.run(args,cwd=cwd,env=env,capture_output=True); elapsed=time.time()-start
 (out/(label+'.stdout')).write_bytes(p.stdout); (out/(label+'.stderr')).write_bytes(p.stderr)
 entry={'label':label,'command':args,'cwd':str(cwd),'exit_code':p.returncode,'elapsed_seconds':elapsed,'stdout_sha256':hashlib.sha256(p.stdout).hexdigest(),'stderr_sha256':hashlib.sha256(p.stderr).hexdigest()};runs.append(entry)
 print(json.dumps(entry),flush=True)
 if label=='compile' and p.returncode!=0: break
metadata={'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'toolchain':subprocess.check_output([go,'version'],env=env,text=True).strip(),'uname':platform.uname()._asdict(),'cpu':subprocess.check_output(['lscpu'],text=True),'runs':runs,'environment':{k:env[k] for k in ['GOROOT','GOPATH','GOMODCACHE','GOCACHE','GOPROXY']}}
(out/'run-metadata.json').write_text(json.dumps(metadata,indent=2)+'\n')
after={name:hashlib.sha256((root/name).read_bytes()).hexdigest() for name in files};assert hashes==after
(out/'source-hashes-after.json').write_text(json.dumps(after,indent=2)+'\n')
if any(x['label']=='red' for x in runs):
 events=[json.loads(x) for x in (out/'red.stdout').read_text().splitlines()]; results=[e for e in events if e.get('Action') in ['pass','fail','skip'] and e.get('Test','').startswith('TestRootExecution') and '/' not in e['Test']]
 names=[x for x in (out/'list.stdout').read_text().splitlines() if x.startswith('TestRootExecution')]
 assert len(names)==15 and len(results)==15 and set(names)=={r['Test'] for r in results}
 summary={'listed_names':names,'top_level_count':len(results),'pass':sum(r['Action']=='pass' for r in results),'fail':sum(r['Action']=='fail' for r in results),'skip':sum(r['Action']=='skip' for r in results),'results':[{k:r[k] for k in ['Test','Action','Elapsed']} for r in results]}
 (out/'test-summary.json').write_text(json.dumps(summary,indent=2)+'\n'); print(json.dumps(summary),flush=True)
