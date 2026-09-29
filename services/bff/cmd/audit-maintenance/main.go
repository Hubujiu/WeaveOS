package main

import (
	"context"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/audit"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	fail := func() { logger.Error("audit maintenance failed"); os.Exit(1) }
	if len(os.Args) != 2 || os.Args[1] != "--once" {
		fail()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pools := make([]*pgxpool.Pool, 0, 2)
	for _, key := range []string{"WEAVEOS_AUDIT_LIVE_DATABASE_URL", "WEAVEOS_AUDIT_COLD_DATABASE_URL"} {
		uri := os.Getenv(key)
		if uri == "" {
			fail()
		}
		cfg, err := pgxpool.ParseConfig(uri)
		if err != nil {
			fail()
		}
		cfg.MaxConns = 4
		cfg.ConnConfig.ConnectTimeout = 2 * time.Second
		cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			fail()
		}
		defer p.Close()
		pools = append(pools, p)
	}
	report := func(r audit.Result, err error) {
		if err != nil {
			logger.Error("audit maintenance failed")
		} else {
			logger.Info("audit maintenance complete", "archived", r.Archived, "expired", r.Expired)
		}
	}
	operation, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	r, err := audit.Maintain(operation, pools[0], pools[1], time.Now().UTC())
	report(r, err)
	if err != nil {
		os.Exit(1)
	}
}
