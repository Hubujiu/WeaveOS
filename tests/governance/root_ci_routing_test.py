"""V059 independent scheduler output wiring; does not emulate GitHub's scheduler."""
import unittest
from root_ci_throughput_test import workflow, ROOT
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
                lane = 'product' if file == 'acceptance.yml' else 'browser' if name == 'browser' else 'backend'
                self.assertEqual(job['if'], "needs.preflight.outputs." + lane + " == 'true'")
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
