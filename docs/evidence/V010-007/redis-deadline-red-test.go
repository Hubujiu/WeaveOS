package session

import (
 "context"
 "net"
 "testing"
 "time"
)

// A real blackholed TCP connection injects transport failure, not fake Redis semantics.
// Caller deadlines must bound readiness/authentication dependency checks.
func TestRedisTransportHonorsCallerDeadline(t *testing.T) {
 listener,err:=net.Listen("tcp","127.0.0.1:0")
 if err!=nil {t.Fatal(err)}
 defer listener.Close()
 done:=make(chan struct{})
 defer close(done)
 go func(){ for { connection,err:=listener.Accept(); if err!=nil{return}; go func(){defer connection.Close();<-done}() } }()
 store:=NewStore("redis://"+listener.Addr().String()+"/15","deadline-test")
 defer store.Close()
 ctx,cancel:=context.WithTimeout(context.Background(),150*time.Millisecond)
 defer cancel()
 started:=time.Now()
 if err:=store.Ping(ctx);err==nil {t.Fatal("blackholed dependency cannot be ready")}
 if elapsed:=time.Since(started);elapsed>time.Second {t.Errorf("caller deadline was ignored: elapsed %s exceeds1s safety margin",elapsed)}
}
