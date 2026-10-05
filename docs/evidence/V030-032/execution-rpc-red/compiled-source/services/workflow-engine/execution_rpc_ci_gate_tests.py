"""Root-owned acceptance of exact execution RPC evidence, synthetic inputs only."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import xml.etree.ElementTree as ET
BASE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('exec_rpc_gate',BASE/'execution_rpc_ci_gate.py')
gate=importlib.util.module_from_spec(spec);spec.loader.exec_module(gate)
class RootExecutionRpcGateTest(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.path=Path(self.temp.name)
  self.manifest=json.loads((BASE/'expected-execution-rpc-tests.json').read_text())
  self.root=ET.Element('testsuite',name=self.manifest['classname'],tests='11',failures='0',errors='0',skipped='0')
  for name in self.manifest['tests']: ET.SubElement(self.root,'testcase',name=name,classname=self.manifest['classname'])
  self.report=self.path/('TEST-'+self.manifest['classname']+'.xml')
  self.unit=self.path/'unit.jsonl';self.interop=self.path/'interop.jsonl';self.write_java()
  self.unit_events=self.events(self.manifest['go_unit_tests']);self.interop_events=self.events(self.manifest['go_interop_tests']);self.write_go()
 def events(self,names):
  p=self.manifest['go_package'];out=[{'Action':'start','Package':p}]
  for n in names:out.extend([{'Action':'run','Package':p,'Test':n},{'Action':'pass','Package':p,'Test':n}])
  return out+[{'Action':'pass','Package':p}]
 def write_java(self):ET.ElementTree(self.root).write(self.report)
 def write_go(self):
  for path,events in ((self.unit,self.unit_events),(self.interop,self.interop_events)):path.write_text(''.join(json.dumps(e)+'\n' for e in events))
 def check(self):return gate.validate(self.path,self.manifest,self.unit,self.interop)
 def test_exact_twenty_four_cases_pass(self):self.assertEqual(24,self.check())
 def test_missing_each_report_fails(self):
  for p in (self.report,self.unit,self.interop):
   saved=p.read_bytes();p.unlink()
   with self.assertRaises(gate.GateError):self.check()
   p.write_bytes(saved)
 def test_java_missing_case_fails(self):
  self.root.remove(self.root[0]);self.root.set('tests','10');self.write_java()
  with self.assertRaises(gate.GateError):self.check()
 def test_java_replaced_case_fails(self):
  self.root[0].set('name','unexpected');self.write_java()
  with self.assertRaises(gate.GateError):self.check()
 def test_java_skip_failure_error_cannot_hide(self):
  for tag in ('skipped','failure','error'):
   child=ET.SubElement(self.root[0],tag);self.write_java()
   with self.assertRaises(gate.GateError):self.check()
   self.root[0].remove(child)
 def test_java_counts_cannot_lie(self):
  self.root.set('failures','1');self.write_java()
  with self.assertRaises(gate.GateError):self.check()
 def test_java_extra_suite_fails(self):
  (self.path/'TEST-unexpected.xml').write_text('<testsuite/>')
  with self.assertRaises(gate.GateError):self.check()
 def test_java_doctype_fails(self):
  self.report.write_text('<!DOCTYPE testsuite [<!ENTITY x "bad">]>'+self.report.read_text())
  with self.assertRaises(gate.GateError):self.check()
 def test_each_go_report_requires_all_cases(self):
  for events in (self.unit_events,self.interop_events):
   saved=copy.deepcopy(events);del events[1:3];self.write_go()
   with self.assertRaises(gate.GateError):self.check()
   events[:]=saved
 def test_each_go_report_requires_completion(self):
  for events in (self.unit_events,self.interop_events):
   last=events.pop();self.write_go()
   with self.assertRaises(gate.GateError):self.check()
   events.append(last)
 def test_go_failure_or_skip_fails(self):
  for events in (self.unit_events,self.interop_events):
   for action in ('fail','skip'):
    events[2]['Action']=action;self.write_go()
    with self.assertRaises(gate.GateError):self.check()
   events[2]['Action']='pass'
 def test_go_duplicate_terminal_fails(self):
  self.unit_events.insert(3,copy.deepcopy(self.unit_events[2]));self.write_go()
  with self.assertRaises(gate.GateError):self.check()
 def test_go_wrong_package_fails(self):
  self.interop_events[2]['Package']='wrong';self.write_go()
  with self.assertRaises(gate.GateError):self.check()
 def test_renaming_expected_case_and_report_together_fails(self):
  initial_manifest=copy.deepcopy(self.manifest);initial_root=copy.deepcopy(self.root)
  initial_unit=copy.deepcopy(self.unit_events);initial_interop=copy.deepcopy(self.interop_events)
  for key in ('tests','go_unit_tests','go_interop_tests'):
   self.manifest=copy.deepcopy(initial_manifest);self.root=copy.deepcopy(initial_root)
   self.unit_events=copy.deepcopy(initial_unit);self.interop_events=copy.deepcopy(initial_interop)
   self.manifest[key][0]='TestRootFakeCase'
   if key=='tests':self.root[0].set('name','TestRootFakeCase')
   else:
    events=self.unit_events if key=='go_unit_tests' else self.interop_events
    events[1]['Test']='TestRootFakeCase';events[2]['Test']='TestRootFakeCase'
   self.write_java();self.write_go()
   with self.assertRaises(gate.GateError):self.check()
 def test_no_optional_skip_or_empty_manifest(self):
  for manifest in ({},dict(self.manifest,tests=[]),dict(self.manifest,go_unit_tests=[]),dict(self.manifest,go_interop_tests=[])):
   with self.assertRaises(gate.GateError):gate.validate(self.path,manifest,self.unit,self.interop)
 def test_malformed_json_and_xml_fail(self):
  for path,bad in ((self.unit,'{bad'),(self.interop,'{bad'),(self.report,'<bad')):
   saved=path.read_text();path.write_text(bad)
   with self.assertRaises(gate.GateError):self.check()
   path.write_text(saved)
if __name__=='__main__':unittest.main()
