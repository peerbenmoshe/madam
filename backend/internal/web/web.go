// Package web serves the frontend: the files Vite builds, embedded in the
// server binary so that the app and the API are one artefact at one URL
// (docs/tech-stack.md §8.4).
package web

import (
	"embed"
	"io/fs"
)

// The build copies frontend/dist/ into assets/ before compiling: `make
// frontend` on the laptop and in CI, a COPY in the Dockerfile. go:embed can't
// reach ../frontend itself, because a pattern may not contain "..".
//
// "all:" also embeds names starting with "." or "_", which Go skips by
// default. Rollup can emit chunks named like that. It also picks up
// assets/.gitkeep, the committed placeholder that keeps this directory
// non-empty, so the package compiles on a fresh clone where nothing is built.
//
//go:embed all:assets
var embedded embed.FS

// Assets returns the embedded files, rooted at assets/, so the page is
// "index.html".
func Assets() fs.FS {
	sub, err := fs.Sub(embedded, "assets")
	if err != nil {
		// fs.Sub fails only for an invalid directory name, and "assets" is a
		// constant: this can't happen at runtime. It runs once, at startup.
		panic(err)
	}
	return sub
}
