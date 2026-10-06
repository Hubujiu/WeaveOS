// The composition root wires the platform host and configured authentication.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	cfg, err := readConfig(os.Getenv)
	if err != nil {
		logger.Error("BFF authentication configuration invalid")
		os.Exit(1)
	}
	startup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	handler, closeResources, err := buildHandler(startup, cfg)
	cancel()
	if err != nil {
		logger.Error("BFF authentication initialization failed")
		os.Exit(1)
	}
	defer closeResources()
	addr := os.Getenv("BFF_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr: addr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
		}
		close(stopped)
	}()
	logger.Info("starting BFF host", "authenticationConfigured", cfg.DatabaseURL != "")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("BFF host failed", "error", err.Error())
		os.Exit(1)
	}
	<-stopped
}
