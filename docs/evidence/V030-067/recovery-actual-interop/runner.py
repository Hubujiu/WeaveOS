import os,subprocess,pathlib,select,time,sys
root=pathlib.Path('/workspace/scratch/75c2273f6a80');repo=root/'WeaveOS-worktrees/V030-067';out=repo/'docs/evidence/V030-067/recovery-actual-interop'
out.mkdir(exist_ok=False);(out/'runner.py').write_bytes(pathlib.Path(__file__).read_bytes());env=os.environ.copy();env['GOPATH']=str(root/'restored-tools/gopath');env['GOCACHE']=str(root/'restored-tools/go-cache');env['GOTOOLCHAIN']='local'
java=(root/'restored-tools/jdk17-path.txt').read_text().strip()+'/bin/java';classpath='target/classes:target/test-classes:'+(root/'restored-tools/v067-classpath').read_text().strip()
(out/'source-head.txt').write_bytes(subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo));(out/'source.patch').write_bytes(subprocess.check_output(['git','diff','--binary','HEAD','--','services/'],cwd=repo));(out/'started.txt').write_text(time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()))
p=subprocess.Popen([java,'-cp',classpath,'org.weaveos.workflow.RootFlowDeletionInteropFixtureMain'],cwd=repo/'services/workflow-engine',env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,bufsize=1)
lines=[];port=None
try:
 deadline=time.monotonic()+60
 while time.monotonic()<deadline:
  if p.poll() is not None:break
  if select.select([p.stdout],[],[],1)[0]:
   line=p.stdout.readline();lines.append(line)
   if line.startswith('V067_READY='):port=int(line.split('=')[1]);break
 if not port:raise RuntimeError('actual fixture failed readiness: '+''.join(lines))
 env['WEAVEOS_V067_RPC_TARGET']='127.0.0.1:'+str(port)
 cmd=[str(root/'restored-tools/go/bin/go'),'test','-race','-json','-count=1','-tags','workflow_deletion_integration','-run','^TestRootGoJavaActualDeletionRuntimeInterop$','./internal/workflowrpc']
 (out/'command.txt').write_text(' '.join(cmd))
 with (out/'go.jsonl').open('w') as log:r=subprocess.run(cmd,cwd=repo/'services/bff',env=env,stdout=log,stderr=subprocess.STDOUT,timeout=120)
 (out/'exit.txt').write_text(str(r.returncode));print((out/'go.jsonl').read_text());code=r.returncode
finally:
 try:tail,_=p.communicate('\n',timeout=30)
 except subprocess.TimeoutExpired:p.kill();tail,_=p.communicate();raise
 (out/'java.txt').write_text(''.join(lines)+tail);(out/'java-exit.txt').write_text(str(p.returncode));(out/'finished.txt').write_text(time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()))
if p.returncode:sys.exit(p.returncode)
sys.exit(code)
