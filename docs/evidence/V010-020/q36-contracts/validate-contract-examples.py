import json
from jsonschema import Draft202012Validator, FormatChecker
api=json.load(open('contracts/openapi/openapi.json'))
schemas=api['components']['schemas'];cases=0
# Independent approved-domain examples. x-* runtime/global limits are tested by
# their implementation owner; this checks the actual JSON Schema vocabulary.
def check(name,value,valid):
 global cases
 schema={'$ref':'#/components/schemas/'+name,'components':api['components']}
 errors=list(Draft202012Validator(schema,format_checker=FormatChecker()).iter_errors(value))
 assert bool(errors)!=valid,(name,value,[str(e) for e in errors[:1]])
 cases+=1
uid='00000000-0000-4000-8000-000000000001'
leaf={'field':'identityIds','operator':'neq','value':uid}
group={'operator':'and','children':[leaf,{'operator':'or','children':[leaf]}]}
check('MemberFilterGroup',group,True)
check('MemberFilterGroup',{'operator':'and','children':[]},False)
check('MemberFilterCondition',{**leaf,'operator':'gt'},False)
check('MemberFilterCondition',{**leaf,'value':None},False)
check('MemberFilterCondition',{'field':'account','operator':'neq','value':None},True)
for levels in [3,4]:
 g=leaf
 for _ in range(levels):g={'operator':'and','children':[g]}
 check('MemberFilterGroup',g,levels==3)
check('EventFilterCondition',{'field':'occurredAt','operator':'gte','value':None},False)
check('EventFilterCondition',{'field':'occurredAt','operator':'eq','value':{'date':'2026-10-01','timeZone':'Asia/Shanghai'}},True)
draft={'kind':'identity','targetId':None,'baseVersion':None,'payload':{'name':'','description':'','templateIds':[],'permissionCodes':[]}}
check('DraftCreateInput',draft,True)
check('DraftCreateInput',{**draft,'ownerId':uid},False)
check('DraftCreateInput',{**draft,'payload':{**draft['payload'],'password':'not-a-secret-test'}},False)
check('DraftCreateInput',{**draft,'targetId':uid,'baseVersion':0},False)
check('DraftCreateInput',{**draft,'targetId':uid,'baseVersion':1},True)
check('DraftCreateInput',{**draft,'targetId':uid},False)
check('DraftCreateInput',{'kind':'member-identities','targetId':uid,'baseVersion':0,'payload':{'identityIds':[]}},True)
check('DraftCreateInput',{'kind':'member-identities','targetId':None,'baseVersion':None,'payload':{'identityIds':[]}},False)
check('DraftCreateInput',{**draft,'payload':{'identityIds':[]}},False)
check('DraftUpdateInput',{'version':0,'payload':draft['payload']},False)
check('DraftUpdateInput',{'version':1,'payload':draft['payload'],'baseVersion':2},False)
for name in schemas:Draft202012Validator.check_schema(schemas[name])
print(f'{cases} independent JSON Schema examples passed; all schemas well-formed (jsonschema 4.26.0). x-* semantic/runtime limits are NOT proven here.')
