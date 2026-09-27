package main

import (
	"context"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/audit"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	fail := func() { logger.Error("audit read refused"); os.Exit(1) }
	uri := os.Getenv("WEAVEOS_AUDIT_READ_DATABASE_URL")
	redisURL := os.Getenv("WEAVEOS_REDIS_URL")
	generation := os.Getenv("WEAVEOS_SESSION_GENERATION")
	if uri == "" || redisURL == "" || generation == "" || len(os.Args) != 1 {
		fail()
	}
	cfg, err := pgxpool.ParseConfig(uri)
	if err != nil {
		fail()
	}
	cfg.MaxConns = 4
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fail()
	}
	defer pool.Close()
	store := session.NewStore(redisURL, generation)
	defer store.Close()
	// Read the current Session credential privately from stdin, never arguments,
	// logs or identity headers. Output is sensitive and only for the verified admin.
	input, err := io.ReadAll(io.LimitReader(os.Stdin, 128))
	if err != nil {
		fail()
	}
	request, err := http.NewRequestWithContext(ctx, "GET", "https://localhost/audit-cli", nil)
	if err != nil {
		fail()
	}
	request.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: strings.TrimSpace(string(input))})
	reader := audit.Reader{Authentication: &session.Authenticator{Sessions: store, DB: pool}, Live: pool}
	events, err := reader.List(request, 100)
	if err != nil {
		fail()
	}
	if err := json.NewEncoder(os.Stdout).Encode(events); err != nil {
		fail()
	}
}
