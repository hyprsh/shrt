// Command shrt serves the hypr.sh URL shortener. It is configured through
// environment variables; see shrt.Config. `shrt health` checks a running shrt.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hyprsh/shrt"
)

// version is set from the release tag at build time, with
// -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	switch {
	case len(os.Args) == 1:
		if err := run(); err != nil {
			slog.Error("shrt stopped", "err", err)
			os.Exit(1)
		}
	case len(os.Args) == 2 && os.Args[1] == "health":
		if err := shrt.CheckHealthFromEnv(context.Background(), os.Getenv); err != nil {
			slog.Error("health check failed", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: shrt [health]")
		os.Exit(2)
	}
}

func run() error {
	if err := shrt.CheckEnv(os.Environ()); err != nil {
		return err
	}
	cfg, err := shrt.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	if cfg.Token == "" {
		slog.Warn("no API token configured, so every API call is refused; set SHRT_TOKEN or SHRT_TOKEN_FILE")
	}

	store, err := shrt.OpenStore(cfg.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           shrt.NewServer(store, cfg.BaseURL, cfg.Token),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("listening", "version", version, "addr", cfg.Addr, "base_url", cfg.BaseURL, "db", cfg.DBPath)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
