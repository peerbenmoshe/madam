package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeDB answers Ping with err, or, if block is set, waits for the context
// to end the way a hung connection would.
type fakeDB struct {
	err   error
	block bool
}

func (f fakeDB) Ping(ctx context.Context) (int32, error) {
	if f.block {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	if f.err != nil {
		return 0, f.err
	}
	return 1, nil
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		db         fakeDB
		method     string
		wantStatus int
		wantBody   string
	}{
		{name: "database up", method: http.MethodGet, wantStatus: http.StatusOK, wantBody: "ok"},
		{
			name:       "database down",
			db:         fakeDB{err: errors.New("connection refused")},
			method:     http.MethodGet,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "database unavailable",
		},
		{name: "POST is not allowed", method: http.MethodPost, wantStatus: http.StatusMethodNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/health", nil)
			rec := httptest.NewRecorder()

			NewRouter(tc.db, nil, noAuth, noHook, testConfig, fakeFrontend).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody != "" && rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestHealthTimesOut(t *testing.T) {
	const timeout = 50 * time.Millisecond
	handler := handleHealth(fakeDB{block: true}, timeout)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	// Generous upper bound: the point is that it returns, not how fast.
	if elapsed > 20*timeout {
		t.Errorf("took %v, want about %v", elapsed, timeout)
	}
}
