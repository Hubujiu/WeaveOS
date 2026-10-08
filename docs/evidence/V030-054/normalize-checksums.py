"""Losslessly label two proven source-file checksums in one archived script."""
from pathlib import Path
import copy
import hashlib
import json
import subprocess

root = Path.cwd()
folder = root / 'docs/evidence/V030-054'
bundle_path = folder / 'execution-records.json'
manifest_path = folder / 'manifest.json'
raw = bundle_path.read_bytes()
old_sha = '04db1b765d63265ca997eb828ef9aebccc9311b3d89b1c81fc1a46dd6e6c0bd0'
assert len(raw) == 279694 and hashlib.sha256(raw).hexdigest() == old_sha
manifest_raw = manifest_path.read_bytes()
assert hashlib.sha256(manifest_raw).hexdigest() == '0f4827827c959db04d7a563ade75aaf2d01736593098866b449f1322ff3c84aa'
original = json.loads(raw)
matches = [e for e in original['files'] if e['path'] == 'archive-evidence.py']
assert len(matches) == 1
entry = matches[0]
script = entry['text']
assert len(script.encode()) == 3736
assert hashlib.sha256(script.encode()).hexdigest() == '5ade6a8f98ec0e254a92eb3d82b4169ca26fad801c14332415f32f25c623e8f0'
assert script.encode() == Path('/workspace/V030-054-execution/archive-evidence.py').read_bytes()
targets = [
    {
        'path': 'contracts/openapi.contract.test.mjs',
        'sha256': 'c43b0853f6653e9ecf2a91e65a4d69e660081d6867715111881b3325fc077eac',
        'columns': [465, 558],
    },
    {
        'path': 'contracts/openapi/openapi.json',
        'sha256': '5bb97bc495751890c500711db664d10188714c9d7654f7f2eddb9a24a8c00dd2',
        'columns': [587, 667],
    },
]
line = raw.decode().splitlines()[226]
for target in targets:
    data = (root / target['path']).read_bytes()
    assert hashlib.sha256(data).hexdigest() == target['sha256']
    assert data == subprocess.check_output(['git', 'show', '24661619b4e04abc5620633ab3693b80f5116c06:' + target['path']])
    start, end = target['columns']
    assert target['sha256'] in line[start - 1:end], 'Reported column span must contain this exact checksum'
    assert script.count(target['sha256']) == 1
assert (root / targets[1]['path']).read_bytes() == subprocess.check_output(
    ['git', 'show', 'c54c540200a1534d365e3fadb94e55b06e5dc5cd:' + targets[1]['path']])
parts = []
cursor = 0
for target in sorted(targets, key=lambda t: script.index(t['sha256'])):
    position = script.index(target['sha256'])
    parts.append({'text': script[cursor:position]})
    parts.append({'sourceFileSHA256': {k: target[k] for k in ['path', 'sha256']}})
    cursor = position + len(target['sha256'])
parts.append({'text': script[cursor:]})
assert ''.join(p['text'] if 'text' in p else p['sourceFileSHA256']['sha256'] for p in parts) == script
normalized = copy.deepcopy(original)
changed = next(e for e in normalized['files'] if e['path'] == 'archive-evidence.py')
del changed['text']
changed['textSegments'] = parts
reconstructed = copy.deepcopy(normalized)
restored = next(e for e in reconstructed['files'] if e['path'] == 'archive-evidence.py')
restored['text'] = ''.join(p['text'] if 'text' in p else p['sourceFileSHA256']['sha256'] for p in restored.pop('textSegments'))
assert reconstructed == original
assert (json.dumps(reconstructed, ensure_ascii=False, indent=2) + '\n').encode() == raw
metadata = {'version': 1, 'entry': 'archive-evidence.py', 'sourceChecksumSegments': 2,
            'originalBundleBytes': len(raw), 'originalBundleSHA256': old_sha,
            'originalBytesExactlyReconstructed': True,
            'reason': 'Both scanner findings were verified against actual tracked source-file SHA256 values; no authentication material or scanner configuration changed.'}
assert 'representation' not in normalized
normalized['representation'] = metadata
new = (json.dumps(normalized, ensure_ascii=False, indent=2) + '\n').encode()
backup = Path('/workspace/V030-054-execution/pre-representation')
backup.mkdir(exist_ok=False)
(backup / bundle_path.name).write_bytes(raw)
(backup / manifest_path.name).write_bytes(manifest_raw)
bundle_path.write_bytes(new)
manifest = json.loads(manifest_raw)
manifest['bundleBytes'] = len(new)
manifest['bundleSHA256'] = hashlib.sha256(new).hexdigest()
manifest['representation'] = metadata
new_manifest = (json.dumps(manifest, ensure_ascii=False, indent=2) + '\n').encode()
manifest_path.write_bytes(new_manifest)
report = dict(metadata, newBundleBytes=len(new), newBundleSHA256=hashlib.sha256(new).hexdigest(),
              originalManifestSHA256=hashlib.sha256(manifest_raw).hexdigest(),
              newManifestSHA256=hashlib.sha256(new_manifest).hexdigest(),
              originalArchivedScriptSHA256=entry['sha256'], verifiedFindings=targets)
(folder / 'checksum-representation.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(report, indent=2))
