"""Root-authored exact five-conflict resolution; never a generic ours/theirs merge."""
from pathlib import Path
import subprocess, json, hashlib
ROOT=Path(__file__).resolve().parents[2]
OURS='0ea9ddfa1ca0fe39988dfe88ab68cde895544204'
THEIRS='3d793c6f1bb88b8889b483a1fead81bce13e26be'
FIX='5a88d63d8d510190308e01bc1cf2f80291338030'
def git(*args):return subprocess.check_output(['git',*args],cwd=ROOT,text=True)
def replace(s,a,b):
 if s.count(a)!=1:raise SystemExit('Unexpected exact insertion point: '+a)
 return s.replace(a,b,1)
expected={'contracts/openapi/openapi.json','docs/tasks/index.md','services/bff/internal/apprecordhttp/http.go','services/bff/internal/apprecordservice/service.go','services/bff/internal/appstructure/record_http_decode.go'}
if git('rev-parse','HEAD').strip()!=OURS or git('rev-parse','MERGE_HEAD').strip()!=THEIRS:raise SystemExit('Wrong integration parents')
if set(git('diff','--name-only','--diff-filter=U').splitlines())!=expected:raise SystemExit('Unexpected conflict set; Root review required')
p='contracts/openapi/openapi.json';(ROOT/p).write_text(git('show',THEIRS+':'+p));subprocess.check_call(['node','scripts/v030-046/register-workflow-read.mjs'],cwd=ROOT)
p='services/bff/internal/apprecordservice/service.go';s=git('show',THEIRS+':'+p)
s=replace(s,'\tWorkflowLifecycleBases *querycontext.Store','\tWorkflowLifecycleBases *querycontext.Store\n\tWorkflowReads *querycontext.Store')
s=replace(s,'WorkflowLifecycleBases: newWorkflowLifecycleStore(client, generation)}','WorkflowLifecycleBases: newWorkflowLifecycleStore(client, generation), WorkflowReads: newWorkflowReadStore(client, generation)}');(ROOT/p).write_text(s)
p='services/bff/internal/apprecordhttp/http.go';s=git('show',THEIRS+':'+p);s=replace(s,'if s.workflowLifecycleHTTP(w, r, p) || s.workflowHTTP(w, r, p) {','if s.workflowReadHTTP(w, r, p) || s.workflowLifecycleHTTP(w, r, p) || s.workflowHTTP(w, r, p) {');(ROOT/p).write_text(s)
p='services/bff/internal/appstructure/record_http_decode.go';s=git('show',THEIRS+':'+p)
s=replace(s,'switch kind {','switch kind {\n\tcase "workflow.instances.search":\n\t\trequired = []string{"page"}\n\t\toptional = []string{"pageSize", "queryVersion"}')
s=replace(s,'if kind == "workflow.task.action" || kind == "workflow.lifecycle.action" {','if kind == "workflow.task.action" || kind == "workflow.lifecycle.action" || kind == "workflow.instances.search" {');(ROOT/p).write_text(s)
p='docs/tasks/index.md';s=git('show',THEIRS+':'+p);lines=[l for l in git('show',OURS+':'+p).splitlines() if l.startswith('- [V030-046')]
if len(lines)!=1:raise SystemExit('Expected one V046 task index line')
(ROOT/p).write_text(s+'\n'+lines[0]+'\n')
# The same isolated native-gesture fix; do not copy the diagnostic workflow.
for p in ['apps/web/src/TablePresetManager.tsx','apps/web/src/q36-b2.component.spec.ts']:(ROOT/p).write_text(git('show',FIX+':'+p))
p='infra/server/deploy/compatibility.json';manifest=json.loads(git('show',THEIRS+':'+p));migration='migrations/00023_workflow_record_order_index.sql'
if any(x['path']==migration for x in manifest['migrations']):raise SystemExit('Index migration already registered')
manifest['migrations'].append({'path':migration,'sha256':hashlib.sha256((ROOT/'db'/migration).read_bytes()).hexdigest()})
manifest['source']+=' V030-046 read access path: hot00023 after hot00022 adds non-partial record-scoped stable-order index; no data/role changes; Down drops only that index. No deployment authorization.'
(ROOT/p).write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
p='docs/tasks/V030-046.md';s=(ROOT/p).read_text();start=s.index('<!-- task-meta')+len('<!-- task-meta');end=s.index('-->',start);meta=json.loads(s[start:end]);meta['pr']=54
if 'V030-045' not in meta['dependsOn']:meta['dependsOn'].append('V030-045')
for path in ['services/bff/internal/apprecordservice/root_workflow_read_cost_test.go','services/bff/internal/apprecordservice/root_workflow_read_index_test.go','db/migrations/00023_workflow_record_order_index.sql','infra/server/deploy/compatibility.json','scripts/v030-046/integrate-lifecycle-candidate.py','apps/web/src/TablePresetManager.tsx','apps/web/src/q36-b2.component.spec.ts']:
 if path not in meta['allowedPaths']:meta['allowedPaths'].append(path)
s=s[:start]+'\n'+json.dumps(meta,ensure_ascii=False)+'\n'+s[end:]
s+='\n## Integrated candidate, not final acceptance\nRoot exact-resolution script integrates V045 candidate3d793c6 (migration022, lifecycle, cumulative seven-identity gate) with V046. V045 must pass its own develop acceptance; this merge is not that approval. Migration023 adds measured record-order index only after real missing-index RED. Synthetic100k/1000-related comparison is in access-path-cost; ordered first-page SQL0.115-0.150ms, fingerprint4.116-4.948ms, index9551872bytes, not full API latency or process memory. Root native-event regression exposed early focus-out close transitions consuming React clicks; isolated fix5a88d63d passed30 genuine gestures, whole three-engine regressions still independently checked. Exact integrated migrations/tests/CI remain pending; no main/deployment.\n'
(ROOT/p).write_text(s)
print('Exact conflicts resolved; both services/routes/decoders, lifecycle+Save API, and source-only023 preserved. Formatting, test and commit still required.')
