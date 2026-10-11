"""Register the pre-frozen independent structure-copy contract after observed RED."""
import copy,json,pathlib
r=pathlib.Path(__file__).resolve().parents[2];p=r/'contracts/openapi/openapi.json';api=json.loads(p.read_text());s=api['components']['schemas']
ref=lambda n:{'$ref':'#/components/schemas/'+n}
obj=lambda props:{'type':'object','additionalProperties':False,'required':list(props),'properties':props}
version=lambda minimum:{'type':'integer','minimum':minimum,'maximum':9007199254740991}
uuid=copy.deepcopy(s['TemplateImportRequest']['properties']['operationId']);nullable={'anyOf':[uuid,{'type':'null'}]}
name={'type':'string','pattern':r'^(?![\s\S]*\u0000)\s*\S(?:[\s\S]{0,98}\S)?\s*$','description':'TrimSpace then 1..100 Unicode code points, no NUL; request bound also applies.'}
kinds={'type':'string','enum':['application','directory','table','form']}
s['StructureCopySource']=obj(dict(appId=uuid,kind=kinds,id=uuid))
s['StructureCopyTarget']=obj(dict(appId=nullable,directoryId=nullable,name=name))
s['StructureCopyGroupMapping']=obj(dict(sourceGroupId=uuid,name=name))
props=dict(source=ref('StructureCopySource'),target=ref('StructureCopyTarget'))
scope=[{'if':{'properties':{'source':{'properties':{'kind':{'const':'application'}}}}},'then':{'properties':{'target':{'properties':{'appId':{'type':'null'},'directoryId':{'type':'null'}}}}},'else':{'properties':{'target':{'properties':{'appId':uuid}}}}}]
s['StructureCopyPlanRequest']=obj(props);s['StructureCopyPlanRequest']['allOf']=scope
props={**props,'expectedSourceFingerprint':{'type':'string','pattern':'^[0-9a-f]{64}$'},'expectedTargetStructureVersion':version(0),'expectedTargetPolicyRevision':version(0),'bindings':{'type':'array','items':ref('TemplateBinding')},'groupMappings':{'type':'array','items':ref('StructureCopyGroupMapping'),'maxItems':128}}
s['StructureCopyPreflightRequest']=obj(props);s['StructureCopyPreflightRequest']['allOf']=scope
s['StructureCopyExecuteRequest']=obj({**props,'operationId':uuid});s['StructureCopyExecuteRequest']['allOf']=scope
s['StructureCopyReferences']=obj(dict(userIds={'type':'array','items':uuid,'uniqueItems':True},departmentIds={'type':'array','items':uuid,'uniqueItems':True}))
s['StructureCopyPlanResult']=obj(dict(sourceFingerprint={'type':'string','pattern':'^[0-9a-f]{64}$'},targetStructureVersion=version(0),targetPolicyRevision=version(0),manifest=ref('TemplateManifest'),externalReferences=ref('StructureCopyReferences')))
s['StructureCopyPreflightResult']=obj(dict(valid={'type':'boolean','const':True},counts=ref('TemplateCounts'),mappedManifest=ref('TemplateManifest')))
s['StructureCopyResult']=obj(dict(operationId=uuid,appId=uuid,resourceKind=kinds,id=uuid,structureVersion=version(1),policyRevision=version(1),status={'type':'string','const':'copied'}))
def envelope(code,data):return {'allOf':[ref('Envelope'),{'type':'object','properties':{'code':code,'data':data}}]}
for prefix,result in [('StructureCopyPlan','StructureCopyPlanResult'),('StructureCopyPreflight','StructureCopyPreflightResult'),('StructureCopy','StructureCopyResult')]:s[prefix+'Envelope']=envelope({'const':'OK'},ref(result))
codes=[c for c in s['TemplateErrorEnvelope']['allOf'][1]['properties']['code']['enum'] if c!='COMMON_VALIDATION_FAILED']+['APPLICATION_COPY_SOURCE_CHANGED']
s['StructureCopyErrorEnvelope']={'oneOf':[envelope({'type':'string','enum':codes},{'type':'null'}),envelope({'const':'COMMON_VALIDATION_FAILED'},ref('ValidationErrorData')),envelope({'const':'APPLICATION_STRUCTURE_CONFLICT'},obj(dict(currentStructureVersion=version(0)))),ref('TemplateUnconfirmedEnvelope')]}
for action,request,result,status in [('plan','StructureCopyPlanRequest','StructureCopyPlanEnvelope','200'),('preflight','StructureCopyPreflightRequest','StructureCopyPreflightEnvelope','200'),('execute','StructureCopyExecuteRequest','StructureCopyEnvelope','201')]:
 op=copy.deepcopy(api['paths']['/api/v1/application-templates/import']['post']);op['operationId']={'plan':'planStructureCopy','preflight':'preflightStructureCopy','execute':'executeStructureCopy'}[action];op['summary']=action.capitalize()+' independent empty structure copy'
 op['description']='Copy only selected application/directory/table/form and required complete table dependencies, never business rows/history. Explicit source and target management authority, user/department mappings and independent new group names. Source fingerprint and target structure/policy CAS are rechecked in the same transaction. All new workflows are disabled. Strict JSON <=4194304 bytes, depth32; reject unknown/duplicate/case-confused keys, query (including bare ?), compressed bodies and trailing input. Plan/preflight are one read-only RR snapshot. Execute is one local atomic transaction; original actor/key/body recovers seven-key receipt after permission or source revocation. Unknown COMMIT returns only operationId with503 and no Location/Cookies; confirmed commit renewal failure retains201 and clears Cookies.'
 op['requestBody']['content']['application/json']['schema']=ref(request)
 success=op['responses'].pop('201');success['description']='Read-only advisory result' if status=='200' else 'Confirmed independent empty copy or original seven-key receipt';success['content']['application/json']['schema']=ref(result)
 if status=='200':success['headers'].pop('Location',None)
 else:success['headers']['Location']['schema']={'type':'string','pattern':'^/api/v1/applications/[0-9a-f-]+(?:/(?:directories|tables|forms)/[0-9a-f-]+)?$'}
 for response in op['responses'].values():response['content']['application/json']['schema']=ref('StructureCopyErrorEnvelope')
 op['responses'][status]=success;api['paths']['/api/v1/structure-copies/'+action]={'post':op}
item=ref('StructureCopyResult');choices=s['ApplicationOperation']['properties']['result']['oneOf']
if item not in choices:choices.append(item)
p.write_text(json.dumps(api,ensure_ascii=False,indent=2)+'\n')
p=r/'contracts/errors/codes.json';codes=json.loads(p.read_text());codes['APPLICATION_COPY_SOURCE_CHANGED']={'httpStatus':409,'grpcStatus':'FAILED_PRECONDITION','public':True};p.write_text(json.dumps(codes,ensure_ascii=False,indent=2)+'\n')
