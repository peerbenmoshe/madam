package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const indexHTML = `<!doctype html><div id="root"></div>`

// built is what `make frontend` leaves in assets/, in miniature.
var built = fstest.MapFS{
	"index.html":               {Data: []byte(indexHTML)},
	"assets/index-a1b2c3.js":   {Data: []byte("console.log('app')")},
	"assets/index-d4e5f6.css":  {Data: []byte("body{}")},
	"assets/_helper-0a1b2c.js": {Data: []byte("export{}")},
	"robots.txt":               {Data: []byte("User-agent: *")},
	".gitkeep":                 {Data: nil},
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string // exact, when set
		wantType    string // prefix of Content-Type, when set
		wantCaching string // Cache-Control, when set
	}{
		{name: "root is the page", path: "/", wantStatus: 200, wantBody: indexHTML, wantType: "text/html", wantCaching: cacheRevalidate},
		{name: "index.html itself, no redirect", path: "/index.html", wantStatus: 200, wantBody: indexHTML, wantCaching: cacheRevalidate},
		{name: "client-side route", path: "/day/2026-11-06", wantStatus: 200, wantBody: indexHTML, wantType: "text/html", wantCaching: cacheRevalidate},
		{name: "hashed script", path: "/assets/index-a1b2c3.js", wantStatus: 200, wantBody: "console.log('app')", wantType: "text/javascript", wantCaching: cacheImmutable},
		{name: "hashed stylesheet", path: "/assets/index-d4e5f6.css", wantStatus: 200, wantType: "text/css", wantCaching: cacheImmutable},
		{name: "chunk starting with _", path: "/assets/_helper-0a1b2c.js", wantStatus: 200, wantCaching: cacheImmutable},
		{name: "unhashed root file", path: "/robots.txt", wantStatus: 200, wantType: "text/plain", wantCaching: cacheRevalidate},
		{name: "missing chunk is 404, not the page", path: "/assets/index-old999.js", wantStatus: 404},
		{name: "missing file with an extension", path: "/favicon.ico", wantStatus: 404},
		{name: "assets directory is not listed", path: "/assets/", wantStatus: 404},
		{name: "assets without slash", path: "/assets", wantStatus: 404},
		{name: "unknown name under assets", path: "/assets/whatever", wantStatus: 404},
		{name: "the placeholder is hidden", path: "/.gitkeep", wantStatus: 404},
		{name: "hidden directory", path: "/.git/config", wantStatus: 404},
		{name: "traversal stays inside", path: "/../../etc/passwd", wantStatus: 200, wantBody: indexHTML},
		{name: "HEAD has no body", method: http.MethodHead, path: "/", wantStatus: 200, wantBody: "", wantType: "text/html"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req := httptest.NewRequest(method, tc.path, nil)
			rec := httptest.NewRecorder()

			Handler(built).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if (tc.wantBody != "" || method == http.MethodHead) && rec.Body.String() != tc.wantBody {
				t.Errorf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantStatus == 404 && strings.Contains(rec.Body.String(), "root") {
				t.Errorf("a 404 sent the page: %q", rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); tc.wantType != "" && !strings.HasPrefix(got, tc.wantType) {
				t.Errorf("Content-Type = %q, want %q...", got, tc.wantType)
			}
			if got := rec.Header().Get("Cache-Control"); tc.wantCaching != "" && got != tc.wantCaching {
				t.Errorf("Cache-Control = %q, want %q", got, tc.wantCaching)
			}
		})
	}
}

// A binary compiled before `make frontend` holds only .gitkeep.
func TestHandlerWithoutBuild(t *testing.T) {
	unbuilt := fstest.MapFS{".gitkeep": {Data: nil}}
	for path, want := range map[string]int{
		"/":                503,
		"/day/2026-11-06":  503,
		"/assets/index.js": 404,
	} {
		rec := httptest.NewRecorder()
		Handler(unbuilt).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s: status = %d, want %d", path, rec.Code, want)
		}
	}
	if HasIndex(unbuilt) {
		t.Error("HasIndex = true for a tree without index.html")
	}
}

func TestHandlerCompresses(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/assets/index-a1b2c3.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	Handler(built).ServeHTTP(rec, req)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(body) != "console.log('app')" {
		t.Errorf("decompressed body = %q", body)
	}
}

// A range of a compressed body would be wrong, so ranges are ignored.
func TestHandlerIgnoresRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()

	Handler(built).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != indexHTML {
		t.Errorf("status %d, body %q: want 200 and the whole page", rec.Code, rec.Body.String())
	}
}
