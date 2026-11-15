package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/peerbenmoshe/madam/backend/internal/db"
	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// onboardingMaxBytes caps the request body. The largest valid one is under
// a hundred bytes.
const onboardingMaxBytes = 4 << 10

// The request's field names, as the page sends them and as errors name them.
const (
	fieldCycleStartUnknown = "cycle_start_unknown"
	fieldCycleStartDate    = "cycle_start_date"
	fieldMeasurementTime   = "measurement_time"
)

// onboardingRequest is the body of POST /api/onboarding. Every field is a
// pointer so that "not sent" can be told from false or "": the flag must be
// said, never guessed.
type onboardingRequest struct {
	// CycleStartUnknown is the answer "I don't know". Required.
	CycleStartUnknown *bool `json:"cycle_start_unknown"`
	// CycleStartDate is day 1 of her current cycle, "YYYY-MM-DD". Required
	// when the start is known, and refused when it isn't.
	CycleStartDate *string `json:"cycle_start_date"`
	// MeasurementTime is her fixed measurement time, "HH:MM". Required.
	MeasurementTime *string `json:"measurement_time"`
}

// onboarding is a request that passed every check, in the domain's types.
type onboarding struct {
	startUnknown bool
	start        fam.Date // meaningful only when startUnknown is false
	time         fam.Minutes
}

// errAlreadyOnboarded leaves the transaction when the gate finds onboarding
// already done, so everything rolls back.
var errAlreadyOnboarded = errors.New("already onboarded")

// handleOnboarding answers POST /api/onboarding (docs/spec-v1.md §4,
// docs/tech-stack.md §3): it stores the two things she is asked after
// signing up, once.
//
// In one transaction it writes onboarded_at and her answer about the cycle
// start on her users row, the fixed measurement time from -infinity, and
// the cycle start if she knows it. A second submit changes nothing and is
// answered 409. A success answers what GET /api/me would.
//
// now is the clock, passed in so that tests choose the instant: which dates
// are in the future depends on it.
func handleOnboarding(store UserStore, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		id, ok := UserID(ctx)
		if !ok {
			// The route was mounted without RequireUser: our bug.
			slog.ErrorContext(ctx, "onboarding: no user id in the request's context")
			writeInternalError(ctx, w)
			return
		}

		var req onboardingRequest
		if !readJSON(w, r, onboardingMaxBytes, &req) {
			return
		}
		answers, refusal := checkOnboarding(req, now())
		if refusal != nil {
			writeError(ctx, w, refusal.status, refusal.apiError)
			return
		}

		var user db.User
		err := store.InUserTx(ctx, id, func(q *db.Queries) error {
			// Her row may not exist yet: GET /api/me usually creates it,
			// but nothing obliges a client to call that first.
			if err := q.EnsureUser(ctx, id); err != nil {
				return fmt.Errorf("ensure the user's row: %w", err)
			}
			// The gate. It changes the row only while onboarded_at is NULL.
			changed, err := q.CompleteOnboarding(ctx, db.CompleteOnboardingParams{
				ID:                id,
				CycleStartUnknown: answers.startUnknown,
			})
			if err != nil {
				return fmt.Errorf("complete onboarding: %w", err)
			}
			if changed == 0 {
				return errAlreadyOnboarded
			}
			err = q.CreateFirstMeasurementTime(ctx, db.CreateFirstMeasurementTimeParams{
				UserID:          id,
				MeasurementTime: db.MinutesToPg(answers.time),
			})
			if err != nil {
				return fmt.Errorf("write the first measurement time: %w", err)
			}
			if !answers.startUnknown {
				err = q.CreateCycleStart(ctx, db.CreateCycleStartParams{
					UserID:    id,
					StartDate: db.DateToPg(answers.start),
				})
				if err != nil {
					return fmt.Errorf("write the cycle start: %w", err)
				}
			}
			if user, err = q.GetUser(ctx, id); err != nil {
				return fmt.Errorf("read the user's row: %w", err)
			}
			return nil
		})
		if errors.Is(err, errAlreadyOnboarded) {
			writeError(ctx, w, http.StatusConflict,
				apiError{Code: codeAlreadyOnboarded, Message: "onboarding is already done; nothing was changed"})
			return
		}
		if err != nil {
			// The cause goes to the log only.
			slog.ErrorContext(ctx, "onboarding: database", "err", err)
			writeInternalError(ctx, w)
			return
		}

		writeJSON(ctx, w, http.StatusOK, meResponseFor(user))
	}
}

// checkOnboarding applies the rules to a decoded request, at the instant
// now. It returns the answers in the domain's types, or the first thing
// that is wrong. The page checks all of this before sending, so one error
// at a time is enough here. No message repeats a value that was sent.
func checkOnboarding(req onboardingRequest, now time.Time) (onboarding, *refusal) {
	var out onboarding

	// A flag that wasn't sent is a broken request, not a wrong answer: 400.
	if req.CycleStartUnknown == nil {
		return out, refuse(http.StatusBadRequest, codeBadRequest, fieldCycleStartUnknown,
			"cycle_start_unknown is required: true or false")
	}
	out.startUnknown = *req.CycleStartUnknown

	switch {
	case out.startUnknown && req.CycleStartDate != nil:
		return out, refuse(http.StatusUnprocessableEntity, codeUnexpectedField, fieldCycleStartDate,
			"cycle_start_date must be left out when cycle_start_unknown is true")
	case !out.startUnknown && req.CycleStartDate == nil:
		return out, refuse(http.StatusUnprocessableEntity, codeMissingField, fieldCycleStartDate,
			"cycle_start_date is required when cycle_start_unknown is false")
	case !out.startUnknown:
		start, refused := acceptedDate(fieldCycleStartDate, *req.CycleStartDate, now)
		if refused != nil {
			return out, refused
		}
		out.start = start
	}

	if req.MeasurementTime == nil {
		return out, refuse(http.StatusUnprocessableEntity, codeMissingField, fieldMeasurementTime,
			"measurement_time is required")
	}
	at, err := fam.ParseMinutes(*req.MeasurementTime)
	if err != nil {
		return out, refuse(http.StatusUnprocessableEntity, codeInvalidTime, fieldMeasurementTime,
			"measurement_time must be written HH:MM, from 00:00 to 23:59")
	}
	out.time = at

	return out, nil
}
