#!/bin/bash
set -euo pipefail
ROOT=/workspace/scratch/75c2273f6a80
R=$ROOT/WeaveOS-worktrees/V030-068
FIX=$ROOT/app-recovery-fixtures/V030-068-backup-fixture-native
OUT=$R/docs/evidence/V030-068/backup-fixture-green
PG=$ROOT/restored-tools/pgdist/usr/lib/postgresql/18/bin
export LD_LIBRARY_PATH=$ROOT/restored-tools/pgdist/usr/lib/x86_64-linux-gnu
export PATH="$PG:$PATH"
[[ ! -e "$FIX" ]]||exit 2
mkdir "$FIX";cp "$0" "$OUT/native-runner.sh"
"$PG/initdb" -D "$FIX/pgdata" -L "$ROOT/restored-tools/pgdist/usr/share/postgresql/18" -U weaveos_test -A trust --no-locale --encoding=UTF8 > "$FIX/initdb.log"
"$PG/pg_ctl" -D "$FIX/pgdata" -l "$FIX/pg.log" -o "-h 127.0.0.1 -p 55444 -k ''" start >/dev/null
trap '"$PG/pg_ctl" -D "$FIX/pgdata" stop -m fast >/dev/null' EXIT
export PGHOST=127.0.0.1 PGPORT=55444 PGUSER=weaveos_test
python - "$R" "$FIX" > "$OUT/native-result.txt" 2>&1 <<'PY'
from pathlib import Path
import re,subprocess,sys
r=Path(sys.argv[1]);fix=Path(sys.argv[2])
for label,source in [('original',r/'docs/evidence/V030-068/backup-fixture-red/backup.test.mjs.txt'),('corrected',r/'infra/runtime/backup.test.mjs')]:
 db='weaveos_backup_fixture_'+label;subprocess.run(['createdb',db],check=True)
 files=re.findall(r"readFileSync\('(db/migrations/[^']+)'",source.read_text())
 for f in files:
  up=(r/f).read_text().split('-- +goose Down')[0]
  subprocess.run(['psql','-X','-v','ON_ERROR_STOP=1','-d',db],input='BEGIN;\n'+up+'\nCOMMIT;',text=True,check=True,capture_output=True)
 result=subprocess.run(['psql','-X','-v','ON_ERROR_STOP=1','-v','VERBOSITY=verbose','-d',db],input=(r/'infra/runtime/roles.sql').read_text(),text=True,capture_output=True)
 print(label,'migration_count',len(files),'roles_exit',result.returncode)
 if label=='original':
  assert result.returncode==3 and '42P01' in result.stderr and 'workflow_deletions' in result.stderr,result.stderr
  print(result.stderr)
 else:
  assert result.returncode==0,result.stderr
  q="SELECT has_table_privilege('auth_backup','applications.workflow_deletions','SELECT'),has_table_privilege('auth_backup','applications.workflow_deletions','DELETE'),has_table_privilege('auth_app','applications.workflow_deletions','DELETE'),has_function_privilege('auth_app','applications.complete_workflow_deletion(uuid,uuid,uuid,uuid)','EXECUTE');"
  out=subprocess.run(['psql','-X','-At','-d',db,'-c',q],capture_output=True,text=True,check=True).stdout.strip();assert out=='t|f|f|t',out;print('bounded privileges',out)
print('PASS: native PostgreSQL fixture schema and canonical role installation; not a Docker encrypted-backup run')
PY
cat "$OUT/native-result.txt"
echo 0 > "$OUT/native-exit.txt"
