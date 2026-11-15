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

// The codes (docs/tech-stack.md §3). The page reads them, so changing one
// is changing the API.
const (
	// No single endpoint's.
	codeUnauthorized    = "unauthorized"
	codeNotFound        = "not_found"
	codeInternal        = "internal"
	codeAuthUnavailable = "auth_unavailable"

	// A request body (request.go).
	codeBadRequest           = "bad_request"
	codeTooLarge             = "too_large"
	codeUnsupportedMediaType = "unsupported_media_type"

	// A well-formed body that a rule refuses: 422, with the field.
	codeMissingField    = "missing_field"
	codeUnexpectedField = "unexpected_field"
	codeInvalidDate     = "invalid_date"
	codeDateInFuture    = "date_in_future"
	codeDateTooEarly    = "date_too_early"
	codeInvalidTime     = "invalid_time"
	// A value that isn't one of the values a field has: an enum, a tag, a
	// secretion amount, a unit, a name in "confirmed".
	codeInvalidValue = "invalid_value"
	// Notes or a secretion description over its limit.
	codeTooLong = "too_long"
	// A temperature outside 20.00–50.00 °C. Refused, never asked about
	// (AC-17).
	codeTemperatureOutOfRange = "temperature_out_of_range"

	// The account's state refuses it: 409.
	codeAlreadyOnboarded = "already_onboarded"
	codeNotOnboarded     = "not_onboarded"

	// She must be asked first: 409 too. Nothing is wrong with the request;
	// it is saved once it comes again with this code's name in "confirmed"
	// (docs/tech-stack.md §3). The code is the question.
	codeNoTemperature      = "no_temperature"
	codeUnusualTemperature = "unusual_temperature"
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
