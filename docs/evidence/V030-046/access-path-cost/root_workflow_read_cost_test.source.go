package apprecordservice

import (
 "context"
 "encoding/json"
 "os"
 "reflect"
 "strings"
 "testing"
 "time"
 "github.com/jackc/pgx/v5"
)

// Synthetic SQL access-path comparison, not a production latency claim.
// Only transaction-local temporary tables/indexes are mutated.
func TestRootWorkflowReadSyntheticAccessPaths(t *testing.T){
 ctx,cancel:=context.WithTimeout(context.Background(),90*time.Second);defer cancel()
 dsn:=os.Getenv("WEAVEOS_TEST_DATABASE_URL");if dsn==""{t.Fatal("isolated PostgreSQL required")}
 conn,e:=pgx.Connect(ctx,dsn);if e!=nil{t.Fatal(e)};defer conn.Close(context.Background())
 tx,e:=conn.Begin(ctx);if e!=nil{t.Fatal(e)};defer tx.Rollback(context.Background())
 if _,e=tx.Exec(ctx,`CREATE TEMP TABLE root_read_cost_instances (LIKE applications.workflow_instances INCLUDING ALL) ON COMMIT DROP;
CREATE TEMP TABLE root_read_cost_definitions(app_id uuid NOT NULL,id uuid PRIMARY KEY,name text NOT NULL) ON COMMIT DROP;
INSERT INTO root_read_cost_definitions VALUES('10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000003','Synthetic flow');
INSERT INTO root_read_cost_instances(id,app_id,table_id,flow_id,view_id,record_id,initiator_id,definition_version,state,sequence,created_at,updated_at)
SELECT md5('instance-'||n)::uuid,'10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000004',md5('record-'||(n%100))::uuid,'10000000-0000-4000-8000-000000000005',1,'completed',1,'2026-01-01'::timestamptz+n*interval '1 second','2026-01-01'::timestamptz+n*interval '1 second' FROM generate_series(1,100000)n;
ANALYZE root_read_cost_instances; ANALYZE root_read_cost_definitions;`);e!=nil{t.Fatal(e)}
 var record string;if e=tx.QueryRow(ctx,"SELECT md5('record-0')::uuid::text").Scan(&record);e!=nil{t.Fatal(e)}
 args:=[]any{"10000000-0000-4000-8000-000000000001","10000000-0000-4000-8000-000000000002",record}
 base:=strings.ReplaceAll(strings.ReplaceAll(workflowReadSelect,"applications.workflow_instances","root_read_cost_instances"),"applications.workflow_definitions","root_read_cost_definitions")+workflowReadOrder
 var expected []string
 for _,variant:=range []string{"existing","prefix","ordered"}{
  if variant!="existing"{suffix:="";if variant=="ordered"{suffix=",created_at DESC,id DESC"};if _,e=tx.Exec(ctx,"CREATE INDEX root_read_cost_candidate ON root_read_cost_instances(app_id,table_id,record_id"+suffix+")");e!=nil{t.Fatal(e)}}
  page:=base+" LIMIT 20 OFFSET 980";rows,e:=tx.Query(ctx,page,args...);if e!=nil{t.Fatal(e)};var actual []string
  for rows.Next(){var raw []byte;if e=rows.Scan(&raw);e!=nil{t.Fatal(e)};var row WorkflowInstanceSummary;if e=json.Unmarshal(raw,&row);e!=nil{t.Fatal(e)};actual=append(actual,row.ID)};if e=rows.Err();e!=nil{t.Fatal(e)};rows.Close()
  if variant=="existing"{expected=actual;if len(expected)!=20{t.Fatalf("fixture deep page length %d",len(expected))}}else if !reflect.DeepEqual(expected,actual){t.Fatal("index changed result order")}
  var size int64;if variant!="existing"{if e=tx.QueryRow(ctx,"SELECT pg_relation_size('root_read_cost_candidate')").Scan(&size);e!=nil{t.Fatal(e)}}
  for _,query:=range []struct{name,sql string}{{"first",base+" LIMIT 20"},{"deep",page},{"fingerprint",base}}{for sample:=0;sample<3;sample++{var plan []byte;if e=tx.QueryRow(ctx,"EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query.sql,args...).Scan(&plan);e!=nil{t.Fatal(e)};t.Logf("ACCESS_PATH variant=%s query=%s sample=%d rows=100000 related=1000 index_bytes=%d plan=%s",variant,query.name,sample,size,plan)}}
  if variant!="existing"{if _,e=tx.Exec(ctx,"DROP INDEX root_read_cost_candidate");e!=nil{t.Fatal(e)}}
 }
}
