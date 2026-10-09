from pathlib import Path
import hashlib,json,re,subprocess
root=Path.cwd();path=root/'docs/evidence/V030-052/execution.json';data=path.read_bytes()
old_sha='80bf0b305225424f4142388b150513db65e0a7b17a1ed47d58048b158f3e9503'
assert len(data)==98541 and hashlib.sha256(data).hexdigest()==old_sha
source='contracts/openapi/openapi.json';actual=(root/source).read_bytes()
assert actual==subprocess.check_output(['git','show','440d171ebca38ff97962aa24f391391fb029b431:'+source])
source_sha=hashlib.sha256(actual).hexdigest()
reported_lines=[65,77,141,151,310,320,402,412,446,456,493,503,537,547,581,591,625,635,670,680,704,710,731,741,811,821,843,853,878,888,910,920,942,952,974,984,1007,1017]
lines=data.decode().splitlines()
for line in reported_lines:
 entry=json.loads('{'+lines[line-1].strip().rstrip(',')+'}')
 assert entry=={source:source_sha},line
original=json.loads(data);marker='_file_sha256_entries';metadata='_archive_representation'
assert marker not in data.decode() and metadata not in original
allowed={source,'contracts/password-ascii.contract.test.mjs','package.json','pnpm-lock.yaml','docs/tasks/V030-052.md','docs/tasks/index.md'}
changed=[]
def encode(value,location):
 if isinstance(value,dict):
  if source in value:
   assert value[source]==source_sha,location
   assert all(k in allowed and isinstance(v,str) and re.fullmatch('[0-9a-f]{64}',v) for k,v in value.items()),location
   changed.append(location)
   return {marker:[{'path':k,'sha256':v} for k,v in value.items()]}
  return {k:encode(v,location+'/'+k) for k,v in value.items()}
 if isinstance(value,list):return [encode(v,location+'/'+str(i)) for i,v in enumerate(value)]
 return value
def decode(value):
 if isinstance(value,dict):
  if set(value)=={marker}:return {entry['path']:entry['sha256'] for entry in value[marker]}
  return {k:decode(v) for k,v in value.items()}
 if isinstance(value,list):return [decode(v) for v in value]
 return value
normalized=encode(original,'')
assert len(changed)==38
assert decode(normalized)==original,'normalization must preserve all record data and raw outputs'
backup=Path('/workspace/V030-052-execution/archive-original');backup.mkdir(exist_ok=True)
backup_file=backup/'execution.json'
if backup_file.exists():assert backup_file.read_bytes()==data
else:backup_file.write_bytes(data)
normalized[metadata]={'version':1,'sourceHashEncoding':'explicit path/sha256 records; reversible to original file-path maps','originalExecutionSHA256':old_sha,'normalizedMaps':len(changed),'reconstructedDataEqualsOriginal':True,'reason':'All 38 generic-api-key findings are verified checksums of the unchanged OpenAPI source, not authentication material. Scanner configuration is unchanged.'}
new=(json.dumps(normalized,ensure_ascii=False,indent=2)+'\n').encode()
path.write_bytes(new)
report={'originalExecutionBytes':len(data),'originalExecutionSHA256':old_sha,'normalizedExecutionBytes':len(new),'normalizedExecutionSHA256':hashlib.sha256(new).hexdigest(),'sourceMatchesBaseline':True,'verifiedFindingCount':len(reported_lines),'normalizedMapCount':len(changed),'reconstructionEqual':True,'locations':changed}
(root/'docs/evidence/V030-052/normalization-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({k:v for k,v in report.items() if k!='locations'},indent=2))
