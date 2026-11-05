// Command server runs the Madam HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/peerbenmoshe/madam/backend/internal/api"
	"github.com/peerbenmoshe/madam/backend/internal/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Cloud Run sets PORT; 8080 is the local default.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Locally the Makefile sets DATABASE_URL; on Cloud Run it comes from
	// Secret Manager (docs/tech-stack.md §7). Either way it's the application
	// role, never the owner, so Row Level Security applies (docs/data-model.md §9).
	pool, err := newPool(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	// Runs after the server has shut down, so in-flight requests keep the pool.
	defer pool.Close()

	srv := &http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           api.NewRouter(db.NewStore(pool)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Cloud Run sends SIGTERM before stopping an instance; Ctrl+C sends SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	// Let in-flight requests finish, but don't wait forever.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	slog.Info("shut down cleanly")
	return nil
}
