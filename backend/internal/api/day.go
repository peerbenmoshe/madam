package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// One day of her chart, addressed by its date (docs/tech-stack.md §3):
//
//	GET    /api/days/2026-11-15         what the day holds; "entry" is null if nothing
//	PUT    /api/days/2026-11-15         save the day's entry, new or existing
//	DELETE /api/days/2026-11-15/entry   delete the day's entry
//
// DELETE has "/entry" in its path because the entry is all it deletes. From
// Slice 5 a day can also be the start of a cycle, and that stays.
//
// The rules about what an entry may hold are internal/fam's and are
// written in docs/fam-rules.md §9. This file reads the request, asks them,
// and answers.

// dayMaxBytes caps PUT's body. The largest valid one holds two texts at
// their limits, 2,000 and 200 characters, and one character is at most 12
// bytes of JSON (a \u escape pair): under 27 KiB.
const dayMaxBytes = 32 << 10

// The request's field names, as errors name them. The entry's fields are
// named plainly, without "entry." in front: each name is used once.
const (
	fieldDate                 = "date" // the date in the path
	fieldEntry                = "entry"
	fieldConfirmed            = "confirmed"
	fieldTemperature          = "temperature"
	fieldMeasuredAt           = "measured_at"
	fieldBleeding             = "bleeding"
	fieldSecretionCategory    = "secretion_category"
	fieldSensation            = "sensation"
	fieldSecretionAmount      = "secretion_amount"
	fieldSecretionDescription = "secretion_description"
	fieldCervixFirmness       = "cervix_firmness"
	fieldCervixPosition       = "cervix_position"
	fieldDisturbances         = "disturbances"
	fieldSymptoms             = "symptoms"
	fieldIntercourse          = "intercourse"
	fieldNotes                = "notes"
)

// dayRequest is the body of PUT /api/days/{date}.
type dayRequest struct {
	// Entry is the whole entry: it replaces whatever the day held. Required.
	Entry *entryRequest `json:"entry"`
	// Confirmed names the questions she has already answered "yes" to:
	// "no_temperature", "unusual_temperature". May be left out.
	Confirmed []string `json:"confirmed"`
}

// entryRequest is an entry as a client sends it. A pointer is nil when the
// field was left out or sent as null, which both mean "not said".
type entryRequest struct {
	Temperature *temperatureRequest `json:"temperature"`
	// MeasuredAt is "HH:MM". Required with a temperature, refused without.
	MeasuredAt *string `json:"measured_at"`
	// Bleeding is required: leaving it out must never be read as "none".
	Bleeding             *string `json:"bleeding"`
	SecretionCategory    *string `json:"secretion_category"`
	Sensation            *string `json:"sensation"`
	SecretionAmount      *int    `json:"secretion_amount"`
	SecretionDescription *string `json:"secretion_description"`
	CervixFirmness       *string `json:"cervix_firmness"`
	CervixPosition       *string `json:"cervix_position"`
	// Disturbances and Symptoms are required lists; [] says "none". A
	// list left out, or null, decodes to a nil slice and is refused.
	Disturbances []string `json:"disturbances"`
	Symptoms     []string `json:"symptoms"`
	Intercourse  *string  `json:"intercourse"`
	Notes        *string  `json:"notes"`
}

// temperatureRequest is a temperature as typed: whole hundredths of a
// degree, and the unit they are in. 36.62 °C is {3662, "celsius"}.
type temperatureRequest struct {
	Hundredths *int    `json:"hundredths"`
	Unit       *string `json:"unit"`
}

// dayResponse answers GET and PUT.
type dayResponse struct {
	Date string `json:"date"`
	// FixedTime is her usual measurement time on this date, "HH:MM": what
	// "Measured at" starts from on a day without a reading.
	FixedTime string `json:"fixed_time"`
	// Entry is null on a day with no entry.
	Entry *entryResponse `json:"entry"`
}

// entryResponse is an entry as the page receives it: the same fields as
// entryRequest, every one always present, null where nothing was said.
type entryResponse struct {
	Temperature          *temperatureResponse `json:"temperature"`
	MeasuredAt           *string              `json:"measured_at"`
	Bleeding             string               `json:"bleeding"`
	SecretionCategory    *string              `json:"secretion_category"`
	Sensation            *string              `json:"sensation"`
	SecretionAmount      *int                 `json:"secretion_amount"`
	SecretionDescription *string              `json:"secretion_description"`
	CervixFirmness       *string              `json:"cervix_firmness"`
	CervixPosition       *string              `json:"cervix_position"`
	Disturbances         []string             `json:"disturbances"`
	Symptoms             []string             `json:"symptoms"`
	Intercourse          *string              `json:"intercourse"`
	Notes                *string              `json:"notes"`
}

