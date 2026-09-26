package session

import (
 "context"
 "os"
 "testing"
)

func TestLifecycleUsesRealRedisAndClosesClient(t *testing.T) {
 url:=os.Getenv("WEAVEOS_TEST_REDIS_URL");if url=="" {t.Fatal("isolated Redis URL required")}
 store:=NewStore(url,"lifecycle")
 if err:=store.Ping(context.Background());err!=nil {t.Fatal("healthy real Redis must ping successfully")}
 if err:=store.Close();err!=nil {t.Fatal("client close must succeed")}
 if store.Ping(context.Background())==nil {t.Fatal("closed Redis client must fail readiness")}
}
