// The composition root wires the platform host and configured authentication.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)
	if err := runBFF(logger); err != nil {
		logger.Error("BFF host failed", "error", err.Error())
		os.Exit(1)
	}
}

func runBFF(logger *slog.Logger) (err error) {
	processCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := readConfig(os.Getenv)
	if err != nil {
		return errors.New("BFF authentication configuration invalid")
	}
	startup, cancel := context.WithTimeout(processCtx, 5*time.Second)
	host, err := buildHost(startup, processCtx, cfg)
	cancel()
	if err != nil {
		return errors.New("BFF authentication initialization failed")
	}
	defer func() {
		if host.Close() != nil {
			err = errors.Join(err, errBFFHostCleanup)
		}
	}()
	addr := os.Getenv("BFF_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{
		Addr: addr, Handler: host.Handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("BFF HTTP listener initialization failed")
	}
	logger.Info("starting BFF host", "authenticationConfigured", cfg.DatabaseURL != "")
	return serveBFF(processCtx, host, server, listener, 10*time.Second)
}
