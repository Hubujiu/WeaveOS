import importlib.util
from pathlib import Path
import tempfile
import unittest
import zipfile

SPEC=importlib.util.spec_from_file_location('verify_sources',Path(__file__).with_name('verify_sources.py'))
module=importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(module)

class RootSchemaSourceTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name)
        # Independent official Maven resources. CI must supply these source jars,
        # not build expectations by asking the tested loader to produce them.
        paths=[('common','org/flowable/common/db/create/flowable.postgres.create.common.sql'),('engine','org/flowable/db/create/flowable.postgres.create.engine.sql'),('engine','org/flowable/db/create/flowable.postgres.create.history.sql')]
        self.expected=[]
        import os
        source=Path(os.environ.get('FLOWABLE_OFFICIAL_SOURCE_DIR','/tmp/flowable-source-review'))
        for jar,path in paths:
            with zipfile.ZipFile(source/(jar+'.jar')) as z: b=z.read(path)
            (self.root/Path(path).name).write_bytes(b);self.expected.append(b.decode('utf8'))
    def test_exact_order_and_original_bytes(self):
        self.assertEqual(module.load_verified_native_sql(self.root),tuple(self.expected))
    def test_changed_sql_rejected_without_leaking_content(self):
        p=self.root/'flowable.postgres.create.common.sql';p.write_text('SECRET BUSINESS SQL')
        with self.assertRaises(ValueError) as caught:module.load_verified_native_sql(self.root)
        self.assertNotIn('SECRET BUSINESS SQL',str(caught.exception))
    def test_missing_resource_rejected(self):
        (self.root/'flowable.postgres.create.history.sql').unlink()
        with self.assertRaises(ValueError):module.load_verified_native_sql(self.root)
    def test_unknown_version_rejected(self):
        with self.assertRaises(ValueError):module.load_verified_native_sql(self.root,'9.0.0')
    def test_symlink_resource_rejected(self):
        p=self.root/'flowable.postgres.create.engine.sql';p.rename(self.root/'elsewhere.sql');p.symlink_to('elsewhere.sql')
        with self.assertRaises(ValueError):module.load_verified_native_sql(self.root)
    def test_unrelated_files_ignored(self):
        (self.root/'unrelated.sql').write_text('DROP DATABASE unrelated;')
        self.assertEqual(module.load_verified_native_sql(self.root),tuple(self.expected))
    def test_invalid_utf8_rejected_as_safe_validation_error(self):
        (self.root/'flowable.postgres.create.engine.sql').write_bytes(b'\xff\xfe')
        with self.assertRaises(ValueError):module.load_verified_native_sql(self.root)

if __name__=='__main__':unittest.main()
