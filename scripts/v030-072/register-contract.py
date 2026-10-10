"""Register only the frozen V072 lifecycle surface; preserve all older schemas."""
import copy,json,pathlib
r=pathlib.Path(__file__).resolve().parents[2]
p=r/'contracts/openapi/openapi.json';a=json.loads(p.read_text());s=a['components']['schemas']
ref=lambda name:{'$ref':'#/components/schemas/'+name}
uuid=copy.deepcopy(s['RecordMutationResult']['properties']['id']);uuid['not']={'const':'00000000-0000-0000-0000-000000000000'}
version=lambda minimum:{'type':'integer','minimum':minimum,'maximum':9007199254740991}
def obj(props):return {'type':'object','additionalProperties':False,'required':list(props),'properties':props}
s['RecordLifecycleRequest']=obj(dict(operationId=uuid,expectedSchemaVersion=version(0),expectedRecordVersion=version(1)))
s['RecordLifecycleResult']=obj(dict(operationId=uuid,id=uuid,recordVersion=version(2),schemaVersion=version(1),deleted={'type':'boolean'}))
s['RecordLifecycleState']=obj(dict(id=uuid,recordVersion=version(1),schemaVersion=version(1),deleted={'type':'boolean'}))
for name in ['RecordLifecycleResult','RecordLifecycleState']:
 env=copy.deepcopy(s['RecordMutationResultEnvelope']);env['allOf'][1]['properties']['data']=ref(name);s[name+'Envelope']=env
base='/api/v1/applications/{appId}/forms/{viewId}/records/{recordId}'
for action in ['deletion','restoration']:
 op=copy.deepcopy(a['paths'][base]['patch'])
 op['operationId']='deleteRecordRecoverably' if action=='deletion' else 'restoreRecord'
 op['summary']='Change retained record lifecycle: '+action
 op['description']='V072: explicit recoverable lifecycle command, not physical record removal. Current application owner or Bootstrap only; ordinary edit grants do not authorize it. Closed 4096-byte JSON, live Session/CSRF/expected actor, schema and record CAS. Starting/active workflows and unknown pending commands block deletion. Same operation retries recover the minimal confirmed result; no expiry or purge, no automatic workflow trigger. Confirmed commit survives subsequent renewal failure with cookies cleared; unknown commit returns 503 with original operationId.'
 op['requestBody']={'required':True,'content':{'application/json':{'schema':ref('RecordLifecycleRequest')}}}
 op['responses']['200']['content']['application/json']['schema']=ref('RecordLifecycleResultEnvelope')
 a['paths'][base+'/'+action]={'post':op}
op=copy.deepcopy(a['paths'][base]['get']);op['operationId']='getRecordLifecycle';op['summary']='Read minimal retained record lifecycle'
op['description']='Current owner or Bootstrap, one read-only snapshot. No business values or historical actor disclosure. Retained records are included for explicit restoration.'
op['responses']['200']['content']['application/json']['schema']=ref('RecordLifecycleStateEnvelope')
head=copy.deepcopy(op);head['operationId']='headRecordLifecycle'
for response in head['responses'].values():response.pop('content',None)
a['paths'][base+'/lifecycle']={'get':op,'head':head}
items=s['ApplicationOperation']['properties']['result']['oneOf'];item=ref('RecordLifecycleResult')
if item not in items:items.append(item)
error='APPLICATION_RECORD_LIFECYCLE_CONFLICT';codes=s['RecordErrorEnvelope']['allOf'][1]['properties']['code']['enum']
if error not in codes:codes.append(error)
p.write_text(json.dumps(a,ensure_ascii=False,indent=2)+'\n')
p=r/'contracts/errors/codes.json';c=json.loads(p.read_text());c[error]={'httpStatus':409,'grpcStatus':'FAILED_PRECONDITION','public':True};p.write_text(json.dumps(c,ensure_ascii=False,indent=2)+'\n')
