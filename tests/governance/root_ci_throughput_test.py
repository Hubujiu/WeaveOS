"""Root-owned PRD V030-049 executable workflow contracts.

These assert scheduler/cache declarations, not successful product execution.
Independent runtime/report contracts are exercised separately.
V030-055 additionally executes reviewed disk-preparation Bash with recorded external effects.
"""
import copy
import json
import shutil
import subprocess
import sys
import tempfile
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

    def test_preflight_cannot_reserve_empty_go_dependency_cache(self):
        job = workflow(ROOT, 'preflight.yml')['jobs']['preflight']
        setups = [step for step in job['steps']
                  if step.get('uses', '').startswith('actions/setup-go@')]
        self.assertEqual(len(setups), 1)
        self.assertEqual(setups[0]['with'].get('cache'), 'false',
                         'gofmt-only preflight must not reserve an immutable empty cache')
        for file in ['ci.yml', 'performance.yml']:
            for name, target in workflow(ROOT, file)['jobs'].items():
                for step in target.get('steps', []):
                    if step.get('uses', '').startswith('actions/setup-go@'):
                        with self.subTest(file=file, job=name):
                            self.assertIn('.github/workflows/preflight.yml',
                                          step['with']['cache-dependency-path'].split(),
                                          'cache key must invalidate the old empty preflight generation')

    def test_python_download_cache_uses_actual_runtime(self):
        job = workflow(ROOT, 'preflight.yml')['jobs']['preflight']
        caches = [step for step in job['steps'] if step.get('with', {}).get('path') == '.work/pip-cache']
        self.assertEqual(len(caches), 1)
        self.assertIn('steps.python-runtime.outputs.version', caches[0]['with']['key'])
        versions = [step for step in job['steps'] if step.get('id') == 'python-runtime']
        self.assertEqual(len(versions), 1)
        self.assertIn('python3', versions[0]['run'])
        self.assertIn('GITHUB_OUTPUT', versions[0]['run'])

class HostedDiskContracts(unittest.TestCase):
    """V030-055: real Bash control flow; external effects are recording stubs.

    This executes reviewed repository code, not arbitrary untrusted shell.
    It does not delete an SDK or prove real disk capacity or tool behavior.
    """

    def setUp(self):
        doc = workflow(ROOT, 'acceptance.yml')
        job = doc['jobs']['product']
        matches = [step for step in job['steps'] if step.get('name') ==
                   'Prepare hosted Linux disk for browser images and vulnerability database']
        self.assertEqual(len(matches), 1, 'one reachable disk preparation step is required')
        self.step = matches[0]
        self.assertEqual(self.step.get('shell'), 'bash', 'exercise the declared GitHub Bash semantics')
        self.assertNotIn('if', self.step, 'disk preparation cannot be conditionally skipped')
        self.assertNotIn('continue-on-error', self.step, 'disk failure cannot be hidden')
        self.assertIsInstance(self.step.get('run'), str)
        for scope in [doc, job, self.step]:
            self.assertFalse({'RUNNER_ENVIRONMENT', 'RUNNER_OS'} & set(scope.get('env', {})),
                             'workflow cannot override the runner eligibility inputs')

    def exercise(self, environment, operating_system, cleanup_status=0):
        # Only external side effects are replaced. The original YAML run body,
        # Bash parser, test builtin, ordering and exit propagation remain real.
        bash = shutil.which('bash')
        self.assertIsNotNone(bash, 'Bash is required; absence is an environment error')
        with tempfile.TemporaryDirectory(prefix='weaveos-hosted-disk-') as directory:
            temp = Path(directory)
            bin_dir = temp / 'bin'
            bin_dir.mkdir()
            calls_file = temp / 'calls.jsonl'
            stub = ('#!' + sys.executable + '\n' +
                    'import json, os, sys\n'
                    'from pathlib import Path\n'
                    'name = Path(sys.argv[0]).name\n'
                    'with open(os.environ["CALLS_FILE"], "a") as output:\n'
                    '    output.write(json.dumps([name, *sys.argv[1:]]) + "\\n")\n'
                    'sys.exit(int(os.environ["CLEANUP_STATUS"]) if name == "sudo" '
                    'else (0 if name == "df" else 97))\n')
            for name in ['sudo', 'df', 'rm', 'docker']:
                executable = bin_dir / name
                executable.write_text(stub)
                executable.chmod(0o700)
            script = temp / 'disk.sh'
            script.write_text(self.step['run'])
            # No inherited credentials, BASH_ENV, ENV, shell functions or PATH.
            env = {'PATH': str(bin_dir), 'HOME': str(temp), 'LC_ALL': 'C',
                   'CALLS_FILE': str(calls_file), 'CLEANUP_STATUS': str(cleanup_status)}
            if environment is not None:
                env['RUNNER_ENVIRONMENT'] = environment
            if operating_system is not None:
                env['RUNNER_OS'] = operating_system
            result = subprocess.run([bash, '--noprofile', '--norc', '-e', '-o', 'pipefail', str(script)],
                                    cwd=temp, env=env, capture_output=True, text=True, timeout=10)
            calls = [json.loads(line) for line in calls_file.read_text().splitlines()] if calls_file.exists() else []
            return result, calls

    def test_only_hosted_linux_can_issue_any_external_preparation_call(self):
        for environment in ['github-hosted', 'self-hosted', 'unknown', '', None]:
            for operating_system in ['Linux', 'Windows', 'macOS', '', None]:
                if (environment, operating_system) == ('github-hosted', 'Linux'):
                    continue
                with self.subTest(environment=environment, operating_system=operating_system):
                    result, calls = self.exercise(environment, operating_system)
                    self.assertNotEqual(result.returncode, 0, 'ineligible runner must refuse preparation')
                    self.assertEqual(calls, [], 'ineligible runner issued an external preparation call')

    def test_hosted_linux_requests_only_the_single_approved_sdk_cleanup(self):
        result, calls = self.exercise('github-hosted', 'Linux')
        self.assertEqual(result.returncode, 0, result.stderr)
        # Exact current command adapter, not a general sudo/rm argument parser.
        # The independent target restriction is the single unused Android SDK.
        self.assertEqual([call for call in calls if call[0] != 'df'],
                         [['sudo', 'rm', '-rf', '--', '/usr/local/lib/android']],
                         'cleanup must request exactly the one approved SDK, with no extra destructive calls')

    def test_cleanup_failure_stops_the_step_without_successful_continuation(self):
        result, calls = self.exercise('github-hosted', 'Linux', cleanup_status=23)
        self.assertEqual(result.returncode, 23, 'Bash must propagate the actual cleanup failure')
        self.assertEqual(calls, [['sudo', 'rm', '-rf', '--', '/usr/local/lib/android']],
                         'failed cleanup must stop before subsequent external commands')


if __name__ == '__main__':
    unittest.main()
