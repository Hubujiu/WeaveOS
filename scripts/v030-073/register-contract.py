"""Register only the pre-frozen noncascade structure deletion contract."""
import copy,json,pathlib
r=pathlib.Path(__file__).resolve().parents[2];p=r/'contracts/openapi/openapi.json';api=json.loads(p.read_text());s=api['components']['schemas']
ref=lambda n:{'$ref':'#/components/schemas/'+n}
obj=lambda props:{'type':'object','additionalProperties':False,'required':list(props),'properties':props}
version=lambda minimum:{'type':'integer','minimum':minimum,'maximum':9007199254740991}
uuid=copy.deepcopy(s['RecordLifecycleRequest']['properties']['operationId'])
s['StructureDeletionRequest']=obj(dict(operationId=uuid,expectedStructureVersion=version(0),expectedResourceVersion=version(0)))
for name,rule in [('ApplicationDeletionRequest',version(1)),('DirectoryDeletionRequest',{'type':'integer','const':0})]:
 s[name]=copy.deepcopy(s['StructureDeletionRequest']);s[name]['properties']['expectedResourceVersion']=rule
s['StructureDeletionResult']=obj(dict(operationId=uuid,appId=uuid,resourceKind={'type':'string','enum':['application','directory','table','form']},id=uuid,structureVersion=version(1),deleted={'type':'boolean','const':True}))
s['StructureDeletionResultEnvelope']=copy.deepcopy(s['RecordMutationResultEnvelope']);s['StructureDeletionResultEnvelope']['allOf'][1]['properties']['data']=ref('StructureDeletionResult')
deps=['directories','tables','forms','permission_groups','workflows','drafts','pending_commands','grants','records']
s['StructureDeletionDependencies']=obj(dict(dependencies={'type':'array','minItems':1,'maxItems':8,'uniqueItems':True,'items':{'type':'string','enum':deps}}))
choices=[]
for item in s['DefinitionErrorEnvelope']['oneOf']:
 props=item.get('allOf',[{},{}])[1].get('properties',{}) if 'allOf' in item else {}
 if props.get('code',{}).get('const') in ['COMMON_VALIDATION_FAILED','APPLICATION_STRUCTURE_CONFLICT','APPLICATION_SCHEMA_CONFLICT','APPLICATION_VIEW_CONFLICT','APPLICATION_OPERATION_UNCONFIRMED'] or '$ref' in item:choices.append(copy.deepcopy(item))
 elif 'enum' in props.get('code',{}):
  entry=copy.deepcopy(item);entry['allOf'][1]['properties']['code']['enum'].append('APPLICATION_POLICY_CONFLICT');choices.append(entry)
choices.append({'allOf':[ref('Envelope'),{'type':'object','properties':{'code':{'const':'APPLICATION_STRUCTURE_NOT_EMPTY'},'data':ref('StructureDeletionDependencies')}}]})
s['StructureDeletionErrorEnvelope']={'oneOf':choices}
base='/api/v1/applications/{appId}'
for kind,suffix,resourceParam,request in [
 ('application','',None,'ApplicationDeletionRequest'),('directory','/directories/{directoryId}','directoryId','DirectoryDeletionRequest'),('table','/tables/{tableId}','tableId','StructureDeletionRequest'),('form','/forms/{viewId}','viewId','StructureDeletionRequest')]:
 op=copy.deepcopy(api['paths'][base+'/forms/{viewId}']['put'])
 op['operationId']='delete'+kind.title()+'NonCascade';op['summary']='Delete active '+kind+' without cascade'
 op['description']='V073 approved noncascade deletion. Closed 4096-byte JSON; current owner or Bootstrap and live Session/CSRF/expected actor. Retain stable metadata identities, typed business rows and all history/private presets. Reject active dependencies; unavailable is not empty. Same actor/key/fingerprint replays only the six-key minimum receipt before current resource eligibility. No resurrection, expiry or purge. Unknown COMMIT is 503 with original operationId; confirmed commit with failed renewal is 200 with cookies cleared. expectedResourceVersion is policyRevision for application, fixed 0 for directory, schemaVersion for table, viewVersion for form.'
 params=[{'name':'appId','in':'path','required':True,'schema':uuid}]
 if resourceParam:params.append({'name':resourceParam,'in':'path','required':True,'schema':uuid})
 params.extend([{'$ref':'#/components/parameters/ExpectedActor'},{'$ref':'#/components/parameters/CsrfToken'}]);op['parameters']=params
 op['requestBody']={'required':True,'content':{'application/json':{'schema':ref(request)}}}
 for status,response in op['responses'].items():
  if status=='200':response['content']['application/json']['schema']=ref('StructureDeletionResultEnvelope');response['description']='Atomic minimum confirmed deletion receipt'
  else:response['content']['application/json']['schema']=ref('StructureDeletionErrorEnvelope');response['description']='Closed current-resource, authorization, CAS, dependency or unconfirmed error'
 op['responses'].pop('201',None);api['paths'][base+suffix+'/deletion']={'post':op}
item=ref('StructureDeletionResult');choices=s['ApplicationOperation']['properties']['result']['oneOf']
if item not in choices:choices.append(item)
p.write_text(json.dumps(api,ensure_ascii=False,indent=2)+'\n')
p=r/'contracts/errors/codes.json';codes=json.loads(p.read_text());codes['APPLICATION_STRUCTURE_NOT_EMPTY']={'httpStatus':409,'grpcStatus':'FAILED_PRECONDITION','public':True};p.write_text(json.dumps(codes,ensure_ascii=False,indent=2)+'\n')
