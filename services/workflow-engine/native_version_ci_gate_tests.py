"""Root-owned independent report acceptance; no implementation-generated expectations."""
import tempfile
import unittest
from pathlib import Path
import xml.etree.ElementTree as ET
from native_version_ci_gate import GateError, validate
CLASS = 'org.weaveos.workflow.RootNativeVersioningTest'
NAMES = (
 'scopedPublicationsUseNativeVersionsAndKeepRunningDefinition',
 'sameFlowInDifferentAppsHasIndependentNativeSequence',
 'sameScopedPublicationReplaysAcrossEngineReopen',
 'scopedKeyMustMatchAppAndFlowAndRejectUnsafeXml',
 'legacyToScopedKeepsOriginalDefinitionAndBusinessRevision',
 'suspendingDefinitionLeavesInstancesRunnableAndHistoryReadable',
 'concurrentScopedReplayCommitsOneDefinition',
 'scopedFailureRollsBackLedgerAndNativeVersion',
)
class Acceptance(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
  self.path=Path(self.temp.name)/f'TEST-{CLASS}.xml'
  self.root=ET.Element('testsuite',name=CLASS,tests='8',failures='0',errors='0',skipped='0')
  for name in NAMES: ET.SubElement(self.root,'testcase',classname=CLASS,name=name)
 def write(self): self.path.write_bytes(ET.tostring(self.root))
 def reject(self):
  self.write()
  with self.assertRaises(GateError): validate(self.temp.name)
 def test_complete(self): self.write();self.assertEqual(8,validate(self.temp.name))
 def test_missing(self): self.root.remove(self.root[-1]);self.root.set('tests','7');self.reject()
 def test_extra(self): ET.SubElement(self.root,'testcase',classname=CLASS,name='extra');self.root.set('tests','9');self.reject()
 def test_skipped(self): ET.SubElement(self.root[0],'skipped');self.root.set('skipped','1');self.reject()
 def test_failure(self): ET.SubElement(self.root[0],'failure');self.root.set('failures','1');self.reject()
 def test_wrong_suite(self): self.root.set('name','Other');self.reject()
 def test_duplicate(self): self.root[-1].set('name',NAMES[0]);self.reject()
 def test_miscount(self): self.root.set('tests','9');self.reject()
 def test_doctype(self):
  self.write();self.path.write_bytes(b'<!DOCTYPE testsuite>'+self.path.read_bytes())
  with self.assertRaises(GateError): validate(self.temp.name)
if __name__ == '__main__': unittest.main()
