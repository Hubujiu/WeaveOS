"""Root normative byte construction, independent of Go and Java implementations."""
import hashlib,json,struct
from pathlib import Path
def uid(n):return f'{n:08x}-0000-4000-8000-{n:012x}'
def txt(s):
 b=s.encode('utf-8');return struct.pack('>I',len(b))+b
evidence=hashlib.sha256(b'root-audit-evidence').digest()
def payload(start,allow,rosters,routes):
 out=b'WVFPAY\0\1'+evidence+bytes([start])
 if start:
  out+=bytes([allow])+struct.pack('>I',len(rosters))
  for n,actors in sorted(rosters.items()):
   out+=txt(n)+struct.pack('>I',len(actors))+b''.join(txt(a) for a in sorted(actors))
 out+=struct.pack('>I',len(routes))
 for n,v in sorted(routes.items()):out+=txt(n)+bytes([v])
 return out
def result(state,schema,record,tasks=(),reason=''):
 out=b'WVFRSL\0\1'+txt(uid(107))+txt('' if state=='unchanged' else 'engine-process-7')+txt(state)+txt(reason)
 out+=struct.pack('>QQI',schema,record,len(tasks))
 for t in sorted(tasks):
  out+=b''.join(txt(s) for s in t[:4])+struct.pack('>Q',t[4])
 return out
vectors={}
def add(name,raw):vectors[name]={'hex':raw.hex(),'sha256':hashlib.sha256(raw).hexdigest(),'size':len(raw)}
add('payload_start',payload(True,True,{uid(2):[uid(8),uid(9)],uid(3):[uid(10),uid(11),uid(12)]},{}))
add('payload_agree',payload(False,False,{}, {uid(3):False}))
add('payload_withdraw',payload(False,False,{},{}))
tasks=[(uid(201),uid(2),uid(8),'engine-task-8',1),(uid(202),uid(2),uid(9),'engine-task-9',1)]
add('result_active',result('active',1,1,tasks))
for state in ['completed','rejected','withdrawn']:add('result_'+state,result(state,2,7))
add('result_no_effect',result('unchanged',1,1,reason='cancelled'))
Path(__file__).with_name('root-execution-vectors.json').write_text(json.dumps(vectors,indent=2)+'\n')
