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
	"github.com/peerbenmoshe/madam/backend/internal/web"
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

	// Which Supabase project's tokens we accept. Not a secret: the page
	// talks to the same address. Its public keys are fetched on the first
	// authenticated request, not here, so a Supabase outage can't stop the
	// server from starting (docs/tech-stack.md §4).
	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		return errors.New("SUPABASE_URL is not set")
	}
	verifier, err := api.NewJWTVerifier(supabaseURL, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		return fmt.Errorf("SUPABASE_URL: %w", err)
	}

	// Supabase calls this hook before it creates an account; the sign-up
	// goes ahead only with the invite code (docs/spec-v1.md §2). Both values
	// come from Secret Manager on Cloud Run and from the Makefile's
	// throwaway defaults locally. Without them there would be no gate, so
	// the server doesn't start.
	hookSecret, inviteCode := os.Getenv("AUTH_HOOK_SECRET"), os.Getenv("INVITE_CODE")
	if hookSecret == "" {
		return errors.New("AUTH_HOOK_SECRET is not set")
	}
	if inviteCode == "" {
		return errors.New("INVITE_CODE is not set")
	}
	signUpHook, err := api.NewBeforeUserCreatedHook(hookSecret, inviteCode)
	if err != nil {
		return fmt.Errorf("sign-up hook (AUTH_HOOK_SECRET, INVITE_CODE): %w", err)
	}

	// The frontend is embedded at compile time (internal/web). A binary built
	// before `make frontend` still serves the API, and answers pages with 503.
	assets := web.Assets()
	if !web.HasIndex(assets) {
		slog.Warn("frontend not built: pages answer 503 until `make frontend` runs; /api/ works")
	}

	store := db.NewStore(pool)
	srv := &http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           api.NewRouter(store, store, verifier, signUpHook, web.Handler(assets)),
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
