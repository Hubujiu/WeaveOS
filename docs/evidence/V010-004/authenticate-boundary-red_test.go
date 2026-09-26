package session_test

import (
 "context"
 "errors"
 "net/http"
 "net/http/httptest"
 "os"
 "testing"

 "github.com/Hubujiu/WeaveOS/services/bff/internal/session"
 "github.com/jackc/pgx/v5/pgxpool"
)

// Oracle: Redis Session §§4–7 and current auth.users.status/auth_version dictionary.
func TestAuthenticationChecksRealCurrentUserAndBoundWrites(t *testing.T) {
 ctx:=context.Background();dbURL:=os.Getenv("WEAVEOS_TEST_DATABASE_URL")
 if dbURL==""{t.Fatal("isolated migrated PostgreSQL 18 is required")}
 pool,err:=pgxpool.New(ctx,dbURL);if err!=nil{t.Fatal(err)};defer pool.Close()
 var id string
 if err:=pool.QueryRow(ctx,"INSERT INTO auth.users(account) VALUES ('SessionBoundaryUser') RETURNING id").Scan(&id);err!=nil{t.Fatal(err)}
 defer func(){_,_ =pool.Exec(ctx,"DELETE FROM auth.users WHERE id=$1",id)}()
 redisURL,generation:=isolatedRedis(t);store:=session.NewStore(redisURL,generation)
 sid,csrf,err:=store.Create(ctx,session.Record{UserID:id,SessionRef:"550e8400-e29b-41d4-a716-446655440010",AuthVersion:"1"});if err!=nil{t.Fatal(err)};defer store.Revoke(ctx,sid)
 a:=&session.Authenticator{Sessions:store,DB:pool,Origin:"https://weaveos.test"}
 req:=httptest.NewRequest("GET","https://weaveos.test/api/v1/sessions/current",nil)
 req.AddCookie(&http.Cookie{Name:"__Host-session",Value:sid});req.AddCookie(&http.Cookie{Name:"__Host-csrf",Value:csrf})
 req.Header.Set("Origin","https://weaveos.test");req.Header.Set("X-CSRF-Token",csrf);req.Header.Set("X-User-Id","forged")
 p,err:=a.Authenticate(req,false)
 if err!=nil || p.UserID!=id || p.SessionRef=="" || p.BootstrapAdmin {t.Fatalf("valid real session must authenticate exact PG user, ignoring external identity: %v",err)}
 if _,err:=a.Authenticate(req,true);err!=nil{t.Fatalf("bound write rejected: %v",err)}
 req.Header.Del("X-CSRF-Token")
 if _,err:=a.Authenticate(req,true);!errors.Is(err,session.ErrForbidden){t.Fatalf("missing CSRF must forbid: %v",err)}
 req.Header.Set("X-CSRF-Token",csrf);req.Header.Set("Origin","https://evil.test")
 if _,err:=a.Authenticate(req,true);!errors.Is(err,session.ErrForbidden){t.Fatalf("hostile Origin must forbid: %v",err)}
 req.Header.Set("Origin","https://weaveos.test")
 if _,err:=pool.Exec(ctx,"UPDATE auth.users SET status='disabled',auth_version=auth_version+1 WHERE id=$1",id);err!=nil{t.Fatal(err)}
 if _,err:=a.Authenticate(req,false);!errors.Is(err,session.ErrUnauthorized){t.Fatalf("disabled current user must invalidate existing session: %v",err)}
 if _,err:=pool.Exec(ctx,"UPDATE auth.users SET status='active',auth_version=auth_version+1 WHERE id=$1",id);err!=nil{t.Fatal(err)}
 if _,err:=a.Authenticate(req,false);!errors.Is(err,session.ErrUnauthorized){t.Fatalf("reenabling must not restore stale version: %v",err)}
 sid2,csrf2,err:=store.Create(ctx,session.Record{UserID:id,SessionRef:"550e8400-e29b-41d4-a716-446655440011",AuthVersion:"3"});if err!=nil{t.Fatal(err)};defer store.Revoke(ctx,sid2)
 fresh:=httptest.NewRequest("GET","https://weaveos.test/api/v1/sessions/current",nil)
 fresh.AddCookie(&http.Cookie{Name:"__Host-session",Value:sid2});fresh.AddCookie(&http.Cookie{Name:"__Host-csrf",Value:csrf2})
 current,err:=a.Authenticate(fresh,false);if err!=nil{t.Fatalf("current version must work: %v",err)}
 key,err:=keyFor(generation,sid2);if err!=nil{t.Fatal(err)}
 redisCommand(t,redisURL,"PEXPIRE",key,"5000")
 w:=httptest.NewRecorder();if err:=a.Renew(ctx,w,fresh,current);err!=nil || len(w.Result().Cookies())!=2{t.Fatalf("successful activity must synchronize both cookies: %v",err)}
 if ttl:=redisCommand(t,redisURL,"PTTL",key).(int64);ttl<3590000 || ttl>3600000{t.Errorf("success must slide real Redis TTL, got %d",ttl)}
 foreign:=&session.Authenticator{Sessions:session.NewStore(redisURL,generation+"-restored"),DB:pool,Origin:a.Origin}
 if _,err:=foreign.Authenticate(fresh,false);!errors.Is(err,session.ErrUnauthorized){t.Fatalf("restored generation must reject old cookies: %v",err)}
 unavailable:=&session.Authenticator{Sessions:session.NewStore("redis://127.0.0.1:1/15",generation),DB:pool,Origin:a.Origin}
 if _,err:=unavailable.Authenticate(fresh,false);!errors.Is(err,session.ErrUnavailable){t.Fatalf("Redis unavailable must fail closed as a system error: %v",err)}
 if _,err:=store.Revoke(ctx,sid2);err!=nil{t.Fatal(err)}
 if err:=a.Renew(ctx,httptest.NewRecorder(),fresh,current);!errors.Is(err,session.ErrUnauthorized){t.Fatalf("renew must not revive revoked session: %v",err)}
 pool.Close()
 if _,err:=a.Authenticate(req,false);!errors.Is(err,session.ErrUnavailable){t.Fatalf("PG dependency failure must be a system error: %v",err)}
}

func TestUninitializedSessionStoreFailsClosed(t *testing.T) {
 var store session.Store
 if _,err:=store.Load(context.Background(),"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");err==nil{t.Fatal("uninitialized store must return an error")}
}
