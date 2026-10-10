// Root-owned immutable-evidence migration registration. No production upgrade.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {validateInstalledPersonnelRoles} from '../../infra/server/deploy/personnel-upgrade.mjs';
const read=p=>readFileSync(new URL('../../'+p,import.meta.url));
const hash=b=>createHash('sha256').update(b).digest('hex');
const manifest=JSON.parse(read('infra/server/deploy/compatibility.json'));
const prior=[
  {
    "path": "migrations/00001_auth.sql",
    "sha256": "ab5c96db2fb775501df3fc2c55a16f9ffd6c8630c6e9f875aa238230b6b522b4"
  },
  {
    "path": "archive-migrations/00001_archive.sql",
    "sha256": "8896aa6c3a43d661425fefb5dff001222bef827cd8e4b5093ae409ac4179db2e"
  },
  {
    "path": "migrations/00002_personnel.sql",
    "sha256": "544512f1654ae0a96f51e0fa24f61b58e3f15c8a2e3fa2928b0dd969e5a1b91e"
  },
  {
    "path": "archive-migrations/00002_personnel_audit.sql",
    "sha256": "7276893525b0a06176f63a1845287182dc7ec2589952849789ab9490b975c995"
  },
  {
    "path": "migrations/00003_query_drafts.sql",
    "sha256": "9b1f546d158931d09245070a4d5b6b46dd64b128b008363ae7a44f74bae8860a"
  },
  {
    "path": "migrations/00004_query_revision_writers.sql",
    "sha256": "85faf7406d0e82ce7ca14aaf10276f80ef5a88a5c957170f111a261e910c925d"
  },
  {
    "path": "migrations/00005_table_presets.sql",
    "sha256": "9f8ec8b86e7d7ab208940b88f43283ca11895a34e18b3af328e25b1e8ba40509"
  },
  {
    "path": "archive-migrations/00003_apps_audit.sql",
    "sha256": "70c8493acefb442c354f412ddf25b140f8361f930353fcf4cd342d257db0f4ab"
  },
  {
    "path": "migrations/00006_apps_policy.sql",
    "sha256": "1cca96dc6e5b7acf11c0c9f40364d1fdb5d9d57b4f8ddd8b0036dd5b5bf15fb0"
  },
  {
    "path": "archive-migrations/00004_app_structure_audit.sql",
    "sha256": "8e6ff898386db6ec02a09a33c46b24d3787293d21eb4246020740825a5a49a16"
  },
  {
    "path": "migrations/00007_app_structure.sql",
    "sha256": "080b6daf0aacf0763d54dfadcbcffc2135486835700895b08010c77cb5d12531"
  },
  {
    "path": "migrations/00008_app_records.sql",
    "sha256": "3df7c46c65744b6bf42ba37e625e8fa2d865806ed139e03306d5a8a2b42d63b0"
  },
  {
    "path": "migrations/00009_record_save_history.sql",
    "sha256": "6468e6f8bf0e1d349840238c5fa44a316abf9ad53cf46055b6a9b025bdf8bed1"
  },
  {
    "path": "migrations/00010_member_source_label_width.sql",
    "sha256": "326d7d0f171dba8350f7e8fbd1f96c5182235c969d73b4f0a585571c7b8a9a51"
  },
  {
    "path": "migrations/00011_record_reference_defaults.sql",
    "sha256": "d15de1b9265a7d8da2dab448cfe2ba026472a15a08c0163f5e60f237b1c1e9f8"
  },
  {
    "path": "migrations/00012_actual_reference_defaults.sql",
    "sha256": "450b3e8aa0bd1f2419915074a32d1d8d47ce5fe2b82c891f4f058a1a429e3490"
  },
  {
    "path": "migrations/00013_record_command_fence.sql",
    "sha256": "2e75231b237d8e61cb3a291cbbe144cfa1e10ee0a7090e96a0c02f61ff0e0498"
  },
  {
    "path": "migrations/00014_workflow_command_ledger.sql",
    "sha256": "c54a0414b1beb9ea835bf830cc7a735431d6d816059f22821ffab5e87e37f2cb"
  },
  {
    "path": "migrations/00015_workflow_catalog.sql",
    "sha256": "5056a36d4f9ed64fdd98432f231ec67012cde693439ac75139e595b51f0911fc"
  },
  {
    "path": "archive-migrations/00005_workflow_management_audit.sql",
    "sha256": "f2fced03856ceca324d73946e892d7f95a9140a0221f5ea7a38a070a84b83db3"
  },
  {
    "path": "migrations/00016_workflow_management_operations.sql",
    "sha256": "f68d650947d665a7ccb905cd5c7d0d85dfe15b4dcc501c00066cd38c00b71929"
  },
  {
    "path": "archive-migrations/00006_workflow_publication_audit.sql",
    "sha256": "71f6539e37b16b9baba3531449faa7fc694de713282b097c56e472e6d470005c"
  },
  {
    "path": "migrations/00017_workflow_publications.sql",
    "sha256": "26dc61e4b3811862b66fd5db5ad0bff872e20284c83159744c3eb280486eb4fb"
  },
  {
    "path": "migrations/00018_workflow_execution_projection.sql",
    "sha256": "70b73412bccd5b95ede435f8c1142496dd0c4c53e60163e15a9591b784527f6b"
  },
  {
    "path": "migrations/00019_workflow_execution_recovery.sql",
    "sha256": "c7fc65a5b0437e73245490804237fe3189428a33e4329f40c16b45aa9e2d3d34"
  }
];
const path='migrations/00020_workflow_evidence.sql';
const expected='6699d65269ddbeeddc429486038865e09b35a857b407639487ade4aaaf8cb4d9';
test('Root V036: one reviewed evidence migration follows recovery hot19',()=>{
 assert.deepEqual(manifest.migrations.filter(x=>x.path===path),[{path,sha256:expected}]);
 const paths=manifest.migrations.map(x=>x.path);assert.ok(paths.indexOf(path)>paths.indexOf('migrations/00019_workflow_execution_recovery.sql'));
 assert.equal(hash(read('db/'+path)),expected);
});
test('Root V036: every older migration identity and source remains unchanged',()=>{
 for(const old of prior){assert.deepEqual(manifest.migrations.filter(x=>x.path===old.path),[old]);assert.equal(hash(read('db/'+old.path)),old.sha256);}
});
test('Root V036: only reviewed append-only evidence roles are admitted',()=>{
 const sql=read('infra/runtime/roles.sql').toString('utf8');assert.equal(hash(sql),'8b0e70f9a3708039d87d7848c0fba43d3ba240f018080c6a5d3a997efecf395f');
 assert.doesNotThrow(()=>validateInstalledPersonnelRoles(sql));assert.doesNotThrow(()=>validateInstalledPersonnelRoles(sql.replace(/\n/g,'\r\n')));
 for(const extra of ['GRANT UPDATE ON applications.workflow_evidence_blobs TO auth_app;','GRANT DELETE ON applications.workflow_evidence_documents TO auth_backup;','GRANT SELECT ON applications.workflow_evidence_members TO auth_reader;'])assert.throws(()=>validateInstalledPersonnelRoles(sql+'\n'+extra+'\n'));
});
test('Root V036: encrypted backup fixture includes evidence migration and original wire vectors',()=>{
 const source=read('infra/runtime/backup.test.mjs').toString('utf8');
 assert.ok(source.includes("readFileSync('db/migrations/00020_workflow_evidence.sql'"));
 assert.ok(source.includes('root-evidence-vectors.json'));
 for(const table of ['workflow_evidence_blobs','workflow_evidence_documents','workflow_evidence_members'])assert.ok(source.includes('INSERT INTO applications.'+table));
});
test('Root V036: legacy fixture resets enumerate evidence dependencies without CASCADE',()=>{
 for(const path of ['services/bff/internal/applications/set_cost_test.go','services/bff/internal/audit/maintenance_test.go']){
  const source=read(path).toString('utf8');const lines=source.split('\n').filter(x=>x.includes('TRUNCATE applications.'));assert.equal(lines.length,1);
  for(const table of ['workflow_evidence_members','workflow_evidence_documents','workflow_evidence_blobs'])assert.ok(lines[0].includes('applications.'+table));
  assert.ok(!/\bCASCADE\b/i.test(lines[0]));
 }
});
