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
                self.assertTrue({'services/bff/go.mod', 'services/bff/go.sum'} <= paths)

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

    def test_component_shards_use_isolated_jobs_and_verified_completion(self):
        doc = workflow(ROOT, 'acceptance.yml')
        jobs = doc['jobs']
        self.assertIn('components', jobs, 'the full existing component suite must move to isolated jobs')
        job = jobs['components']
        self.assertEqual(job['runs-on'], 'ubuntu-24.04')
        self.assertEqual(job['strategy']['matrix']['shard'], ['1', '2', '3', '4'])
        self.assertEqual(job['strategy'].get('fail-fast'), 'false')
        commands = '\n'.join(step.get('run', '') for step in job.get('steps', []))
        self.assertIn('apps/web/playwright.component.config.ts', commands)
        self.assertIn('--list', commands, 'discovery must independently enumerate the whole candidate')
        self.assertRegex(commands, r'--shard[= ]\$\{\{\s*matrix\.shard\s*\}\}/4')
        self.assertIn('--workers=1', commands)
        self.assertIn('--retries=0', commands)
        uploads = [s for s in job['steps'] if s.get('uses', '').startswith('actions/upload-artifact@')]
        self.assertTrue(uploads)
        for step in uploads:
            for key in ['github.run_id', 'github.run_attempt', 'matrix.shard']:
                self.assertIn(key, step['with']['name'])
            self.assertEqual(step.get('if'), 'always()')
        self.assertIn('component-coverage', jobs)
        self.assertIn('components', ancestors(jobs, 'component-coverage'))
        self.assertIn('component-coverage', ancestors(jobs, 'product'))
        aggregate = '\n'.join(s.get('run', '') for s in jobs['component-coverage'].get('steps', []))
        self.assertIn('verify-component-artifacts', aggregate)

    def test_product_container_cache_contains_only_public_dependencies(self):
        doc = workflow(ROOT, 'acceptance.yml')
        job = doc['jobs']['product']
        self.assertIn('WEAVEOS_DEPENDENCY_CACHE_DIR', job.get('env', {}))
        caches = [step for step in job['steps']
                  if step.get('uses', '').startswith('actions/cache@') and
                  '.work/dependency-cache/go-mod' in step.get('with', {}).get('path', '').split()]
        self.assertEqual(len(caches), 1)
        settings = caches[0]['with']
        self.assertEqual(set(settings['path'].split()), {
            '.work/dependency-cache/go-mod', '.work/dependency-cache/go-build',
            '.work/dependency-cache/pnpm-store'})
        self.assertNotIn('restore-keys', settings)
        for key in ['runner.os', 'runner.arch', 'hashFiles(', '.go-version',
                    '.node-version', 'pnpm-lock.yaml', 'package.json',
                    'services/bff/go.mod', 'services/bff/go.sum',
                    'infra/acceptance/run.mjs', 'infra/runtime/run.mjs']:
            self.assertIn(key, settings['key'])

    def test_python_download_cache_uses_actual_runtime(self):
        job = workflow(ROOT, 'preflight.yml')['jobs']['preflight']
        caches = [step for step in job['steps'] if step.get('with', {}).get('path') == '.work/pip-cache']
        self.assertEqual(len(caches), 1)
        self.assertIn('steps.python-runtime.outputs.version', caches[0]['with']['key'])
        versions = [step for step in job['steps'] if step.get('id') == 'python-runtime']
        self.assertEqual(len(versions), 1)
        self.assertIn('python3', versions[0]['run'])
        self.assertIn('GITHUB_OUTPUT', versions[0]['run'])

if __name__ == '__main__':
    unittest.main()
