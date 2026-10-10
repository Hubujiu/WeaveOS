"""Apply the frozen V030-070 template HTTP schemas without changing existing contracts."""
import json
from pathlib import Path
p=Path('contracts/openapi/openapi.json');api=json.loads(p.read_text());s=api['components']['schemas']
def ref(n):return {'$ref':'#/components/schemas/'+n}
def obj(props):return {'type':'object','additionalProperties':False,'required':list(props),'properties':props}
def arr(items,maximum=None):
 d={'type':'array','items':items}
 if maximum is not None:d['maxItems']=maximum
 return d
uuid={'type':'string','format':'uuid','pattern':'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$','not':{'const':'00000000-0000-0000-0000-000000000000'}}
name={'type':'string','minLength':1,'maxLength':100};nullable={'anyOf':[uuid,{'type':'null'}]};position={'type':'integer','minimum':0,'maximum':2147483647}
s['TemplateApplication']=obj({'id':uuid,'name':name})
s['TemplateDirectory']=obj({'id':uuid,'name':name,'parentId':nullable,'position':position})
s['TemplateTable']=obj({'id':uuid,'name':name,'directoryId':nullable,'position':position,'fields':arr(ref('Field'),200)})
s['TemplateForm']=obj({'id':uuid,'tableId':uuid,'name':name,'directoryId':nullable,'position':position,'layout':arr(ref('LayoutNodeInput'))})
s['TemplateGraph']=obj({'version':{'type':'integer','const':1},'nodes':arr(ref('TemplateNode'),100),'edges':arr(ref('TemplateEdge'),200)})
s['TemplateGraph']['properties']['nodes']['minItems']=2;s['TemplateGraph']['properties']['edges']['minItems']=1
s['TemplateNode']=obj({'id':uuid,'kind':{'type':'string','enum':['start','end','approval','condition']},'approval':{'anyOf':[ref('WorkflowApproval'),{'type':'null'}]},'condition':ref('RecordFilterGroup')});s['TemplateNode']['required']=['id','kind']
s['TemplateEdge']=obj({'from':uuid,'to':uuid,'branch':{'type':'string','enum':['true','false']}});s['TemplateEdge']['required']=['from','to']
s['TemplateWorkflow']=obj({'id':uuid,'tableId':uuid,'viewId':uuid,'name':name,'graph':ref('TemplateGraph'),'allowWithdraw':{'type':'boolean'},'triggers':ref('WorkflowTriggers')})
s['TemplateGrant']=obj({'resourceKind':{'type':'string','enum':['application','directory','form']},'resourceId':uuid,'action':{'type':'string','enum':['menu.enter','data.create','data.read','data.edit','data.history']},'rowScope':{'type':'string','enum':['all','own']},'fields':arr(uuid,200)})
s['TemplateGrant']['oneOf']=[ref('ApplicationMenuGrant'),ref('ApplicationChildMenuGrant'),ref('ApplicationDataGrant')]
s['TemplatePermissionGroup']=obj({'id':uuid,'name':name,'enabled':{'type':'boolean'},'memberIds':arr(uuid,10000),'grants':arr(ref('TemplateGrant'),10000)})
s['TemplateManifest']=obj({'format':{'type':'string','const':'weaveos.structure-template'},'version':{'type':'integer','const':1},'application':ref('TemplateApplication'),'directories':arr(ref('TemplateDirectory'),1000),'tables':arr(ref('TemplateTable'),128),'forms':arr(ref('TemplateForm'),256),'workflows':arr(ref('TemplateWorkflow'),128),'permissionGroups':arr(ref('TemplatePermissionGroup'),128)})
s['TemplateManifest']['description']='Structure only; raw UTF-8 manifest <=1048576 bytes, depth<=32. Reject duplicate/unknown keys and dangling or cross-resource references. No owner, records, attachments, instances, evidence, sessions or secrets. External references require explicit typed mappings; imports regenerate all internal identities and never deploy or enable workflows.'
s['TemplateBinding']=obj({'kind':{'type':'string','enum':['user','department']},'sourceId':uuid,'targetId':uuid})
s['TemplatePreflightRequest']=obj({'manifest':ref('TemplateManifest'),'bindings':arr(ref('TemplateBinding'))})
s['TemplateCounts']=obj({k:{'type':'integer','minimum':0} for k in ['directories','tables','fields','forms','workflows','permissionGroups']})
s['TemplatePreflightResult']=obj({'valid':{'type':'boolean','const':True},'counts':ref('TemplateCounts')})
for key,data in [('TemplateManifestEnvelope','TemplateManifest'),('TemplatePreflightEnvelope','TemplatePreflightResult')]:s[key]={'allOf':[ref('Envelope'),{'type':'object','properties':{'code':{'const':'OK'},'data':ref(data)}}]}
errors=['COMMON_INVALID_ARGUMENT','COMMON_VALIDATION_FAILED','COMMON_UNSUPPORTED_MEDIA_TYPE','COMMON_METHOD_NOT_ALLOWED','AUTH_UNAUTHENTICATED','COMMON_CSRF_REJECTED','AUTH_SESSION_CHANGED','APPLICATION_FORBIDDEN','APPLICATION_NOT_FOUND','APPLICATION_RESOURCE_INVALID','APPLICATION_TEMPLATE_NOT_EXPORTABLE','APPLICATION_TEMPLATE_TOO_LARGE','COMMON_SERVICE_UNAVAILABLE','API_NOT_FOUND']
s['TemplateErrorEnvelope']={'allOf':[ref('Envelope'),{'type':'object','properties':{'code':{'type':'string','enum':errors},'data':{'anyOf':[{'type':'null'},ref('ValidationErrorData')]}}}]}
def response(code,data=None,head=False):
 r={'description': 'Read-only result' if code==200 else 'Registered safe template or Session error; no internal SQL or source configuration leaked','headers':{'X-Request-Id':{'$ref':'#/components/headers/RequestId'},'Cache-Control':{'schema':{'type':'string','const':'no-store'}}}}
 if code==401:r['headers']['WWW-Authenticate']={'$ref':'#/components/headers/SessionChallenge'}
 if code==405:r['headers']['Allow']={'schema':{'type':'string'}}
 if not head:r['content']={'application/json':{'schema':ref(data or 'TemplateErrorEnvelope')}}
 return r
