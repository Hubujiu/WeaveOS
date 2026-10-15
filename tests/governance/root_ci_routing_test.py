"""V059 independent scheduler output wiring; does not emulate GitHub's scheduler."""
import unittest
from root_ci_throughput_test import workflow, ROOT, assert_gate_graph
import copy
class Routing(unittest.TestCase):
    def test_entrypoints_have_fixed_fail_closed_aggregate(self):
        for file in ['ci.yml', 'acceptance.yml']:
            doc = workflow(ROOT, file)
            self.assertNotIn('paths', doc['on']['pull_request'] or {})
            self.assertNotIn('paths-ignore', doc['on']['pull_request'] or {})
            jobs = doc['jobs']
            self.assertIn('selection-gate', jobs)
            gate = jobs['selection-gate']
            self.assertEqual(gate['if'], 'always()')
            self.assertEqual(set(gate['needs']), set(jobs) - {'selection-gate'})
            self.assertNotIn('continue-on-error', gate)
            runs = [s['run'] for s in gate['steps'] if 'run' in s]
            self.assertEqual(runs, ['node scripts/check-ci-selection.mjs'])
            self.assertEqual(gate['env']['NEEDS_JSON'], '${{ toJSON(needs) }}')
            for name, job in jobs.items():
                if name in ['preflight','governance','selection-gate']: continue
                self.assertIn('preflight', job['needs'] if isinstance(job['needs'],list) else [job['needs']])
                lane = 'product' if file == 'acceptance.yml' or name == 'workflow-browser' else 'browser' if name == 'browser' else 'backend'
                self.assertEqual(job['if'], "needs.preflight.outputs." + lane + " == 'true'")
    def test_real_engine_browser_is_product_only_and_required(self):
        jobs = workflow(ROOT, 'ci.yml')['jobs']
        self.assertIn('workflow-browser', jobs, 'P23 composed real browser-engine proof is missing')
        job = jobs['workflow-browser']
        self.assertEqual(job['if'], "needs.preflight.outputs.product == 'true'")
        self.assertIn('workflow-browser', jobs['selection-gate']['needs'])
        commands = '\n'.join(step.get('run', '') for step in job['steps'])
        self.assertIn('pnpm build', commands)
        self.assertIn('WEAVEOS_PRODUCT_BROWSER_MODE=1', commands)
        self.assertIn('node services/workflow-engine/run-formal-runtime.mjs', commands)
        self.assertNotIn('continue-on-error', job)
        uploads = [s for s in job['steps'] if s.get('uses', '').startswith('actions/upload-artifact@')]
        self.assertEqual(len(uploads), 1)
        self.assertEqual(uploads[0]['if'], 'always()')
        self.assertEqual(uploads[0]['with']['path'].strip(), 'services/workflow-engine/product-browser-reports/')
        self.assertNotIn('.work', uploads[0]['with']['path'], 'private Session exchange never uploaded')

    def test_only_result_verifier_can_use_always(self):
        valid={'jobs':{'preflight':{'uses':'./.github/workflows/preflight.yml'},
          'build':{'needs':'preflight'},'selection-gate':{'needs':['preflight','build'],
          'if':'always()','steps':[{'run':'node scripts/check-ci-selection.mjs'}]}}}
        assert_gate_graph(self, valid)
        for mutate in [lambda d:d['jobs']['selection-gate']['steps'].append({'run':'pnpm build'}),
                       lambda d:d['jobs']['selection-gate'].update({'needs':['preflight']}),
                       lambda d:d['jobs']['build'].update({'if':'always()'}),
                       lambda d:d['jobs']['selection-gate'].update({'continue-on-error':'true'})]:
            bad=copy.deepcopy(valid);mutate(bad)
            with self.assertRaises(AssertionError): assert_gate_graph(self,bad)

    def test_reusable_outputs_and_release_override_are_explicit(self):
        doc = workflow(ROOT, 'preflight.yml')
        for key in ['plan','backend','browser','product']:
            self.assertEqual(doc['on']['workflow_call']['outputs'][key]['value'], '${{ jobs.preflight.outputs.'+key+' }}')
            self.assertEqual(doc['jobs']['preflight']['outputs'][key], '${{ steps.routing.outputs.'+key+' }}')
        self.assertEqual(doc['on']['workflow_call']['inputs']['force_full']['default'],'true')
        for file in ['ci.yml','acceptance.yml']:
            d=workflow(ROOT,file)
            self.assertEqual(d['on']['workflow_call']['inputs']['force_full']['default'],'true')
            self.assertIn('inputs.force_full',d['jobs']['preflight']['with']['force_full'])
            self.assertIn('pull_request',d['jobs']['preflight']['with']['force_full'])
        self.assertIn('inputs.verify_release',workflow(ROOT,'acceptance.yml')['jobs']['preflight']['with']['force_full'])
if __name__ == '__main__': unittest.main()
