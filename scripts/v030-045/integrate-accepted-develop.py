"""Root-owned deterministic resolution for the accepted V043 baseline only."""
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
BASE = 'b056fc34f41d389c5a4aee54b6e81a29dae578bd'
def git(*args):
    return subprocess.check_output(['git', *args], cwd=ROOT, text=True)
allowed = {'contracts/openapi/openapi.json', 'docs/tasks/index.md', 'services/bff/internal/appstructure/record_http_decode.go', 'services/workflow-engine/formal_runtime_gate.py', 'services/workflow-engine/formal_runtime_gate_tests.py'}
unmerged = set(git('diff', '--name-only', '--diff-filter=U').splitlines())
if not unmerged <= allowed:
    raise SystemExit('Unexpected conflicts; Root must review: '+', '.join(sorted(unmerged-allowed)))
# Use the accepted API including node Save, then register only the new lifecycle delta.
api_path = 'contracts/openapi/openapi.json'
(ROOT/api_path).write_text(git('show', BASE+':'+api_path))
subprocess.check_call(['node', 'scripts/v030-045/register-lifecycle.mjs'], cwd=ROOT)
path = 'services/bff/internal/appstructure/record_http_decode.go'
s = git('show', BASE+':'+path)
if 'case "workflow.lifecycle.action"' in s or 'case "workflow.task.save"' not in s:
    raise SystemExit('Unexpected accepted decoder baseline')
s = s.replace('switch kind {', 'switch kind {\n\tcase "workflow.lifecycle.action":\n\t\trequired = []string{"operationId", "action", "basisToken"}\n\t\toptional = []string{"targetNodeId"}', 1)
s = s.replace('if kind == "workflow.task.action" {\n\t\tlimit = 4096', 'if kind == "workflow.task.action" || kind == "workflow.lifecycle.action" {\n\t\tlimit = 4096', 1)
block = '''\tif kind == "workflow.lifecycle.action" {
        var operation, action, token string
        if json.Unmarshal(m["operationId"], &operation) != nil || operation == "00000000-0000-0000-0000-000000000000" || json.Unmarshal(m["action"], &action) != nil || (action != "withdraw" && action != "return") || json.Unmarshal(m["basisToken"], &token) != nil || token == "" || utf8.RuneCountInString(token) > 256 { return nil, recordBodyInvalid() }
        target, present := m["targetNodeId"]
        if action == "withdraw" && present { return nil, recordBodyInvalid() }
        if action == "return" { var id string; if !present || json.Unmarshal(target,&id) != nil || !appfields.ValidID(id) || id == "00000000-0000-0000-0000-000000000000" { return nil, recordBodyInvalid() } }
    }
'''
needle = '\tfor _, key := range []string{"operationId", "targetRecordId"} {'
if s.count(needle) != 1: raise SystemExit('Decoder insertion point differs')
s = s.replace(needle, block+needle)
(ROOT/path).write_text(s)
path = 'docs/tasks/index.md'
s = git('show', BASE+':'+path)
s += '\n- [V030-045 · 撤回与实际节点退回](V030-045.md) — 当前资格、原子命令接受及HTTP分层通过；真实Flowable和最终CI待验收\n'
(ROOT/path).write_text(s)
# These two Root-authored files explicitly retain all six accepted identities and add one.
for path in ['services/workflow-engine/formal_runtime_gate.py', 'services/workflow-engine/formal_runtime_gate_tests.py']:
    s = git('show', 'HEAD:'+path)
    if 'TestRootFormalRuntimeNodeSaveThenApproveLatest' not in s or 'TestRootFormalRuntimeReturnThenWithdrawRecovery' not in s:
        raise SystemExit('Root gate lacks accepted or new scenario')
    (ROOT/path).write_text(s)
print('Resolved only frozen API, decoder, index, and cumulative seven-case gate; formatting/staging/verification still required.')