// temperatureResponse is the raw reading, as she typed it, in her
// account's unit.
type temperatureResponse struct {
	Hundredths int    `json:"hundredths"`
	Unit       string `json:"unit"`
}

// errNotOnboarded leaves the transaction when the account hasn't
// onboarded, so nothing is written.
var errNotOnboarded = errors.New("not onboarded")

// handleGetDay answers GET /api/days/{date} with what the day holds. A day
// without an entry is a 200 too, with "entry": null: the page still needs
// the day's fixed time.
func handleGetDay(store UserStore, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		id, date, ok := userAndDate(ctx, w, r, now())
		if !ok {
			return
		}

		var res dayResponse
		err := store.InUserTx(ctx, id, func(q *db.Queries) error {
			account, err := readDayAccount(ctx, q, id, date)
			if err != nil {
				return err
			}
			row, err := q.GetEntry(ctx, db.GetEntryParams{UserID: id, EntryDate: db.DateToPg(date)})
			if errors.Is(err, pgx.ErrNoRows) {
				res = dayResponseFor(date, account, nil)
				return nil
			}
			if err != nil {
				return fmt.Errorf("read the entry: %w", err)
			}
			return answerWithRow(&res, date, account, row)
		})
		if err != nil {
			writeDayFailure(ctx, w, "get day", err)
			return
		}
		writeJSON(ctx, w, http.StatusOK, res)
	}
}

// handlePutDay answers PUT /api/days/{date}: it saves the day's entry,
// whether or not the day had one, and answers what GET would.
//
// A request the rules refuse never reaches the database. One that needs a
// question first is answered 409 with the question as its code; it is
// saved when it comes again with that code in "confirmed".
func handlePutDay(store UserStore, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		id, date, ok := userAndDate(ctx, w, r, now())
		if !ok {
			return
		}

		var req dayRequest
		if !readJSON(w, r, dayMaxBytes, &req) {
			return
		}
		values, refused := checkDay(req)
		if refused != nil {
			writeError(ctx, w, refused.status, refused.apiError)
			return
		}
		params, err := values.UpsertParams(id, date)
		if err != nil {
			// checkDay let through a number that fits no column: our bug.
			slog.ErrorContext(ctx, "put day: convert", "err", err)
			writeInternalError(ctx, w)
			return
		}

		var res dayResponse
		err = store.InUserTx(ctx, id, func(q *db.Queries) error {
			// First, so that an account that hasn't onboarded writes nothing.
			account, err := readDayAccount(ctx, q, id, date)
			if err != nil {
				return err
			}
			row, err := q.UpsertEntry(ctx, params)
			if err != nil {
				return fmt.Errorf("save the entry: %w", err)
			}
			// Answered from the row the database returned, not from the
			// request: what is stored is what she is shown.
			return answerWithRow(&res, date, account, row)
		})
		if err != nil {
			writeDayFailure(ctx, w, "put day", err)
			return
		}
		writeJSON(ctx, w, http.StatusOK, res)
	}
}

