"""Add the frozen V071 private-view preset API without rewriting old schemas."""
import json,copy
from pathlib import Path
path=Path('contracts/openapi/openapi.json');api=json.loads(path.read_text());s=api['components']['schemas']
def ref(name):return {'$ref':'#/components/schemas/'+name}
def obj(properties):return {'type':'object','additionalProperties':False,'required':list(properties),'properties':properties}
def arr(items,limit=205):return {'type':'array','items':items,'uniqueItems':True,'maxItems':limit}
uuid={'type':'string','format':'uuid','pattern':'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$','not':{'const':'00000000-0000-0000-0000-000000000000'}}
version={'type':'integer','minimum':1,'maximum':9007199254740991}
column={'oneOf':[uuid,{'type':'string','enum':['id','createdBy','createdAt','updatedAt','recordVersion']}]}
s['ApplicationPresetAnd']=obj({'operator':{'const':'and','type':'string'},'children':{'type':'array','minItems':1,'maxItems':20,'items':ref('RecordFilterCondition')}})
s['ApplicationPresetOr']=obj({'operator':{'const':'or','type':'string'},'children':{'type':'array','minItems':1,'maxItems':20,'items':ref('ApplicationPresetAnd')}})
s['ApplicationPresetFilter']={'oneOf':[{'type':'null'},ref('ApplicationPresetAnd'),ref('ApplicationPresetOr')],'x-max-canonical-bytes':16384,'description':'At most20 leaves total; no empty intermediate groups or automatic DNF conversion. Existing typed comparisons and whole-visible-row read coverage remain mandatory.'}
state={'name':{'type':'string','minLength':1,'maxLength':100},'filter':ref('ApplicationPresetFilter'),'sort':ref('RecordSort'),'hiddenColumnIds':arr(column),'columnOrder':arr(column),'columnWidths':{'type':'object','maxProperties':205,'propertyNames':column,'additionalProperties':version}}
s['ApplicationPresetState']=obj(state);s['ApplicationPresetState']['x-max-canonical-bytes']=32768
s['ApplicationPresetState']['description']='Canonical private configuration, not rows or active/query state. At least one currently readable business column remains visible; hidden affects display only. No filtering/sorting without complete visible-row coverage. Widths are positive safe integer CSS pixels; order is partial and duplicate-free.'
input_state=copy.deepcopy(state);input_state['name']={'type':'string','minLength':1,'x-max-length-after-trim':100,'description':'TrimSpace before validating1–100 Unicode code points; no NUL. Exact case-sensitive uniqueness in current private scope.'}
s['ApplicationPresetCreate']=obj({'operationId':uuid,'expectedSchemaVersion':version,**input_state});s['ApplicationPresetCreate']['x-max-raw-body-bytes']=65536
s['ApplicationPresetUpdate']=obj({'operationId':uuid,'expectedSchemaVersion':version,'expectedVersion':version,**input_state});s['ApplicationPresetUpdate']['x-max-raw-body-bytes']=65536
s['ApplicationPresetValid']=obj({**state,'id':uuid,'appId':uuid,'viewId':uuid,'version':version,'invalid':{'type':'boolean','const':False},'createdAt':{'type':'string','format':'date-time'},'updatedAt':{'type':'string','format':'date-time'}})
s['ApplicationPresetInvalid']=obj({'id':uuid,'name':state['name'],'version':version,'invalid':{'type':'boolean','const':True},'reason':{'type':'string','enum':['FIELD_UNAVAILABLE','PERMISSION_CHANGED','DEFINITION_CHANGED']}})
s['ApplicationPresetItem']={'oneOf':[ref('ApplicationPresetValid'),ref('ApplicationPresetInvalid')]}
s['ApplicationPresetList']=obj({'items':{'type':'array','maxItems':20,'items':ref('ApplicationPresetItem')}})
s['ApplicationPresetMutationResult']=obj({'operationId':uuid,'id':uuid,'version':version})
for name,data in [('ApplicationPresetItemEnvelope','ApplicationPresetItem'),('ApplicationPresetListEnvelope','ApplicationPresetList'),('ApplicationPresetMutationEnvelope','ApplicationPresetMutationResult')]:
 s[name]={'allOf':[ref('Envelope'),{'type':'object','properties':{'code':{'const':'OK'},'data':ref(data)}}]}
