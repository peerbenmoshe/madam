package main

import (
	"strings"
	"testing"
)

func TestNewPool(t *testing.T) {
	// The Makefile exports PGSSLROOTCERT, and pgx also reads PGSSLMODE. Clear
	// both so each case depends only on its own URL. (pgx ignores empty ones.)
	t.Setenv("PGSSLROOTCERT", "")
	t.Setenv("PGSSLMODE", "")

	tests := []struct {
		name    string
		url     string
		wantErr string // empty: no error expected
	}{
		{name: "missing", url: "", wantErr: "DATABASE_URL is not set"},
		{name: "local container, no TLS", url: "postgres://madam:madam@127.0.0.1:5432/madam?sslmode=disable"},
		{name: "localhost, no TLS", url: "postgres://madam:madam@localhost:5432/madam?sslmode=disable"},
		{name: "Compose service name, no TLS", url: "postgres://madam:madam@postgres:5432/madam?sslmode=disable"},
		{name: "remote, verify-full", url: "postgres://u:pw@db.example.com:5432/postgres?sslmode=verify-full"},
		{name: "remote, no TLS", url: "postgres://u:pw@db.example.com:5432/postgres?sslmode=disable", wantErr: "verify-full"},
		{name: "remote, unverified TLS", url: "postgres://u:pw@db.example.com:5432/postgres?sslmode=require", wantErr: "verify-full"},
		{name: "remote, CA only", url: "postgres://u:pw@db.example.com:5432/postgres?sslmode=verify-ca", wantErr: "verify-full"},
		{name: "remote, default sslmode", url: "postgres://u:pw@db.example.com:5432/postgres", wantErr: "verify-full"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pool, err := newPool(t.Context(), tc.url)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				pool.Close()
				return
			}
			if err == nil {
				pool.Close()
				t.Fatalf("no error, want one containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// The error ends up in Cloud Run's logs, so it must not repeat the password.
func TestNewPoolParseErrorHidesPassword(t *testing.T) {
	_, err := newPool(t.Context(), "postgres://u:s3cretpw@db.example.com:notaport/postgres")
	if err == nil {
		t.Fatal("no error for a malformed URL")
	}
	if strings.Contains(err.Error(), "s3cretpw") {
		t.Errorf("error leaks the password: %v", err)
	}
}
