package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxConns per instance. Cloud Run runs at most 2 instances, and every
// connection holds one of the Supabase session pooler's server slots for as
// long as it lives (docs/tech-stack.md §5).
const maxConns = 4

// newPool configures the database connection pool from a connection URL.
// It doesn't connect: pgxpool opens connections on first use, so a database
// outage can't stop the server from starting. A missing or malformed URL is a
// configuration error, though, and stops it straight away.
func newPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	// pgx also accepts "key=value" strings, and reads anything that isn't a
	// URL that way, ignoring what it doesn't recognise. A URL with stray
	// quotes would then silently become "local socket, current OS user".
	if !strings.HasPrefix(url, "postgres://") && !strings.HasPrefix(url, "postgresql://") {
		return nil, errors.New("DATABASE_URL must start with postgres:// or postgresql:// (no quotes or spaces around it)")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if err := requireVerifiedTLS(&cfg.ConnConfig.Config); err != nil {
		return nil, err
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = 0 // nothing connects while the instance is idle
	cfg.ConnConfig.RuntimeParams["application_name"] = "madam"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	return pool, nil
}

// requireVerifiedTLS refuses a configuration that would talk to a remote
// database without checking its certificate. sslmode=require encrypts but
// accepts any certificate, and the default, prefer, can fall back to plain
// text; only verify-full checks both the issuer and the host name.
func requireVerifiedTLS(cfg *pgconn.Config) error {
	targets := append([]*pgconn.FallbackConfig{
		{Host: cfg.Host, Port: cfg.Port, TLSConfig: cfg.TLSConfig},
	}, cfg.Fallbacks...)
	for _, t := range targets {
		if isLocalHost(t.Host) {
			continue
		}
		if t.TLSConfig == nil || t.TLSConfig.InsecureSkipVerify {
			return fmt.Errorf("DATABASE_URL: the connection to %s must use sslmode=verify-full", t.Host)
		}
	}
	return nil
}

// isLocalHost reports whether host can only be on this machine or its local
// network: a Unix socket, a loopback address, or a name without a dot, such as
// "localhost" or the Docker Compose service "postgres".
func isLocalHost(host string) bool {
	if strings.HasPrefix(host, "/") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return !strings.Contains(host, ".")
}
