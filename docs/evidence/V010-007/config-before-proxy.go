package main

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/auth"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
)

type config struct {
	DatabaseURL, RedisURL, Origin, Generation, AuditKeyID string
	AuditKey                                              []byte
	// Declaration only for compiling the configuration test-first snapshot.
	TrustedProxyHosts                                     []string
}

func readConfig(get func(string) string) (config, error) {
	cfg := config{DatabaseURL: get("WEAVEOS_DATABASE_URL"), RedisURL: get("WEAVEOS_REDIS_URL"), Origin: get("WEAVEOS_PUBLIC_ORIGIN"), Generation: get("WEAVEOS_SESSION_GENERATION"), AuditKeyID: get("WEAVEOS_AUDIT_KEY_ID")}
	if raw := get("WEAVEOS_AUDIT_HMAC_KEY"); raw != "" {
		key, err := base64.StdEncoding.Strict().DecodeString(raw)
		if err != nil {
			return config{}, errors.New("invalid audit key encoding")
		}
		cfg.AuditKey = key
	}
	return cfg, nil
}
func buildHandler(ctx context.Context, cfg config) (http.Handler, func(), error) {
	if cfg.DatabaseURL == "" && cfg.RedisURL == "" && cfg.Origin == "" && cfg.Generation == "" && cfg.AuditKeyID == "" && len(cfg.AuditKey) == 0 {
		return httpserver.NewHandler(nil), func() {}, nil
	}
	parsed, err := url.Parse(cfg.Origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || cfg.DatabaseURL == "" || cfg.RedisURL == "" || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(cfg.Generation) || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`).MatchString(cfg.AuditKeyID) || len(cfg.AuditKey) < 32 {
		return nil, nil, errors.New("incomplete or invalid authentication configuration")
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, errors.New("invalid authentication database configuration")
	}
	sessions := session.NewStore(cfg.RedisURL, cfg.Generation)
	close := func() { _ = sessions.Close(); pool.Close() }
	s := &auth.Service{Pool: pool, Sessions: sessions, Origin: cfg.Origin, AuditKeyID: cfg.AuditKeyID, AuditKey: cfg.AuditKey, Logger: slog.Default()}
	if err := s.Ready(ctx); err != nil {
		close()
		return nil, nil, errors.New("authentication dependencies unavailable")
	}
	return httpserver.NewHandler(s.Ready, s), close, nil
}
