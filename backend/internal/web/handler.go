package web

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

const (
	indexFile = "index.html"

	// Vite writes every bundled file to assets/ with a hash of its contents
	// in the name, so a changed file always gets a new name.
	assetsDir = "assets"

	// A hashed name never changes content, so the browser may keep it for a
	// year without asking again.
	cacheImmutable = "public, max-age=31536000, immutable"
	// Everything else, index.html above all, is checked on every load, so a
	// deploy reaches an open phone on its next reload. index.html is small.
	cacheRevalidate = "no-cache"
)

// HasIndex reports whether fsys holds a built frontend.
func HasIndex(fsys fs.FS) bool {
	_, err := fs.Stat(fsys, indexFile)
	return err == nil
}

// Handler serves the single-page app in fsys. The router sends it every GET
// and HEAD outside /api/.
//
//   - A file that exists is served as itself.
//   - A missing name with a file extension (/favicon.ico), or anything under
//     /assets/, is 404. After a deploy an old tab may ask for a chunk that no
//     longer exists, and it must get an error, not HTML labelled as script.
//   - Any other path is a client-side route (/day/2026-11-06): it gets
//     index.html, and the app decides what to show.
//
// Text responses are gzip-compressed when the browser accepts it.
func Handler(fsys fs.FS) http.Handler {
	serve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveApp(fsys, w, r)
	})
	return middleware.Compress(5)(serve)
}

func serveApp(fsys fs.FS, w http.ResponseWriter, r *http.Request) {
	// Clean first: "/../x" becomes "/x", so no request can name anything
	// outside fsys (fs.FS also refuses ".." itself).
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")

	switch {
	case name == "" || name == indexFile:
		serveIndex(fsys, w, r)
	case hasHiddenSegment(name):
		// .gitkeep, or anything else that was never meant to be public.
		http.Error(w, "not found", http.StatusNotFound)
	case isRegularFile(fsys, name):
		cache := cacheRevalidate
		if strings.HasPrefix(name, assetsDir+"/") {
			cache = cacheImmutable
		}
		serveFile(fsys, w, r, name, cache)
	case path.Ext(name) != "" || name == assetsDir || strings.HasPrefix(name, assetsDir+"/"):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		serveIndex(fsys, w, r)
	}
}

// serveIndex sends the app's page, or 503 if the binary was built without a
// frontend (a plain `go build` before `make frontend`).
func serveIndex(fsys fs.FS, w http.ResponseWriter, r *http.Request) {
	if !HasIndex(fsys) {
		http.Error(w, "frontend not built (run make frontend)", http.StatusServiceUnavailable)
		return
	}
	serveFile(fsys, w, r, indexFile, cacheRevalidate)
}

func serveFile(fsys fs.FS, w http.ResponseWriter, r *http.Request, name, cache string) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		// The file was there a moment ago (isRegularFile, HasIndex), and the
		// embedded files never change, so this is a programming error.
		slog.ErrorContext(r.Context(), "frontend: read embedded file", "name", name, "err", err)
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", cache)
	// No byte ranges: a range of a gzip-compressed body would be wrong, and
	// nothing this small needs one. Without the header, ServeContent sends
	// the whole file.
	r.Header.Del("Range")
	// ServeContent sets Content-Type from the extension and answers HEAD. The
	// zero time means "unknown", so there's no Last-Modified: embedded files
	// have no modification time.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func isRegularFile(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && info.Mode().IsRegular()
}

// hasHiddenSegment reports whether any part of name starts with ".".
func hasHiddenSegment(name string) bool {
	for segment := range strings.SplitSeq(name, "/") {
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}