error_codes=['COMMON_VALIDATION_FAILED','COMMON_UNSUPPORTED_MEDIA_TYPE','COMMON_SERVICE_UNAVAILABLE','AUTH_UNAUTHENTICATED','COMMON_CSRF_REJECTED','AUTH_SESSION_CHANGED','APPLICATION_NOT_FOUND','APPLICATION_FORBIDDEN','APPLICATION_SCHEMA_NOT_READY','APPLICATION_SCHEMA_CONFLICT','APPLICATION_OPERATION_CONFLICT','APPLICATION_PRESET_NAME_CONFLICT','APPLICATION_PRESET_LIMIT_REACHED','APPLICATION_PRESET_CONFLICT']
s['ApplicationPresetErrorEnvelope']={'oneOf':[{'allOf':[ref('Envelope'),{'type':'object','properties':{'code':{'type':'string','enum':error_codes},'data':{'oneOf':[{'type':'null'},ref('ValidationErrorData')]}}}]},{'allOf':[ref('Envelope'),{'type':'object','properties':{'code':{'const':'APPLICATION_OPERATION_UNCONFIRMED'},'data':ref('RecordUnconfirmedData')}}]}]}
base='/api/v1/applications/{appId}/forms/{viewId}/table-presets'
for suffix,methods in [('',{'get':('listApplicationPresets',200,'ApplicationPresetListEnvelope'),'head':('headApplicationPresets',200,None),'post':('createApplicationPreset',201,'ApplicationPresetMutationEnvelope')}),('/{presetId}',{'get':('getApplicationPreset',200,'ApplicationPresetItemEnvelope'),'head':('headApplicationPreset',200,None),'put':('updateApplicationPreset',200,'ApplicationPresetMutationEnvelope'),'delete':('discardApplicationPreset',204,None)})]:
 routes={}
 for method,(operation,status,response_schema) in methods.items():
  params=[{'name':name,'in':'path','required':True,'schema':uuid} for name in ['appId','viewId']+(['presetId'] if suffix else [])]+[{'$ref':'#/components/parameters/ExpectedActor'}]
  if method not in ['get','head']:params.append({'$ref':'#/components/parameters/CsrfToken'})
  if method=='delete':params += [{'name':'operationId','in':'query','required':True,'schema':uuid},{'name':'expectedVersion','in':'query','required':True,'schema':version}]
  headers={'X-Request-Id':{'$ref':'#/components/headers/RequestId'},'Cache-Control':{'schema':{'type':'string','const':'no-store'}}}
  success={'description':'Current private configuration or original confirmed minimum receipt. Invalid configurations are redacted as a whole.','headers':copy.deepcopy(headers)}
  if status==201:success['headers']['Location']={'schema':{'type':'string'},'description':'Created private preset URL'}
  if response_schema:success['content']={'application/json':{'schema':ref(response_schema)}}
  responses={str(status):success}
  for code in [400,401,403,404,409,415,503]:
   response={'description':'Safe private preset, current Session, schema/CAS or unknown-commit error; no private operands or internal SQL.','headers':copy.deepcopy(headers)}
   if code==401:response['headers']['WWW-Authenticate']={'$ref':'#/components/headers/SessionChallenge'}
   if method!='head':response['content']={'application/json':{'schema':ref('ApplicationPresetErrorEnvelope')}}
   responses[str(code)]=response
  op={'operationId':operation,'summary':operation,'security':[{'WebSession':[]}],'parameters':params,'responses':responses,'description':'Trusted Session owner plus real application and form-view scope. Same-table views do not share presets. Current menu.enter and data.read required; original confirmed write recovery returns only operationId/id/version. Save is not apply; no automatic query refresh or projection narrowing. No unknown query or compressed body.'}
  if method in ['post','put']:op['requestBody']={'required':True,'content':{'application/json':{'schema':ref('ApplicationPresetCreate' if method=='post' else 'ApplicationPresetUpdate')}}};op['x-max-body-bytes']=65536
  routes[method]=op
 api['paths'][base+suffix]=routes
for name in ['ApplicationPresetMutationResult','WorkflowMutationResult','WorkflowManualStartResult','WorkflowRoundStartResult','WorkflowDeletionResult']:
 recovery=ref(name)
 if recovery not in s['ApplicationOperation']['properties']['result']['oneOf']:s['ApplicationOperation']['properties']['result']['oneOf'].append(recovery)
path.write_text(json.dumps(api,ensure_ascii=False,indent=2)+'\n')
p=Path('contracts/errors/codes.json');codes=json.loads(p.read_text())
for name in ['APPLICATION_PRESET_NAME_CONFLICT','APPLICATION_PRESET_LIMIT_REACHED','APPLICATION_PRESET_CONFLICT']:codes[name]={'httpStatus':409,'grpcStatus':'ABORTED' if name=='APPLICATION_PRESET_CONFLICT' else 'RESOURCE_EXHAUSTED' if name=='APPLICATION_PRESET_LIMIT_REACHED' else 'ALREADY_EXISTS','public':True}
p.write_text(json.dumps(codes,ensure_ascii=False,indent=2)+'\n')
