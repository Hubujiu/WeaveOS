package auth

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type queryBarrier struct {
	reached, release chan struct{}
	once             sync.Once
}
type barrierKey struct{}

func (b *queryBarrier) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if strings.Contains(d.SQL, "GetLoginRecord") && len(d.Args) == 1 && d.Args[0] == "member" {
		return context.WithValue(ctx, barrierKey{}, true)
	}
	return ctx
}
func (b *queryBarrier) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if matched, _ := ctx.Value(barrierKey{}).(bool); matched {
		b.once.Do(func() { close(b.reached) })
		select {
		case <-b.release:
		case <-ctx.Done():
		}
	}
}

// DDL §5: verifying a pre-reset hash cannot yield a post-reset auth_version Session.
// The tracer pauses only after a real PostgreSQL credential read; it does not mock data.
func TestPasswordResetBetweenCredentialReadAndSessionCreation(t *testing.T) {
	a := setup(t)
	a.user(t, "admin", true)
	id := a.user(t, "member", false)
	admin := a.login(t, "admin")
	barrier := &queryBarrier{reached: make(chan struct{}), release: make(chan struct{})}
	cfg, err := pgxpool.ParseConfig(os.Getenv("WEAVEOS_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Tracer = barrier
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { close(barrier.release); pool.Close() }()
	a.service.Pool = pool
	result := make(chan int, 1)
	go func() {
		result <- a.request("POST", "/api/v1/sessions", map[string]string{"account": "member", "password": "Aa1!"}, nil, nil).Code
	}()
	select {
	case <-barrier.reached:
	case <-time.After(5 * time.Second):
		t.Fatal("real credential read did not reach barrier")
	}
	checkStatus(t, a.request("POST", "/api/v1/users/"+id+"/password-reset", map[string]string{}, admin, nil), 200)
	// Release through a separate signal to preserve one deferred channel close.
	barrier.release <- struct{}{}
	select {
	case status := <-result:
		if status != 401 {
			t.Fatalf("old credential snapshot login=%d want401", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("login failed to complete after reset")
	}
}
