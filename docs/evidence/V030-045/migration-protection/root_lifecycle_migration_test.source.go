package apprecordservice

import (
 "context"
 "errors"
 "os"
 "strings"
 "testing"
 "time"

 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgconn"
)

// Exercise the exact 022 statements against a private clone of the real
// operations constraints. Never roll back the shared migrated test database.
func rootLifecycleMigrationFixture(t *testing.T) (context.Context, *pgx.Conn, string, string, string) {
 t.Helper()
 ctx,cancel:=context.WithTimeout(context.Background(),20*time.Second);t.Cleanup(cancel)
 dsn:=os.Getenv("WEAVEOS_TEST_DATABASE_URL");if dsn==""{t.Fatal("isolated PostgreSQL required")}
 c,e:=pgx.Connect(ctx,dsn);if e!=nil{t.Fatal(e)};t.Cleanup(func(){_ = c.Close(context.Background())})
 var id string;if e=c.QueryRow(ctx,"SELECT gen_random_uuid()::text").Scan(&id);e!=nil{t.Fatal(e)}
 schema:=pgx.Identifier{"root_lifecycle_down_"+strings.ReplaceAll(id,"-","")}.Sanitize()
 if _,e=c.Exec(ctx,"CREATE SCHEMA "+schema+"; CREATE TABLE "+schema+".operations (LIKE applications.operations INCLUDING ALL)");e!=nil{t.Fatal(e)}
 t.Cleanup(func(){_,_ = c.Exec(context.Background(),"DROP SCHEMA "+schema+" CASCADE")})
 raw,e:=os.ReadFile("../../../../db/migrations/00022_workflow_lifecycle_operations.sql");if e!=nil{t.Fatal(e)}
 parts:=strings.Split(string(raw),"-- +goose Down");if len(parts)!=2{t.Fatal("single exact Down section required")}
 return ctx,c,schema,strings.ReplaceAll(parts[0],"applications.",schema+"."),strings.ReplaceAll(parts[1],"applications.",schema+".")
}
func rootLifecycleMigrationApply(ctx context.Context,c *pgx.Conn,sql string) error {
 tx,e:=c.Begin(ctx);if e!=nil{return e};defer tx.Rollback(context.Background())
 if _,e=tx.Exec(ctx,sql);e!=nil{return e};return tx.Commit(ctx)
}
func rootLifecycleMigrationInsertSQL(schema string) string {
 return "INSERT INTO "+schema+".operations(actor_user_id,operation_id,app_id,operation_kind,fingerprint) VALUES(gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),$1,decode(repeat('ab',32),'hex'))"
}
func TestRootLifecycleMigrationEmptyDownUpAndHistoryProtection(t *testing.T) {
 ctx,c,schema,up,down:=rootLifecycleMigrationFixture(t)
 if e:=rootLifecycleMigrationApply(ctx,c,down);e!=nil{t.Fatal(e)}
 var pg *pgconn.PgError
 _,e:=c.Exec(ctx,rootLifecycleMigrationInsertSQL(schema),"workflow.task.return")
 if !errors.As(e,&pg)||pg.Code!="23514"{t.Fatalf("Down must restore 021 kind rejection: %v",e)}
 if _,e=c.Exec(ctx,rootLifecycleMigrationInsertSQL(schema),"workflow.task.agree");e!=nil{t.Fatal("Down broke prior agree kind",e)}
 if e=rootLifecycleMigrationApply(ctx,c,up);e!=nil{t.Fatal(e)}
 for _,kind:=range []string{"workflow.instance.withdraw","workflow.task.return"}{if _,e=c.Exec(ctx,rootLifecycleMigrationInsertSQL(schema),kind);e!=nil{t.Fatal(e)}}
 e=rootLifecycleMigrationApply(ctx,c,down);pg=nil
 if !errors.As(e,&pg)||pg.Code!="55000"{t.Fatalf("history must block Down: %v",e)}
 var n int;if e=c.QueryRow(ctx,"SELECT count(*) FROM "+schema+".operations").Scan(&n);e!=nil||n!=3{t.Fatalf("rollback changed history: %d %v",n,e)}
 if _,e=c.Exec(ctx,rootLifecycleMigrationInsertSQL(schema),"workflow.task.return");e!=nil{t.Fatal("failed Down changed constraints",e)}
}
func TestRootLifecycleMigrationDownSeesConcurrentHistory(t *testing.T) {
 ctx,c,schema,_,down:=rootLifecycleMigrationFixture(t)
 other,e:=pgx.Connect(ctx,os.Getenv("WEAVEOS_TEST_DATABASE_URL"));if e!=nil{t.Fatal(e)};defer other.Close(context.Background())
 accepted,e:=c.Begin(ctx);if e!=nil{t.Fatal(e)};defer accepted.Rollback(context.Background())
 if _,e=accepted.Exec(ctx,rootLifecycleMigrationInsertSQL(schema),"workflow.instance.withdraw");e!=nil{t.Fatal(e)}
 var pid int;if e=other.QueryRow(ctx,"SELECT pg_backend_pid()").Scan(&pid);e!=nil{t.Fatal(e)}
 result:=make(chan error,1);go func(){result<-rootLifecycleMigrationApply(ctx,other,down)}()
 blocked:=false
 for !blocked {
  select{case e=<-result:t.Fatalf("Down did not wait for concurrent history: %v",e);case <-ctx.Done():t.Fatal("Down did not reach lock wait");default:}
  if e=accepted.QueryRow(ctx,"SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND NOT granted)",pid).Scan(&blocked);e!=nil{t.Fatal(e)}
  if !blocked{time.Sleep(10*time.Millisecond)}
 }
 if e=accepted.Commit(ctx);e!=nil{t.Fatal(e)}
 select{case e=<-result:case <-ctx.Done():t.Fatal("Down did not finish after acceptance")}
 var pg *pgconn.PgError;if !errors.As(e,&pg)||pg.Code!="55000"{t.Fatalf("must protect newly committed history: %v",e)}
 var n int;if e=c.QueryRow(ctx,"SELECT count(*) FROM "+schema+".operations").Scan(&n);e!=nil||n!=1{t.Fatalf("accepted history lost: %d %v",n,e)}
}
