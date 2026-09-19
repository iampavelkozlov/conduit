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
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Printf("conduit: %v", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWith(ctx, *configPath, *addr, initializeApplication, func(server *http.Server) error {
		return server.ListenAndServe()
	}, func(server *http.Server, ctx context.Context) error { return server.Shutdown(ctx) })
}

type applicationInitializer func(context.Context, string) (*application, func(), error)
type httpServerRunner func(*http.Server) error
type httpServerShutdown func(*http.Server, context.Context) error

func runWith(ctx context.Context, configPath, addr string, initialize applicationInitializer, serve httpServerRunner, shutdown httpServerShutdown) error {
	app, cleanup, err := initialize(ctx, configPath)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	defer cleanup()

	server := &http.Server{
		Addr:              addr,
		Handler:           app.handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
	}

	app.logger.InfoContext(ctx, "server initialized", "addr", addr)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- serve(server)
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
		app.logger.InfoContext(context.WithoutCancel(ctx), "shutting down HTTP server")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := shutdown(server, shutdownCtx); err != nil {
		closeErr := server.Close()
		serveErr := <-serverErr
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return fmt.Errorf("shutdown HTTP server: %w", errors.Join(err, closeErr, serveErr))
	}
	if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP during shutdown: %w", err)
	}
	return nil
}