export='/api/v1/applications/{appId}/structure-template';path={}
for method in ['get','head']:
 path[method]={'operationId':('export' if method=='get' else 'head')+'ApplicationStructureTemplate','summary':'Export complete application structure without business data','description':'Current active owner or database Bootstrap; one read-only RR snapshot. Pending deletion, missing candidate or unknown stored configuration rejects the entire export. No query parameters. Session renewal is required before success.','security':[{'WebSession':[]}],'parameters':[{'name':'appId','in':'path','required':True,'schema':uuid},{'$ref':'#/components/parameters/ExpectedActor'}],'responses':{str(c):response(c,'TemplateManifestEnvelope' if c==200 else None,method=='head') for c in [200,400,401,403,404,405,409,503]}}
api['paths'][export]=path
api['paths']['/api/v1/application-templates/preflight']={'post':{'operationId':'preflightApplicationTemplate','summary':'Validate explicit template mappings without importing','description':'Advisory read-only validation: current active create permission or real Bootstrap, current target users and departments, complete internal remapping feasibility. Does not create application, physical table, operation, audit or personnel configuration. Import must recheck authority and targets. Strict exact-case JSON, no duplicate/unknown keys or query, no compressed encoding; 4MiB whole body and 1MiB manifest.','x-max-body-bytes':4194304,'security':[{'WebSession':[]}],'parameters':[{'$ref':'#/components/parameters/ExpectedActor'},{'$ref':'#/components/parameters/CsrfToken'}],'requestBody':{'required':True,'content':{'application/json':{'schema':ref('TemplatePreflightRequest')}}},'responses':{str(c):response(c,'TemplatePreflightEnvelope' if c==200 else None) for c in [200,400,401,403,404,405,409,413,415,503]}}}
p.write_text(json.dumps(api,ensure_ascii=False,indent=2)+'\n')
p=Path('contracts/errors/codes.json');codes=json.loads(p.read_text())
for k,status,grpc in [('APPLICATION_TEMPLATE_NOT_EXPORTABLE',409,'FAILED_PRECONDITION'),('APPLICATION_TEMPLATE_TOO_LARGE',413,'RESOURCE_EXHAUSTED'),('COMMON_METHOD_NOT_ALLOWED',405,'UNIMPLEMENTED')]:codes[k]={'httpStatus':status,'grpcStatus':grpc,'public':True}
p.write_text(json.dumps(codes,ensure_ascii=False,indent=2)+'\n')
