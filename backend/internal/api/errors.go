package api

import (
	"context"
	"net/http"
)

// The API's errors (docs/tech-stack.md §3). Every error the API answers
// with has this one shape:
//
//	{"error": {"code": "date_in_future", "field": "cycle_start_date", "message": "..."}}
//
// Two things under /api keep their own: the sign-up hook, which answers
// Supabase in Supabase's protocol (hook.go), and a wrong method on a real
// path, which is the router's own 405 with its Allow header.

// apiError is one error, as the page receives it.
type apiError struct {
	// Code is stable and is what the page reads. snake_case.
	Code string `json:"code"`
	// Field is the request's JSON field the error is about, or empty.
	Field string `json:"field,omitempty"`
	// Message is for a developer reading a log or a curl answer. The page
	// never shows it: its own words are in the frontend's text modules. It
	// must never carry a cause (a database error, a token's flaw) or
	// anything a user typed.
	Message string `json:"message"`
}

// errorBody is the response body that carries an apiError.
type errorBody struct {
	Error apiError `json:"error"`
}

// The codes that belong to no single endpoint.
const (
	codeUnauthorized    = "unauthorized"
	codeNotFound        = "not_found"
	codeInternal        = "internal"
	codeAuthUnavailable = "auth_unavailable"
)

// internalErrorJSON is errorBody{internal} already encoded, for the one
// case where encoding itself failed.
const internalErrorJSON = `{"error":{"code":"internal","message":"internal error"}}`

// writeError answers with e in the API's error shape.
func writeError(ctx context.Context, w http.ResponseWriter, status int, e apiError) {
	writeJSON(ctx, w, status, errorBody{Error: e})
}

// writeInternalError answers 500. The cause belongs in the log, written by
// the caller; the response says only that it was our fault.
func writeInternalError(ctx context.Context, w http.ResponseWriter) {
	writeError(ctx, w, http.StatusInternalServerError, apiError{Code: codeInternal, Message: "internal error"})
}
