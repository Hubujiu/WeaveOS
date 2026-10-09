"""Root-owned PRD V030-049 executable workflow contracts.

These assert scheduler/cache declarations, not successful product execution.
Independent runtime/report contracts are exercised separately.
"""
import copy
import os
from pathlib import Path
import unittest
import yaml

ROOT = Path(os.environ.get('WEAVEOS_SOURCE_ROOT', Path(__file__).resolve().parents[2]))

def workflow(root, name):
    with (root / '.github/workflows' / name).open() as f:
        return yaml.load(f, Loader=yaml.BaseLoader)

def ancestors(jobs, name, visited=None):
    visited = set() if visited is None else set(visited)
    if name in visited:
        raise AssertionError('cyclic job dependencies')
    visited.add(name)
    needs = jobs[name].get('needs', [])
    needs = [needs] if isinstance(needs, str) else needs
    result = set(needs)
    for dependency in needs:
        assert dependency in jobs, f'unknown dependency {dependency}'
        result |= ancestors(jobs, dependency, visited)
    return result

def assert_gate_graph(test, doc):
    jobs = doc['jobs']
    gates = [name for name, job in jobs.items()
             if job.get('uses') == './.github/workflows/preflight.yml']
    test.assertEqual(len(gates), 1, 'exactly one call to shared preflight per entry workflow')
    gate = gates[0]
    for name, job in jobs.items():
        if name == gate:
            continue
        test.assertIn(gate, ancestors(jobs, name), f'{name} may start without preflight')
        test.assertNotIn('always()', job.get('if', ''), f'{name} bypasses failed preflight')
        test.assertNotEqual(job.get('continue-on-error'), 'true')

class WorkflowContracts(unittest.TestCase):
    def test_all_test_entrypoints_scheduler_block_heavy_work_on_preflight(self):
        for file in ['ci.yml', 'acceptance.yml'] + (['performance.yml'] if (ROOT / '.github/workflows/performance.yml').exists() else []):
            with self.subTest(file=file):
                assert_gate_graph(self, workflow(ROOT, file))

    def test_graph_contract_detects_missing_edge_and_failure_bypass(self):
        valid = {'jobs': {'gate': {'uses': './.github/workflows/preflight.yml'},
                          'build': {'needs': 'gate'}, 'test': {'needs': ['build']}}}
        assert_gate_graph(self, valid)
        for mutate in [lambda d: d['jobs']['build'].pop('needs'),
                       lambda d: d['jobs']['test'].update({'if': 'always()'}),
                       lambda d: d['jobs']['gate'].update({'uses': './different.yml'})]:
            invalid = copy.deepcopy(valid)
            mutate(invalid)
            with self.assertRaises(AssertionError):
                assert_gate_graph(self, invalid)

    def test_all_existing_go_setup_actions_use_locked_dependency_cache(self):
        setups = []
        for file in ['ci.yml'] + (['performance.yml'] if (ROOT / '.github/workflows/performance.yml').exists() else []):
            doc = workflow(ROOT, file)
            for name, job in doc['jobs'].items():
                for step in job.get('steps', []):
                    if step.get('uses', '').startswith('actions/setup-go@'):
                        setups.append((file, name, step['with']))
        self.assertGreaterEqual(len(setups), 8, 'existing Go preparation must remain covered')
        for file, name, settings in setups:
            with self.subTest(file=file, job=name):
                self.assertEqual(settings.get('go-version-file'), '.go-version')
                self.assertEqual(settings.get('cache'), 'true', 'actual setup-go cache enabled')
                paths = set(settings.get('cache-dependency-path', '').split())
                self.assertTrue({'services/bff/go.mod', 'services/bff/go.sum'} &lt;= paths)

    def test_java_cache_is_narrow_and_bound_to_build_inputs(self):
        matches = []
        for file in ['ci.yml'] + (['performance.yml'] if (ROOT / '.github/workflows/performance.yml').exists() else []):
            doc = workflow(ROOT, file)
            for name, job in doc['jobs'].items():
                if not any('prepare-build.sh' in s.get('run', '') for s in job.get('steps', [])):
                    continue
                caches = [s for s in job['steps'] if s.get('uses', '').startswith('actions/cache@')
                          and 'services/workflow-engine/.work/m2' in s.get('with', {}).get('path', '').split()]
                self.assertEqual(len(caches), 1, f'{file}/{name} requires one narrow Java cache')
                settings = caches[0]['with']
                self.assertEqual(set(settings['path'].split()), {
                    'services/workflow-engine/.work/m2', 'services/workflow-engine/.work/toolchain'})
                self.assertNotIn('restore-keys', settings, 'no cross-input fallback')
                for value in ['runner.os', 'runner.arch', 'hashFiles(',
                              'services/workflow-engine/pom.xml',
                              'services/workflow-engine/prepare-build.sh',
                              'services/workflow-engine/maven-settings.xml']:
                    self.assertIn(value, settings['key'])
                matches.append((file, name))
        self.assertGreater(len(matches), 0)

if __name__ == '__main__':
    unittest.main()
