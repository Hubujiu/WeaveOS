import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

test('restricted backup fixture builds the complete reviewed schema before installing canonical roles',()=>{
 const source=readFileSync(new URL('../../infra/runtime/backup.test.mjs',import.meta.url),'utf8');
 const install=source.indexOf("sql(live,readFileSync('infra/runtime/roles.sql'");
 assert.ok(install>0);
 const migrations=[
  '00022_workflow_lifecycle_operations.sql',
  '00023_workflow_record_order_index.sql',
  '00024_workflow_trigger_configuration.sql',
  '00025_workflow_manual_start_operations.sql',
  '00026_workflow_personal_inbox_index.sql',
  '00027_workflow_independent_journal.sql',
  '00028_workflow_publication_history.sql',
  '00029_workflow_deletions.sql',
 ];
 let previous=source.indexOf('00021_workflow_task_operations.sql');
 for(const file of migrations){
  const index=source.indexOf(file);
  assert.ok(index>previous&&index<install,`${file} must be applied in order before canonical roles`);
  previous=index;
 }
});