// handleDeleteEntry answers DELETE /api/days/{date}/entry with 204 and no
// body. A day that had no entry is a 204 too: what she wanted, no entry on
// that day, is true either way, and a second tap or a second device must
// not look like a failure.
func handleDeleteEntry(store UserStore, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbTimeout)
		defer cancel()

		id, date, ok := userAndDate(ctx, w, r, now())
		if !ok {
			return
		}

		err := store.InUserTx(ctx, id, func(q *db.Queries) error {
			// The same answer as GET and PUT give an account that hasn't
			// onboarded. It has no entries, so nothing is at stake; the
			// page is simply told which screen she belongs on.
			if _, err := onboardedUser(ctx, q, id); err != nil {
				return err
			}
			err := q.DeleteEntry(ctx, db.DeleteEntryParams{UserID: id, EntryDate: db.DateToPg(date)})
			if err != nil {
				return fmt.Errorf("delete the entry: %w", err)
			}
			return nil
		})
		if err != nil {
			writeDayFailure(ctx, w, "delete entry", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// userAndDate reads what every request about a day carries: whose it is,
// from the token, and which day, from the path. ok is false when it has
// already answered, and the handler just returns.
func userAndDate(ctx context.Context, w http.ResponseWriter, r *http.Request, now time.Time) (id pgtype.UUID, date fam.Date, ok bool) {
	id, found := UserID(ctx)
	if !found {
		// The route was mounted without RequireUser: our bug.
		slog.ErrorContext(ctx, "day: no user id in the request's context")
		writeInternalError(ctx, w)
		return id, date, false
	}
	date, refused := acceptedDate(fieldDate, chi.URLParam(r, "date"), now)
	if refused != nil {
		writeError(ctx, w, refused.status, refused.apiError)
		return id, date, false
	}
	return id, date, true
}

// dayAccount is what an answer about a day needs from the account.
type dayAccount struct {
	unit      db.TemperatureUnit
	fixedTime fam.Minutes
}

// readDayAccount reads the account's unit and the fixed measurement time
// in effect on date (docs/fam-rules.md §4). It returns errNotOnboarded for
// an account that hasn't onboarded, which includes one with no users row:
// the fixed time is the row onboarding writes, and reading a day creates
// nothing.
func readDayAccount(ctx context.Context, q *db.Queries, id pgtype.UUID, date fam.Date) (dayAccount, error) {
	user, err := onboardedUser(ctx, q, id)
	if err != nil {
		return dayAccount{}, err
	}
	periods, err := q.ListMeasurementTimePeriods(ctx, id)
	if err != nil {
		return dayAccount{}, fmt.Errorf("read the measurement times: %w", err)
	}
	schedule, err := db.ScheduleFromRows(periods)
	if err != nil {
		return dayAccount{}, fmt.Errorf("the measurement times: %w", err)
	}
	return dayAccount{unit: user.TemperatureUnit, fixedTime: schedule.FixedTime(date)}, nil
}

// onboardedUser reads the account's users row. It returns errNotOnboarded
// when the account hasn't onboarded, which includes having no row at all:
// nothing about a day creates one.
func onboardedUser(ctx context.Context, q *db.Queries, id pgtype.UUID) (db.User, error) {
	user, err := q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, errNotOnboarded
	}
	if err != nil {
		return db.User{}, fmt.Errorf("read the user's row: %w", err)
	}
	if !user.OnboardedAt.Valid {
		return db.User{}, errNotOnboarded
	}
	return user, nil
}

// answerWithRow fills res with the day's answer for an entry row.
func answerWithRow(res *dayResponse, date fam.Date, account dayAccount, row db.Entry) error {
	values, err := db.EntryFromRow(row)
	if err != nil {
		return fmt.Errorf("the entry's row: %w", err)
	}
	*res = dayResponseFor(date, account, &values)
	return nil
}

// writeDayFailure answers a failed transaction: 409 for an account that
// hasn't onboarded, otherwise 500 with the cause in the log only.
func writeDayFailure(ctx context.Context, w http.ResponseWriter, what string, err error) {
	if errors.Is(err, errNotOnboarded) {
		writeError(ctx, w, http.StatusConflict,
			apiError{Code: codeNotOnboarded, Message: "the account hasn't onboarded; nothing was read or changed"})
		return
	}
	slog.ErrorContext(ctx, what+": database", "err", err)
	writeInternalError(ctx, w)
}

// dayResponseFor builds the answer for a day. entry is nil for a day
// without one.
func dayResponseFor(date fam.Date, account dayAccount, entry *db.EntryValues) dayResponse {
	res := dayResponse{Date: date.String(), FixedTime: account.fixedTime.String()}
	if entry == nil {
		return res
	}
	e := entryResponse{
		Bleeding:             string(entry.Bleeding),
		SecretionCategory:    textOf(entry.SecretionCategory),
		Sensation:            textOf(entry.Sensation),
		SecretionAmount:      entry.SecretionAmount,
		SecretionDescription: entry.SecretionDescription,
		CervixFirmness:       textOf(entry.CervixFirmness),
		CervixPosition:       textOf(entry.CervixPosition),
		Disturbances:         entry.Disturbances,
		Symptoms:             entry.Symptoms,
		Intercourse:          textOf(entry.Intercourse),
		Notes:                entry.Notes,
	}
	if entry.Reading != nil {
		// The stored thousandths, shown in her unit by fam: nothing is
		// converted anywhere else (docs/fam-rules.md §3.5).
		t := temperatureResponse{
			Hundredths: int(entry.Reading.Temperature.Celsius()),
			Unit:       string(db.TemperatureUnitCelsius),
		}
		if account.unit == db.TemperatureUnitFahrenheit {
			t = temperatureResponse{
				Hundredths: int(entry.Reading.Temperature.Fahrenheit()),
				Unit:       string(db.TemperatureUnitFahrenheit),
			}
		}
		at := entry.Reading.MeasuredAt.String()
		e.Temperature, e.MeasuredAt = &t, &at
	}
	res.Entry = &e
	return res
}

// textOf turns an optional enum value into the optional string JSON
// carries. E is whichever enum type it is given; ~string says "any type
// whose underlying type is string".
func textOf[E ~string](value *E) *string {
	if value == nil {
		return nil
	}
	s := string(*value)
	return &s
}

// dbEnum is any of the enum types sqlc generates from the migrations: a
// string with a Valid method that knows the type's values. So the list of
// values is the migration's, and isn't written a second time here.
type dbEnum interface {
	~string
	Valid() bool
}

// optionalEnum reads an optional field whose value must be one of an
// enum's. Nothing sent is nil; a value that isn't one of them is refused.
// The type in square brackets says which enum: optionalEnum[db.Sensation].
func optionalEnum[E dbEnum](field string, sent *string) (*E, *refusal) {
	if sent == nil {
		return nil, nil
	}
	value := E(*sent)
	if !value.Valid() {
		return nil, refuse(http.StatusUnprocessableEntity, codeInvalidValue, field,
			field+" is not one of its values")
	}
	return &value, nil
}

// optionalText reads an optional free text. Nothing sent, or an empty
// text, is nil: both mean "none". A text over its limit is refused.
func optionalText(field string, sent *string, check func(string) error) (*string, *refusal) {
	if sent == nil || *sent == "" {
		return nil, nil
	}
	if err := check(*sent); err != nil {
		return nil, refuse(http.StatusUnprocessableEntity, codeTooLong, field, field+" is too long")
	}
	return sent, nil
}

// checkDay applies the rules to a decoded request and returns the entry to
// save, or the first thing that stands in the way. It needs no clock and
// no database, so every rule is tested without either.
//
// The order (docs/tech-stack.md §3): what is missing, each field's own
// value, the 20–50 °C rule, and last the questions. A refusal says "change
// this"; a question (409) says "ask her, then send the same again". No
// message repeats a value that was sent.
func checkDay(req dayRequest) (db.EntryValues, *refusal) {
	const (
		broken  = http.StatusBadRequest          // not what the endpoint reads
		refused = http.StatusUnprocessableEntity // a value a rule refuses
		ask     = http.StatusConflict            // a question first
	)
	var out db.EntryValues
	var no *refusal
	var err error

	if req.Entry == nil {
		return out, refuse(broken, codeBadRequest, fieldEntry, "entry is required")
	}
	e := req.Entry

	// Required, and never guessed: an entry saved without it would say "no
	// bleeding" for a field that was merely dropped.
	if e.Bleeding == nil {
		return out, refuse(broken, codeBadRequest, fieldBleeding, "bleeding is required")
	}
	out.Bleeding = db.BleedingLevel(*e.Bleeding)
	if !out.Bleeding.Valid() {
		return out, refuse(refused, codeInvalidValue, fieldBleeding, "bleeding is not one of its values")
	}

	if e.Disturbances == nil {
		return out, refuse(broken, codeBadRequest, fieldDisturbances, "disturbances is required: a list, empty for none")
	}
	if out.Disturbances, err = fam.CheckDisturbances(e.Disturbances); err != nil {
		return out, refuse(refused, codeInvalidValue, fieldDisturbances, "disturbances holds a tag that isn't one, or one twice")
	}
	if e.Symptoms == nil {
		return out, refuse(broken, codeBadRequest, fieldSymptoms, "symptoms is required: a list, empty for none")
	}
	if out.Symptoms, err = fam.CheckSymptoms(e.Symptoms); err != nil {
		return out, refuse(refused, codeInvalidValue, fieldSymptoms, "symptoms holds a tag that isn't one, or one twice")
	}

	if out.SecretionCategory, no = optionalEnum[db.SecretionCategory](fieldSecretionCategory, e.SecretionCategory); no != nil {
		return out, no
	}
	if out.Sensation, no = optionalEnum[db.Sensation](fieldSensation, e.Sensation); no != nil {
		return out, no
	}
	if out.CervixFirmness, no = optionalEnum[db.CervixFirmness](fieldCervixFirmness, e.CervixFirmness); no != nil {
		return out, no
	}
	if out.CervixPosition, no = optionalEnum[db.CervixPosition](fieldCervixPosition, e.CervixPosition); no != nil {
		return out, no
	}
	if out.Intercourse, no = optionalEnum[db.IntercourseRecord](fieldIntercourse, e.Intercourse); no != nil {
		return out, no
	}

	if e.SecretionAmount != nil {
		if *e.SecretionAmount < 1 || *e.SecretionAmount > 3 {
			return out, refuse(refused, codeInvalidValue, fieldSecretionAmount, "secretion_amount must be 1, 2 or 3")
		}
		out.SecretionAmount = e.SecretionAmount
	}

	if out.SecretionDescription, no = optionalText(fieldSecretionDescription, e.SecretionDescription, fam.CheckSecretionDescription); no != nil {
		return out, no
	}
	if out.Notes, no = optionalText(fieldNotes, e.Notes, fam.CheckNotes); no != nil {
		return out, no
	}

	// The temperature and its time come together or not at all.
	if e.Temperature == nil && e.MeasuredAt != nil {
		return out, refuse(refused, codeUnexpectedField, fieldMeasuredAt, "measured_at must be left out when there is no temperature")
	}
	if t := e.Temperature; t != nil {
		if t.Hundredths == nil || t.Unit == nil {
			return out, refuse(broken, codeBadRequest, fieldTemperature, "temperature needs hundredths and unit")
		}
		unit := db.TemperatureUnit(*t.Unit)
		if !unit.Valid() {
			return out, refuse(refused, codeInvalidValue, fieldTemperature, "temperature's unit must be celsius or fahrenheit")
		}
		if e.MeasuredAt == nil {
			return out, refuse(refused, codeMissingField, fieldMeasuredAt, "measured_at is required with a temperature")
		}
		at, err := fam.ParseMinutes(*e.MeasuredAt)
		if err != nil {
			return out, refuse(refused, codeInvalidTime, fieldMeasuredAt, "measured_at must be written HH:MM, from 00:00 to 23:59")
		}

		// AC-17: outside 20.00–50.00 °C is refused here, with words, and
		// never reaches the database's CHECK. The number travels with its
		// unit, so it is never converted as the wrong one.
		var stored fam.MilliC
		if unit == db.TemperatureUnitFahrenheit {
			stored, err = fam.FromFahrenheit(fam.HundredthsF(*t.Hundredths))
		} else {
			stored, err = fam.FromCelsius(fam.HundredthsC(*t.Hundredths))
		}
		if err != nil {
			return out, refuse(refused, codeTemperatureOutOfRange, fieldTemperature, "temperature is outside 20.00–50.00 °C (68.00–122.00 °F)")
		}
		out.Reading = &db.Reading{Temperature: stored, MeasuredAt: at}
	}

	for _, name := range req.Confirmed {
		if name != codeNoTemperature && name != codeUnusualTemperature {
			return out, refuse(refused, codeInvalidValue, fieldConfirmed, "confirmed names a question that doesn't exist")
		}
	}

	// The questions, last: everything else about the request is in order.
	// A confirmation for a question that doesn't arise is ignored.
	if out.Reading == nil && !slices.Contains(req.Confirmed, codeNoTemperature) {
		return out, refuse(ask, codeNoTemperature, fieldTemperature,
			"the entry has no temperature: ask, then send it again with no_temperature in confirmed")
	}
	if out.Reading != nil && out.Reading.Temperature.IsUnusual() && !slices.Contains(req.Confirmed, codeUnusualTemperature) {
		return out, refuse(ask, codeUnusualTemperature, fieldTemperature,
			"the temperature is outside 35.00–39.00 °C: ask, then send it again with unusual_temperature in confirmed")
	}
	return out, nil
}
