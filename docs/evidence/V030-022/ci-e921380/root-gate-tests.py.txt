"""Root-owned CI gate acceptance. Only synthetic JUnit reports, no network."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import xml.etree.ElementTree as ET
BASE=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location("gate",BASE/"ci_gate.py")
gate=importlib.util.module_from_spec(spec);spec.loader.exec_module(gate)
class RootGateTest(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
  self.path=Path(self.tmp.name);self.manifest=json.loads((BASE/"expected-tests.json").read_text())
  self.root=ET.Element("testsuite",name=self.manifest["classname"],tests=str(len(self.manifest["tests"])),failures="0",errors="0",skipped="0")
  for name in self.manifest["tests"]:ET.SubElement(self.root,"testcase",name=name,classname=self.manifest["classname"])
 def write(self):
  ET.ElementTree(self.root).write(self.path/("TEST-"+self.manifest["classname"]+".xml"))
 def check(self):
  self.write();return gate.validate(self.path,self.manifest)
 def test_exact_eighteen_pass(self):
  self.assertEqual(18,self.check())
 def test_missing_report_fails(self):
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest)
 def test_missing_case_fails(self):
  self.root.remove(self.root[0]);self.root.set("tests","17")
  with self.assertRaises(gate.GateError):self.check()
 def test_duplicate_replacing_case_fails(self):
  self.root[1].set("name",self.root[0].get("name"))
  with self.assertRaises(gate.GateError):self.check()
 def test_wrong_class_fails(self):
  self.root[0].set("classname","other")
  with self.assertRaises(gate.GateError):self.check()
 def test_extra_report_fails(self):
  self.write();(self.path/"TEST-unexpected.xml").write_text("<testsuite/>")
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest)
 def test_failure_error_skip_children_fail_even_when_counts_lie(self):
  for tag in ("failure","error","skipped"):
   child=ET.SubElement(self.root[0],tag)
   with self.assertRaises(gate.GateError):self.check()
   self.root[0].remove(child)
 def test_summary_counts_must_match_cases(self):
  for key,value in (("tests","17"),("failures","1"),("errors","1"),("skipped","1"),("tests","bad")):
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
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest)
 def test_doctype_fails(self):
  self.write();p=next(self.path.glob("TEST-*.xml"));p.write_text('<!DOCTYPE testsuite [<!ENTITY x "bad">]>'+p.read_text())
  with self.assertRaises(gate.GateError):gate.validate(self.path,self.manifest)
if __name__=="__main__":unittest.main()
