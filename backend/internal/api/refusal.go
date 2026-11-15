package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// refusal is an error answer and its status: what a rule says about a
// request it won't accept, before anything is written.
type refusal struct {
	status int
	apiError
}

// refuse builds a refusal about one field of the request.
func refuse(status int, code, field, message string) *refusal {
	return &refusal{status: status, apiError: apiError{Code: code, Field: field, Message: message}}
}

// acceptedDate reads a date as the API writes one and checks that it may
// be recorded at the instant now (docs/fam-rules.md §9.1, §9.2). Every
// date the app accepts goes through it: a cycle start, a day's date.
//
// field names the date in the refusal. No message repeats what was sent.
func acceptedDate(field, written string, now time.Time) (fam.Date, *refusal) {
	date, err := fam.ParseDate(written)
	if err != nil {
		return fam.Date{}, refuse(http.StatusUnprocessableEntity, codeInvalidDate, field,
			field+" must be a real date written YYYY-MM-DD")
	}
	switch err := fam.CheckDate(date, now); {
	case errors.Is(err, fam.ErrDateInFuture):
		return fam.Date{}, refuse(http.StatusUnprocessableEntity, codeDateInFuture, field,
			field+" is after today")
	case errors.Is(err, fam.ErrDateTooEarly):
		return fam.Date{}, refuse(http.StatusUnprocessableEntity, codeDateTooEarly, field,
			field+" is before "+fam.EarliestDate().String())
	case err != nil:
		// CheckDate grew a rule this function doesn't know. Refuse.
		return fam.Date{}, refuse(http.StatusUnprocessableEntity, codeInvalidDate, field,
			field+" is not accepted")
	}
	return date, nil
}
