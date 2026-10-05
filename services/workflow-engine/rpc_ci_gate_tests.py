"""Root-owned CI gate acceptance. Only synthetic JUnit reports, no network."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import xml.etree.ElementTree as ET
BASE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location("rpcgate",BASE/"rpc_ci_gate.py")
gate=importlib.util.module_from_spec(spec);spec.loader.exec_module(gate)
class RootGateTest(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
  self.path=Path(self.tmp.name);self.manifest=json.loads((BASE/"expected-rpc-tests.json").read_text())
  self.root=ET.Element("testsuite",name=self.manifest["classname"],tests=str(len(self.manifest["tests"])),failures="0",errors="0",skipped="0")
  for name in self.manifest["tests"]:ET.SubElement(self.root,"testcase",name=name,classname=self.manifest["classname"])
  self.go_path=self.path/"interop.jsonl"
  self.events=[{"Action":"start","Package":self.manifest["go_package"]}]
  for name in self.manifest["go_tests"]:
   self.events += [{"Action":"run","Package":self.manifest["go_package"],"Test":name},{"Action":"pass","Package":self.manifest["go_package"],"Test":name}]
  self.events.append({"Action":"pass","Package":self.manifest["go_package"]})
  self.write_go()
 def write_go(self):
  self.go_path.write_text("".join(json.dumps(e)+"\n" for e in self.events))
 def write(self):
  ET.ElementTree(self.root).write(self.path/("TEST-"+self.manifest["classname"]+".xml"))
 def check(self):
  self.write();return gate.validate(self.path,self.manifest,self.go_path)
 def test_exact_nine_and_real_interop_pass(self):
  self.assertEqual(10,self.check())
 def test_missing_report_fails(self):
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_missing_case_fails(self):
  self.root.remove(self.root[0]);self.root.set("tests","8")
  with self.assertRaises(gate.GateError):self.check()
 def test_duplicate_replacing_case_fails(self):
  self.root[1].set("name",self.root[0].get("name"))
  with self.assertRaises(gate.GateError):self.check()
 def test_wrong_class_fails(self):
  self.root[0].set("classname","other")
  with self.assertRaises(gate.GateError):self.check()
 def test_extra_report_fails(self):
  self.write();(self.path/"TEST-unexpected.xml").write_text("<testsuite/>")
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_failure_error_skip_children_fail_even_when_counts_lie(self):
  for tag in ("failure","error","skipped"):
   child=ET.SubElement(self.root[0],tag)
   with self.assertRaises(gate.GateError):self.check()
   self.root[0].remove(child)
 def test_summary_counts_must_match_cases(self):
  for key,value in (("tests","8"),("failures","1"),("errors","1"),("skipped","1"),("tests","bad")):
   previous=self.root.get(key);self.root.set(key,value)
   with self.assertRaises(gate.GateError):self.check()
   self.root.set(key,previous)
 def test_duplicate_manifest_fails(self):
  self.manifest["tests"][1]=self.manifest["tests"][0]
  with self.assertRaises(gate.GateError):self.check()
 def test_wrong_suite_fails(self):
  self.root.set("name","other")
  with self.assertRaises(gate.GateError):self.check()
 def test_malformed_xml_fails(self):
  self.write();next(self.path.glob("TEST-*.xml")).write_text("<broken")
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_doctype_fails(self):
  self.write();p=next(self.path.glob("TEST-*.xml"));p.write_text('<!DOCTYPE testsuite [<!ENTITY x "bad">]>'+p.read_text())
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_missing_go_report_fails(self):
  self.write();self.go_path.unlink()
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_empty_go_report_fails(self):
  self.write();self.go_path.write_text("")
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_go_skip_or_failure_rejected(self):
  for action in ("skip","fail"):
   self.events[2]["Action"]=action;self.write_go()
   with self.assertRaises(gate.GateError):self.check()
 def test_go_missing_test_or_package_completion_rejected(self):
  saved=copy.deepcopy(self.events)
  for index in (0,1,2,3):
   self.events=copy.deepcopy(saved);self.events.pop(index);self.write_go()
   with self.assertRaises(gate.GateError):self.check()
 def test_go_wrong_package_or_case_rejected(self):
  self.events[2]["Package"]="wrong";self.write_go()
  with self.assertRaises(gate.GateError):self.check()
  self.events[2]["Package"]=self.manifest["go_package"];self.events[2]["Test"]="Unexpected";self.write_go()
  with self.assertRaises(gate.GateError):self.check()
 def test_go_duplicate_terminal_rejected(self):
  self.events.insert(3,copy.deepcopy(self.events[2]));self.write_go()
  with self.assertRaises(gate.GateError):self.check()
 def test_go_malformed_json_rejected(self):
  self.write();self.go_path.write_text("{broken")
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest,self.go_path)
 def test_go_failure_subtest_cannot_hide_under_passing_parent(self):
  self.events[2:2]=[{"Action":"run","Package":self.manifest["go_package"],"Test":self.manifest["go_tests"][0]+"/hidden"},{"Action":"fail","Package":self.manifest["go_package"],"Test":self.manifest["go_tests"][0]+"/hidden"}];self.write_go()
  with self.assertRaises(gate.GateError):self.check()
if __name__=="__main__":unittest.main()

