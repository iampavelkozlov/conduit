package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"conduit/frontend"
	"conduit/internal/config"
	"conduit/internal/logger"
)

func main() {
	if err := mainError(); err != nil {
		log.Printf("conduit frontend: %v", err)
		os.Exit(1)
	}
}

func mainError() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, os.Args[1:])
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("conduit-frontend", flag.ContinueOnError)
	configPath := flags.String("config", "config/frontend.yaml", "path to frontend config file")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	cfg, err := config.LoadFrontend(*configPath)
	if err != nil {
		return err
	}
	appLogger, err := logger.New(config.LoggerConfig{Level: cfg.Logger.Level, Format: cfg.Logger.Format})
	if err != nil {
		return fmt.Errorf("initialize frontend logger: %w", err)
	}
	app, err := frontend.New(cfg.API.URL, &http.Client{Timeout: cfg.API.Timeout}, appLogger, frontend.Options{
		CookieName: cfg.Session.CookieName, CookieSecure: cfg.Session.Secure,
		CookieTTL: cfg.Session.TTL, MaxFormBytes: cfg.HTTP.MaxFormBytes,
	})
	if err != nil {
		return fmt.Errorf("initialize frontend: %w", err)
	}
	server := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	appLogger.InfoContext(ctx, "frontend initialized", "addr", cfg.HTTP.Addr, "api_url", cfg.API.URL)

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve frontend: %w", err)
		}
		return nil
	case <-ctx.Done():
		appLogger.InfoContext(ctx, "shutting down frontend")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown frontend: %w", errors.Join(err, server.Close()))
	}
	<-serverErr
	return nil
}
