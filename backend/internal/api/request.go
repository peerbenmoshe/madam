package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
)

// readJSON decodes the request's JSON body into v, for every endpoint that
// takes one (docs/tech-stack.md §3). It reports whether it succeeded; when
// it didn't, it has already answered, and the handler just returns.
//
//   - 415 unless the body says it is application/json.
//   - 413 if the body is longer than maxBytes.
//   - 400 if it isn't one JSON value of v's shape. A field v doesn't have is
//     refused, not ignored, so a request can't carry something (a user_id,
//     say) that looks accepted and isn't.
//
// The decoder's own error names the bad field and sometimes quotes the
// body, so it goes to the log only, and only as its kind.
func readJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, v any) bool {
	ctx := r.Context()

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(ctx, w, http.StatusUnsupportedMediaType,
			apiError{Code: codeUnsupportedMediaType, Message: "the body must be application/json"})
		return false
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	dec.DisallowUnknownFields()
	err = dec.Decode(v)
	if err == nil {
		// One value only: anything after it but white space is refused.
		var extra json.RawMessage
		if next := dec.Decode(&extra); next == nil {
			err = errors.New("a second JSON value follows the first")
		} else if !errors.Is(next, io.EOF) {
			err = next
		}
	}
	if err == nil {
		return true
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(ctx, w, http.StatusRequestEntityTooLarge,
			apiError{Code: codeTooLarge, Message: "the body is too large"})
		return false
	}
	slog.InfoContext(ctx, "request body refused", "path", r.URL.Path, "kind", jsonErrorKind(err))
	writeError(ctx, w, http.StatusBadRequest,
		apiError{Code: codeBadRequest, Message: "the body is not the JSON this endpoint reads"})
	return false
}

// jsonErrorKind names what was wrong with a body without repeating any of
// it: a syntax error's text can quote what was sent.
func jsonErrorKind(err error) string {
	var syntax *json.SyntaxError
	var wrongType *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax), errors.Is(err, io.ErrUnexpectedEOF):
		return "not valid JSON"
	case errors.Is(err, io.EOF):
		return "empty body"
	case errors.As(err, &wrongType):
		return "a field of the wrong type"
	default:
		// Mostly "unknown field", which encoding/json reports as plain text.
		return "a field this endpoint doesn't read, or a second value"
	}
}
