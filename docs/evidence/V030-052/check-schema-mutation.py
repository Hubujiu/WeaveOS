from pathlib import Path
import datetime
import hashlib
import json
import re
import shutil
import subprocess
import tempfile
import time

root = Path.cwd()
node = '/workspace/.weaveos-tools/node_modules/node-linux-x64/bin/node'
assert subprocess.check_output([node, '--version'], text=True).strip() == 'v24.14.0'
tracked = ['contracts/password-ascii.contract.test.mjs', 'contracts/openapi/openapi.json', 'package.json', 'pnpm-lock.yaml']
source_hashes = {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in tracked}
assert source_hashes[tracked[0]] == '3ce54f2533f38a7fca1071812b805f9efa51af8d28096dc4cdd4b51cd135e5a0'
base = Path(tempfile.mkdtemp(prefix='schema-mutation-', dir='/workspace/V030-052-execution'))
repo = base / 'repo'
shutil.copytree(root / 'contracts', repo / 'contracts')
(repo / 'node_modules').symlink_to(root / 'node_modules', target_is_directory=True)
schema_path = repo / 'contracts/openapi/openapi.json'
original = schema_path.read_bytes()
command = [node, '--test', '--test-reporter=tap', 'contracts/password-ascii.contract.test.mjs']
records = []
try:
    for name, mutate, expected_pass, expected_fail in [('baseline', False, 2, 0), ('wrong-max-length', True, 1, 1), ('restored', False, 2, 0)]:
        schema_path.write_bytes(original)
        if mutate:
            document = json.loads(original)
            document['paths']['/api/v1/registrations']['post']['requestBody']['content']['application/json']['schema']['properties']['password']['maxLength'] = 3
            schema_path.write_text(json.dumps(document, ensure_ascii=False, indent=2) + '\n')
        report = base / name
        report.mkdir()
        started = datetime.datetime.now(datetime.timezone.utc).isoformat()
        start = time.monotonic()
        result = subprocess.run(command, cwd=repo, capture_output=True, text=True, timeout=60)
        (report / 'stdout.txt').write_text(result.stdout)
        (report / 'stderr.txt').write_text(result.stderr)
        counts = {key: int(re.findall(r'^# ' + key + r' (\d+)$', result.stdout, re.M)[-1]) for key in ['tests', 'pass', 'fail', 'cancelled', 'skipped']}
        record = {'variant': name, 'started': started, 'command': command, 'cwd': str(repo), 'exit': result.returncode, 'durationSeconds': round(time.monotonic() - start, 6), 'counts': counts, 'schemaSHA256': hashlib.sha256(schema_path.read_bytes()).hexdigest()}
        records.append(record)
        (report / 'run.json').write_text(json.dumps(record, indent=2) + '\n')
        (base / 'runs.json').write_text(json.dumps(records, indent=2) + '\n')
        print(json.dumps(record), flush=True)
        assert counts == {'tests': 2, 'pass': expected_pass, 'fail': expected_fail, 'cancelled': 0, 'skipped': 0}, record
        assert (result.returncode == 0) == (expected_fail == 0), record
        if mutate:
            assert 'ERR_ASSERTION' in result.stdout and 'approved synthetic password fixture must match schema' in result.stdout, result.stdout
            assert re.search(r'^\s+expected: true$', result.stdout, re.M) and re.search(r'^\s+actual: false$', result.stdout, re.M), result.stdout
finally:
    schema_path.write_bytes(original)
    after = {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in tracked}
    (base / 'source-hashes.json').write_text(json.dumps({'before': source_hashes, 'after': after, 'isolatedSchemaRestored': schema_path.read_bytes() == original}, indent=2) + '\n')
    assert after == source_hashes, 'original task source changed'
print('Evidence:', base, flush=True)
