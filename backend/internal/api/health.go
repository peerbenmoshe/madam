package api

import "net/http"

// handleHealthz reports that the process is up and serving HTTP.
// It deliberately checks nothing else; 0c adds a database read.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
