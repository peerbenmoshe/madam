package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// The day endpoints (docs/tech-stack.md §3). Every test runs at
// onboardingNow, 22:30 UTC on 13 November 2026: the latest date that has
// begun anywhere is the 14th, so the 14th is accepted and the 15th isn't
// (docs/fam-rules.md §12.9).
const (
	dayToday    = "2026-11-14"
	dayTomorrow = "2026-11-15"
)

// required is the three fields every entry must carry, for building the
// request bodies below.
const required = `"bleeding":"none","disturbances":[],"symptoms":[]`

// entryBody builds a PUT body from the entry's fields and the names in
// "confirmed" (no "confirmed" at all when empty).
func entryBody(fields string, confirmed ...string) string {
	body := `{"entry":{` + fields + `}`
	if len(confirmed) > 0 {
		body += `,"confirmed":["` + strings.Join(confirmed, `","`) + `"]`
	}
	return body + `}`
}

// celsius and fahrenheit are a reading of that many hundredths, at 07:45.
func celsius(hundredths string) string {
	return required + `,"temperature":{"hundredths":` + hundredths + `,"unit":"celsius"},"measured_at":"07:45"`
}

func fahrenheit(hundredths string) string {
	return required + `,"temperature":{"hundredths":` + hundredths + `,"unit":"fahrenheit"},"measured_at":"07:45"`
}

// sendDay sends a request about a day: GET with an empty body, PUT with
// body as JSON. The bearer token is left out when empty.
func sendDay(t *testing.T, router http.Handler, method, path, token, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

// TestPutDayRefuses is every refusal and every question of PUT
// /api/days/{date}, each with its status, code and field, and none of them
// reaching the database. It needs no database, so plain `make test` runs it.
func TestPutDayRefuses(t *testing.T) {
	const (
		no   = codeNoTemperature
		odd  = codeUnusualTemperature
		temp = fieldTemperature
	)
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantField  string
	}{
		// 400: not what the endpoint reads.
		{"not JSON", `36.62 at 07:45`, 400, codeBadRequest, ""},
		{"an empty body", ``, 415, codeUnsupportedMediaType, ""}, // no body, so sendDay sets no Content-Type
		{"a list", `[]`, 400, codeBadRequest, ""},
		{"two objects", entryBody(required, no) + entryBody(required, no), 400, codeBadRequest, ""},
		{"an unknown field beside the entry", `{"entry":{` + required + `},"confirmed":["no_temperature"],"user_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}`, 400, codeBadRequest, ""},
		{"an unknown field in the entry", entryBody(required+`,"user_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"`, no), 400, codeBadRequest, ""},
		{"the date in the entry", entryBody(required+`,"entry_date":"2026-11-01"`, no), 400, codeBadRequest, ""},
		// Slice 5's field: refused until the server knows what it means.
		{"a cycle start in the entry", entryBody(required+`,"cycle_start":true`, no), 400, codeBadRequest, ""},
		{"a stored column in the entry", entryBody(required+`,"temperature_milli_c":36620`, no), 400, codeBadRequest, ""},
		{"an unknown field in the temperature", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius","adjusted":3655},"measured_at":"07:45"`), 400, codeBadRequest, ""},
		{"bleeding as a number", entryBody(`"bleeding":0,"disturbances":[],"symptoms":[]`, no), 400, codeBadRequest, ""},
		{"the temperature as a number", entryBody(required + `,"temperature":36.62,"measured_at":"07:45"`), 400, codeBadRequest, ""},
		{"hundredths as text", entryBody(required + `,"temperature":{"hundredths":"3662","unit":"celsius"},"measured_at":"07:45"`), 400, codeBadRequest, ""},
		{"degrees where hundredths belong", entryBody(celsius("36.62")), 400, codeBadRequest, ""},
		{"disturbances as text", entryBody(`"bleeding":"none","disturbances":"alcohol","symptoms":[]`, no), 400, codeBadRequest, ""},
		{"the amount as text", entryBody(required+`,"secretion_amount":"2"`, no), 400, codeBadRequest, ""},
		{"confirmed as text", `{"entry":{` + required + `},"confirmed":"no_temperature"}`, 400, codeBadRequest, ""},
		{"confirmed as true", `{"entry":{` + required + `},"confirmed":true}`, 400, codeBadRequest, ""},

		// 400, naming the field: something required wasn't sent. A dropped
		// field must never be read as "none".
		{"an empty object", `{}`, 400, codeBadRequest, fieldEntry},
		{"null", `null`, 400, codeBadRequest, fieldEntry},
		{"a null entry", `{"entry":null}`, 400, codeBadRequest, fieldEntry},
		{"no bleeding", entryBody(`"disturbances":[],"symptoms":[]`, no), 400, codeBadRequest, fieldBleeding},
		{"bleeding null", entryBody(`"bleeding":null,"disturbances":[],"symptoms":[]`, no), 400, codeBadRequest, fieldBleeding},
		{"no disturbances", entryBody(`"bleeding":"none","symptoms":[]`, no), 400, codeBadRequest, fieldDisturbances},
		{"disturbances null", entryBody(`"bleeding":"none","disturbances":null,"symptoms":[]`, no), 400, codeBadRequest, fieldDisturbances},
		{"no symptoms", entryBody(`"bleeding":"none","disturbances":[]`, no), 400, codeBadRequest, fieldSymptoms},
		{"symptoms null", entryBody(`"bleeding":"none","disturbances":[],"symptoms":null`, no), 400, codeBadRequest, fieldSymptoms},
		{"a temperature without its unit", entryBody(required + `,"temperature":{"hundredths":3662},"measured_at":"07:45"`), 400, codeBadRequest, temp},
		{"a temperature without its number", entryBody(required + `,"temperature":{"unit":"celsius"},"measured_at":"07:45"`), 400, codeBadRequest, temp},
		{"an empty temperature", entryBody(required + `,"temperature":{},"measured_at":"07:45"`), 400, codeBadRequest, temp},

		// 422: a value that isn't one of the field's values.
		{"a bleeding level that isn't one", entryBody(`"bleeding":"heavy","disturbances":[],"symptoms":[]`, no), 422, codeInvalidValue, fieldBleeding},
		{"bleeding in another spelling", entryBody(`"bleeding":"None","disturbances":[],"symptoms":[]`, no), 422, codeInvalidValue, fieldBleeding},
		{"bleeding empty", entryBody(`"bleeding":"","disturbances":[],"symptoms":[]`, no), 422, codeInvalidValue, fieldBleeding},
		{"an unknown disturbance", entryBody(`"bleeding":"none","disturbances":["alcohol","jet_lag"],"symptoms":[]`, no), 422, codeInvalidValue, fieldDisturbances},
		{"a disturbance twice", entryBody(`"bleeding":"none","disturbances":["alcohol","alcohol"],"symptoms":[]`, no), 422, codeInvalidValue, fieldDisturbances},
		{"a symptom among the disturbances", entryBody(`"bleeding":"none","disturbances":["pms"],"symptoms":[]`, no), 422, codeInvalidValue, fieldDisturbances},
		{"a null disturbance", entryBody(`"bleeding":"none","disturbances":[null],"symptoms":[]`, no), 422, codeInvalidValue, fieldDisturbances},
		{"an unknown symptom", entryBody(`"bleeding":"none","disturbances":[],"symptoms":["headache"]`, no), 422, codeInvalidValue, fieldSymptoms},
		{"a disturbance among the symptoms", entryBody(`"bleeding":"none","disturbances":[],"symptoms":["alcohol"]`, no), 422, codeInvalidValue, fieldSymptoms},
		{"a secretion category that isn't one", entryBody(required+`,"secretion_category":"watery"`, no), 422, codeInvalidValue, fieldSecretionCategory},
		{"the mock's spelling of a category", entryBody(required+`,"secretion_category":"clear"`, no), 422, codeInvalidValue, fieldSecretionCategory},
		{"a sensation that isn't one", entryBody(required+`,"sensation":"slippery"`, no), 422, codeInvalidValue, fieldSensation},
		{"a sensation, empty", entryBody(required+`,"sensation":""`, no), 422, codeInvalidValue, fieldSensation},
		{"a cervix firmness that isn't one", entryBody(required+`,"cervix_firmness":"hard"`, no), 422, codeInvalidValue, fieldCervixFirmness},
		{"a cervix position that isn't one", entryBody(required+`,"cervix_position":"high"`, no), 422, codeInvalidValue, fieldCervixPosition},
		{"an intercourse value that isn't one", entryBody(required+`,"intercourse":"yes"`, no), 422, codeInvalidValue, fieldIntercourse},
		{"amount 0", entryBody(required+`,"secretion_amount":0`, no), 422, codeInvalidValue, fieldSecretionAmount},
		{"amount 4", entryBody(required+`,"secretion_amount":4`, no), 422, codeInvalidValue, fieldSecretionAmount},
		{"amount -1", entryBody(required+`,"secretion_amount":-1`, no), 422, codeInvalidValue, fieldSecretionAmount},
		// 65537 is 1 as a 16-bit number: it must be refused here, by its
		// own value, before any conversion.
		{"amount 65537", entryBody(required+`,"secretion_amount":65537`, no), 422, codeInvalidValue, fieldSecretionAmount},
		{"a unit that isn't one", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"kelvin"},"measured_at":"07:45"`), 422, codeInvalidValue, temp},
		{"a unit in another spelling", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"Celsius"},"measured_at":"07:45"`), 422, codeInvalidValue, temp},
		{"a question that doesn't exist", entryBody(required, "bleeding_stopped"), 422, codeInvalidValue, fieldConfirmed},
		{"a question that doesn't exist, beside one that does", entryBody(required, no, "yes"), 422, codeInvalidValue, fieldConfirmed},

		// 422: text over its limit (docs/fam-rules.md §12.13).
		{"notes of 2,001 characters", entryBody(required+`,"notes":"`+strings.Repeat("a", 2001)+`"`, no), 422, codeTooLong, fieldNotes},
		{"notes of 2,001 Hebrew letters", entryBody(required+`,"notes":"`+strings.Repeat("א", 2001)+`"`, no), 422, codeTooLong, fieldNotes},
		{"a description of 201 characters", entryBody(required+`,"secretion_description":"`+strings.Repeat("a", 201)+`"`, no), 422, codeTooLong, fieldSecretionDescription},

		// 422: the temperature and its time come together.
		{"a time without a temperature", entryBody(required+`,"measured_at":"07:45"`, no), 422, codeUnexpectedField, fieldMeasuredAt},
		{"a time with a null temperature", entryBody(required+`,"temperature":null,"measured_at":"07:45"`, no), 422, codeUnexpectedField, fieldMeasuredAt},
		{"a temperature without a time", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius"}`), 422, codeMissingField, fieldMeasuredAt},
		{"a temperature with a null time", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":null`), 422, codeMissingField, fieldMeasuredAt},
		{"a one-digit hour", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":"7:45"`), 422, codeInvalidTime, fieldMeasuredAt},
		{"a time with seconds", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":"07:45:00"`), 422, codeInvalidTime, fieldMeasuredAt},
		{"24:00", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":"24:00"`), 422, codeInvalidTime, fieldMeasuredAt},
		{"an empty time", entryBody(required + `,"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":""`), 422, codeInvalidTime, fieldMeasuredAt},

		// 422, AC-17: outside 20.00–50.00 °C is refused, never asked about
		// (docs/fam-rules.md §12.11: L8, L10, L16, L18, L19, L20).
		{"L8: 19.99 °C", entryBody(celsius("1999")), 422, codeTemperatureOutOfRange, temp},
		{"L10: 50.01 °C", entryBody(celsius("5001")), 422, codeTemperatureOutOfRange, temp},
		{"L16: 67.99 °F", entryBody(fahrenheit("6799")), 422, codeTemperatureOutOfRange, temp},
		{"L18: 122.01 °F", entryBody(fahrenheit("12201")), 422, codeTemperatureOutOfRange, temp},
		{"L19: a Fahrenheit number sent as Celsius", entryBody(celsius("9791")), 422, codeTemperatureOutOfRange, temp},
		{"L20: a Celsius number sent as Fahrenheit", entryBody(fahrenheit("3662")), 422, codeTemperatureOutOfRange, temp},
		// The unit bugs the rule exists for.
		{"thousandths where hundredths belong", entryBody(celsius("36620")), 422, codeTemperatureOutOfRange, temp},
		{"whole degrees where hundredths belong", entryBody(celsius("37")), 422, codeTemperatureOutOfRange, temp},
		{"zero", entryBody(celsius("0")), 422, codeTemperatureOutOfRange, temp},
		{"below zero", entryBody(celsius("-3662")), 422, codeTemperatureOutOfRange, temp},
		// Saying "yes, it's correct" doesn't get an impossible value in.
		{"50.01 °C, confirmed as unusual", entryBody(celsius("5001"), odd, no), 422, codeTemperatureOutOfRange, temp},

		// 409: nothing is wrong, and she must be asked first.
		{"AC-13: no temperature", entryBody(required), 409, no, temp},
		{"no temperature, an empty confirmed", `{"entry":{` + required + `},"confirmed":[]}`, 409, no, temp},
		{"no temperature, a null one", entryBody(required + `,"temperature":null`), 409, no, temp},
		{"no temperature, confirmed for the other question", entryBody(required, odd), 409, no, temp},
		{"AC-14, L1: 34.20 °C", entryBody(celsius("3420")), 409, odd, temp},
		{"L4: 34.99 °C", entryBody(celsius("3499")), 409, odd, temp},
		{"L6: 39.01 °C", entryBody(celsius("3901")), 409, odd, temp},
		{"L7: 20.00 °C, accepted and unusual", entryBody(celsius("2000")), 409, odd, temp},
		{"L9: 50.00 °C, accepted and unusual", entryBody(celsius("5000")), 409, odd, temp},
		{"L12: 94.99 °F", entryBody(fahrenheit("9499")), 409, odd, temp},
		{"L14: 102.21 °F", entryBody(fahrenheit("10221")), 409, odd, temp},
		{"34.20 °C, confirmed for the other question", entryBody(celsius("3420"), no), 409, odd, temp},

		// The order: a refusal comes before a question, so she is never
		// asked about an entry that can't be saved anyway.
		{"no temperature and a bad value", entryBody(`"bleeding":"heavy","disturbances":[],"symptoms":[]`), 422, codeInvalidValue, fieldBleeding},
		{"an unusual temperature and notes too long", entryBody(celsius("3420") + `,"notes":"` + strings.Repeat("a", 2001) + `"`), 422, codeTooLong, fieldNotes},
		{"an unusual temperature and a question that doesn't exist", entryBody(celsius("3420"), "sure"), 422, codeInvalidValue, fieldConfirmed},
	}
	router := newRouter(fakeDB{}, untouchedStore{t}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := sendDay(t, router, http.MethodPut, "/api/days/"+dayToday, "", tc.body, nil)
			got := decodeAPIError(t, rec, tc.wantStatus)
			if got.Code != tc.wantCode || got.Field != tc.wantField {
				t.Errorf("got code %q field %q, want code %q field %q", got.Code, got.Field, tc.wantCode, tc.wantField)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
			// What was sent never comes back, and so never reaches a log
			// through the message either.
			for _, sent := range []string{"heavy", "jet_lag", "headache", "watery", "slippery", "kelvin", "bbbbbbbb", "aaaa", "אא", "3420", "9791", "36620"} {
				if strings.Contains(tc.body, sent) && strings.Contains(rec.Body.String(), sent) {
					t.Errorf("the response repeats what was sent: %s", rec.Body.String())
				}
			}
		})
	}
}

// TestDayRefusesItsDate: the date in the path goes through the same rule as
// every date the app accepts (docs/fam-rules.md §9.1, §9.2), on GET and on
// PUT, before the body is looked at and before the database.
func TestDayRefusesItsDate(t *testing.T) {
	tests := []struct {
		name     string
		date     string
		wantCode string
	}{
		{"not YYYY-MM-DD", "2026-2-3", codeInvalidDate},
		{"dd-mm-yyyy", "14-11-2026", codeInvalidDate},
		{"a word", "today", codeInvalidDate},
		{"30 February", "2026-02-30", codeInvalidDate},
		{"a date and a time", "2026-11-14T00:00:00Z", codeInvalidDate},
		{"tomorrow (FD5)", dayTomorrow, codeDateInFuture},
		{"next year", "2027-11-14", codeDateInFuture},
		{"the day before the floor (FD7)", "1999-12-31", codeDateTooEarly},
		{"year 0002", "0002-11-14", codeDateTooEarly},
	}
	router := newRouter(fakeDB{}, untouchedStore{t}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	for _, tc := range tests {
		for _, request := range []struct{ method, body string }{
			{http.MethodGet, ""},
			{http.MethodPut, entryBody(celsius("3662"))},
			// The date is refused first, whatever the body is.
			{http.MethodPut, `not JSON at all`},
		} {
			t.Run(tc.name+", "+request.method, func(t *testing.T) {
				rec := sendDay(t, router, request.method, "/api/days/"+tc.date, "", request.body, nil)
				got := decodeAPIError(t, rec, http.StatusUnprocessableEntity)
				if got.Code != tc.wantCode || got.Field != fieldDate {
					t.Errorf("got code %q field %q, want code %q field %q", got.Code, got.Field, tc.wantCode, fieldDate)
				}
				if strings.Contains(rec.Body.String(), tc.date) && tc.wantCode == codeInvalidDate {
					t.Errorf("the response repeats what was sent: %s", rec.Body.String())
				}
			})
		}
	}
}

// A body must say it is JSON, and must be small; and only the methods that
// exist are answered.
func TestPutDayRefusesTheWrongKindOfBody(t *testing.T) {
	router := newRouter(fakeDB{}, untouchedStore{t}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	good := entryBody(celsius("3662"))

	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		t.Run("Content-Type "+contentType, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/api/days/"+dayToday, strings.NewReader(good))
			if contentType != "" {
				r.Header.Set("Content-Type", contentType)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, r)
			if got := decodeAPIError(t, rec, http.StatusUnsupportedMediaType); got.Code != codeUnsupportedMediaType {
				t.Errorf("code = %q, want %q", got.Code, codeUnsupportedMediaType)
			}
		})
	}

	t.Run("a body over the cap", func(t *testing.T) {
		// Valid JSON, so only its size can be the reason.
		big := `{"entry":{` + celsius("3662") + `}` + strings.Repeat(" ", dayMaxBytes) + `}`
		rec := sendDay(t, router, http.MethodPut, "/api/days/"+dayToday, "", big, nil)
		if got := decodeAPIError(t, rec, http.StatusRequestEntityTooLarge); got.Code != codeTooLarge {
			t.Errorf("code = %q, want %q", got.Code, codeTooLarge)
		}
	})

	// The cap leaves room for both texts at their limits, with every
	// character written the longest way JSON has: a 12-byte \u pair.
	t.Run("the largest valid body fits under the cap", func(t *testing.T) {
		emoji := `😀` // one character
		largest := entryBody(celsius("3662") +
			`,"notes":"` + strings.Repeat(emoji, fam.MaxNotesLength) + `"` +
			`,"secretion_description":"` + strings.Repeat(emoji, fam.MaxSecretionDescriptionLength) + `"` +
			`,"secretion_category":"clear_stretchy","sensation":"creamy","secretion_amount":3` +
			`,"cervix_firmness":"firm","cervix_position":"low_closed","intercourse":"unprotected"`)
		if len(largest) >= dayMaxBytes {
			t.Fatalf("the largest valid body is %d bytes, the cap is %d", len(largest), dayMaxBytes)
		}
		var req dayRequest
		if err := json.Unmarshal([]byte(largest), &req); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		if _, refused := checkDay(req); refused != nil {
			t.Errorf("refused: %+v", refused)
		}
	})

	t.Run("POST and DELETE are the router's own 405", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodDelete, http.MethodPatch} {
			rec := sendDay(t, router, method, "/api/days/"+dayToday, "", good, nil)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: status = %d, want 405", method, rec.Code)
			}
		}
	})

	t.Run("no path below or beside a day", func(t *testing.T) {
		for _, path := range []string{"/api/days", "/api/days/", "/api/days/" + dayToday + "/", "/api/days/" + dayToday + "/notes"} {
			rec := sendDay(t, router, http.MethodGet, path, "", "", nil)
			decodeAPIError(t, rec, http.StatusNotFound)
		}
	})
}

func TestDayNeedsAToken(t *testing.T) {
	router := newRouter(fakeDB{}, untouchedStore{t}, noAuth, noHook, testConfig, fakeFrontend, fixedClock)
	for _, request := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPut, entryBody(celsius("3662"))},
	} {
		rec := sendDay(t, router, request.method, "/api/days/"+dayToday, "", request.body, nil)
		if got := decodeAPIError(t, rec, http.StatusUnauthorized); got.Code != codeUnauthorized {
			t.Errorf("%s: code = %q, want %q", request.method, got.Code, codeUnauthorized)
		}
	}
}

func TestDayHidesDatabaseErrors(t *testing.T) {
	router := newRouter(fakeDB{}, failingStore{}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	for _, request := range []struct{ method, body string }{
		{http.MethodGet, ""},
		{http.MethodPut, entryBody(celsius("3662"))},
	} {
		rec := sendDay(t, router, request.method, "/api/days/"+dayToday, "", request.body, nil)
		if got := decodeAPIError(t, rec, http.StatusInternalServerError); got.Code != codeInternal {
			t.Errorf("%s: code = %q, want %q", request.method, got.Code, codeInternal)
		}
		if body := rec.Body.String(); strings.Contains(body, "secret-host") || strings.Contains(body, "refused") {
			t.Errorf("%s: the response leaks the database error: %q", request.method, body)
		}
	}
}

// A handler mounted without RequireUser must fail, never guess a user.
func TestDayWithoutRequireUser(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"GET": handleGetDay(untouchedStore{t}, fixedClock),
		"PUT": handlePutDay(untouchedStore{t}, fixedClock),
	} {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(name, "/api/days/"+dayToday, strings.NewReader(entryBody(celsius("3662"))))
		r.Header.Set("Content-Type", "application/json")
		handler(rec, r)
		if got := decodeAPIError(t, rec, http.StatusInternalServerError); got.Code != codeInternal {
			t.Errorf("%s: code = %q, want %q", name, got.Code, codeInternal)
		}
	}
}

// TestDayResponseInFahrenheit: an account that views °F is answered in °F,
// converted by fam from the stored thousandths, so what she typed comes
// back (docs/fam-rules.md §12.2, F1). Nothing can set an account's unit
// until Settings (Slice 9), so this is asked of the function, not of the
// endpoint.
func TestDayResponseInFahrenheit(t *testing.T) {
	date := mustFamDate(t, dayToday)
	entry := db.EntryValues{
		Reading:      &db.Reading{Temperature: 36617, MeasuredAt: 7*60 + 45}, // 97.91 °F as stored
		Bleeding:     db.BleedingLevelNone,
		Disturbances: []string{},
		Symptoms:     []string{},
	}
	for _, tc := range []struct {
		unit db.TemperatureUnit
		want temperatureResponse
	}{
		{db.TemperatureUnitFahrenheit, temperatureResponse{Hundredths: 9791, Unit: "fahrenheit"}},
		{db.TemperatureUnitCelsius, temperatureResponse{Hundredths: 3662, Unit: "celsius"}},
	} {
		res := dayResponseFor(date, dayAccount{unit: tc.unit, fixedTime: 7 * 60}, &entry)
		if res.Entry == nil || res.Entry.Temperature == nil || *res.Entry.Temperature != tc.want {
			t.Errorf("%s: temperature = %+v, want %+v", tc.unit, res.Entry.Temperature, tc.want)
		}
		if res.Entry.MeasuredAt == nil || *res.Entry.MeasuredAt != "07:45" || res.FixedTime != "07:00" {
			t.Errorf("%s: measured_at %v, fixed_time %q", tc.unit, res.Entry.MeasuredAt, res.FixedTime)
		}
	}
}

// --- with a real database, as the application role ---

// onboard onboards id through the API with the given fixed time, and no
// known cycle start.
func onboard(t *testing.T, s onboardingSetup, id pgtype.UUID, fixedTime string) {
	t.Helper()
	rec := postOnboarding(t, s.router, "/api/onboarding", s.token(id),
		`{"cycle_start_unknown":true,"measurement_time":"`+fixedTime+`"}`, nil)
	wantOnboarded(t, rec, id)
}

// wantDay checks a 200 about a day and that its body is exactly wantJSON,
// whatever the order of the keys. Comparing the JSON, and not a decoded
// struct, is what notices a null that should be absent, or a key too many.
func wantDay(t *testing.T, rec *httptest.ResponseRecorder, wantJSON string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s; want 200", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	var got, want any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("the expected JSON is not JSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("body\n got %s\nwant %s", rec.Body.String(), wantJSON)
	}
}

// storedEntry reads id's entry for date straight from the database, as
// her. found is false when the day has none.
func storedEntry(t *testing.T, store *db.Store, id pgtype.UUID, date string) (row db.Entry, found bool) {
	t.Helper()
	err := store.InUserTx(t.Context(), id, func(q *db.Queries) error {
		var err error
		row, err = q.GetEntry(t.Context(), db.GetEntryParams{UserID: id, EntryDate: db.DateToPg(mustFamDate(t, date))})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return db.Entry{}, false
	}
	if err != nil {
		t.Fatalf("GetEntry(%s, %s): %v", id, date, err)
	}
	return row, true
}

// The entry every field of which is filled in, as a client sends it and as
// the API answers it. The disturbances are sent out of order on purpose.
const (
	fullEntrySent = `"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":"07:45",` +
		`"bleeding":"spotting","secretion_category":"clear_stretchy","sensation":"wet",` +
		`"secretion_amount":2,"secretion_description":"clear, stretches",` +
		`"cervix_firmness":"soft","cervix_position":"high_open",` +
		`"disturbances":["poor_sleep","alcohol"],"symptoms":["pms","bloating"],` +
		`"intercourse":"protected","notes":"line one\nline two"`
	fullEntryAnswered = `{"temperature":{"hundredths":3662,"unit":"celsius"},"measured_at":"07:45",` +
		`"bleeding":"spotting","secretion_category":"clear_stretchy","sensation":"wet",` +
		`"secretion_amount":2,"secretion_description":"clear, stretches",` +
		`"cervix_firmness":"soft","cervix_position":"high_open",` +
		`"disturbances":["alcohol","poor_sleep"],"symptoms":["bloating","pms"],` +
		`"intercourse":"protected","notes":"line one\nline two"}`
	// An entry with nothing but what is required.
	emptyEntryAnswered = `{"temperature":null,"measured_at":null,"bleeding":"none",` +
		`"secretion_category":null,"sensation":null,"secretion_amount":null,"secretion_description":null,` +
		`"cervix_firmness":null,"cervix_position":null,"disturbances":[],"symptoms":[],` +
		`"intercourse":null,"notes":null}`
)

// TestDaySaveAndRead saves a day and reads it back, through the API and
// from the database: the whole entry, then a second save of the same date
// (AC-4: one entry per date).
func TestDaySaveAndRead(t *testing.T) {
	s := newOnboardingSetup(t)
	a := newUser(t)
	onboard(t, s, a, "06:30")
	token := s.token(a)
	path := "/api/days/" + dayToday

	t.Run("a day with no entry", func(t *testing.T) {
		rec := sendDay(t, s.router, http.MethodGet, path, token, "", nil)
		wantDay(t, rec, `{"date":"2026-11-14","fixed_time":"06:30","entry":null}`)
		if _, found := storedEntry(t, s.store, a, dayToday); found {
			t.Error("reading a day created an entry")
		}
	})

	t.Run("every field, saved and read back", func(t *testing.T) {
		want := `{"date":"2026-11-14","fixed_time":"06:30","entry":` + fullEntryAnswered + `}`
		// The charset is what a browser's fetch may add.
		rec := sendDay(t, s.router, http.MethodPut, path, token, entryBody(fullEntrySent),
			map[string]string{"Content-Type": "application/json; charset=utf-8"})
		wantDay(t, rec, want)
		wantDay(t, sendDay(t, s.router, http.MethodGet, path, token, "", nil), want)

		// And what the database holds: thousandths, minutes, §7's order.
		row, found := storedEntry(t, s.store, a, dayToday)
		if !found {
			t.Fatal("nothing was stored")
		}
		got, err := db.EntryFromRow(row)
		if err != nil {
			t.Fatalf("EntryFromRow: %v", err)
		}
		if got.Reading == nil || got.Reading.Temperature != 36620 || got.Reading.MeasuredAt != 7*60+45 {
			t.Errorf("stored reading = %+v, want 36620 thousandths at minute 465", got.Reading)
		}
		if !reflect.DeepEqual(got.Disturbances, []string{"alcohol", "poor_sleep"}) || !reflect.DeepEqual(got.Symptoms, []string{"bloating", "pms"}) {
			t.Errorf("stored tags = %v and %v, want them in fam-rules §7's order", got.Disturbances, got.Symptoms)
		}
		if got.Notes == nil || *got.Notes != "line one\nline two" {
			t.Errorf("stored notes = %v, want both lines", got.Notes)
		}
	})

	// AC-4: the date has an entry, so saving it again changes that entry.
	// Every field the second save leaves out is emptied, not kept.
	t.Run("a second save of the date replaces the first", func(t *testing.T) {
		before, _ := storedEntry(t, s.store, a, dayToday)
		want := `{"date":"2026-11-14","fixed_time":"06:30","entry":` + emptyEntryAnswered + `}`
		rec := sendDay(t, s.router, http.MethodPut, path, token, entryBody(required, codeNoTemperature), nil)
		wantDay(t, rec, want)
		wantDay(t, sendDay(t, s.router, http.MethodGet, path, token, "", nil), want)

		after, found := storedEntry(t, s.store, a, dayToday)
		if !found {
			t.Fatal("the entry is gone")
		}
		if after.TemperatureMilliC.Valid || after.MeasuredAt.Valid || after.Notes.Valid || after.Intercourse.Valid || len(after.Disturbances) != 0 {
			t.Errorf("the first save's values are still stored: %+v", after)
		}
		if !after.CreatedAt.Time.Equal(before.CreatedAt.Time) || !after.UpdatedAt.Time.After(before.UpdatedAt.Time) {
			t.Errorf("created_at %v → %v (want unchanged), updated_at %v → %v (want later)",
				before.CreatedAt.Time, after.CreatedAt.Time, before.UpdatedAt.Time, after.UpdatedAt.Time)
		}
	})

	t.Run("an empty text is stored as none", func(t *testing.T) {
		rec := sendDay(t, s.router, http.MethodPut, path, token,
			entryBody(required+`,"notes":"","secretion_description":""`, codeNoTemperature), nil)
		wantDay(t, rec, `{"date":"2026-11-14","fixed_time":"06:30","entry":`+emptyEntryAnswered+`}`)
		row, _ := storedEntry(t, s.store, a, dayToday)
		if row.Notes.Valid || row.SecretionDescription.Valid {
			t.Errorf("stored notes %+v and description %+v, want both NULL", row.Notes, row.SecretionDescription)
		}
	})

	t.Run("texts at their limits are saved whole", func(t *testing.T) {
		notes, description := strings.Repeat("א", fam.MaxNotesLength), strings.Repeat("a", fam.MaxSecretionDescriptionLength)
		rec := sendDay(t, s.router, http.MethodPut, path, token,
			entryBody(required+`,"notes":"`+notes+`","secretion_description":"`+description+`"`, codeNoTemperature), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		row, _ := storedEntry(t, s.store, a, dayToday)
		if row.Notes.String != notes || row.SecretionDescription.String != description {
			t.Errorf("stored %d and %d bytes, want %d and %d", len(row.Notes.String), len(row.SecretionDescription.String), len(notes), len(description))
		}
	})

	// A Fahrenheit reading is stored in thousandths of °C, and this
	// account, which views °C, is shown it in °C (fam-rules §12.2, F1).
	t.Run("a Fahrenheit reading, on a Celsius account", func(t *testing.T) {
		rec := sendDay(t, s.router, http.MethodPut, path, token, entryBody(fahrenheit("9791")), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		row, _ := storedEntry(t, s.store, a, dayToday)
		if row.TemperatureMilliC.Int32 != 36617 {
			t.Errorf("stored %d thousandths, want 36617", row.TemperatureMilliC.Int32)
		}
		var body dayResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		if body.Entry == nil || body.Entry.Temperature == nil || *body.Entry.Temperature != (temperatureResponse{Hundredths: 3662, Unit: "celsius"}) {
			t.Errorf("answered %+v, want 36.62 °C", body.Entry)
		}
	})

	// Any past date can be filled in, and its fixed time is the one from
	// onboarding, which applies to every date before the account existed
	// (docs/fam-rules.md §12.8, M4).
	t.Run("a date years back", func(t *testing.T) {
		old := "/api/days/2020-01-01"
		wantDay(t, sendDay(t, s.router, http.MethodGet, old, token, "", nil),
			`{"date":"2020-01-01","fixed_time":"06:30","entry":null}`)
		rec := sendDay(t, s.router, http.MethodPut, old, token, entryBody(celsius("3662")), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		// And it didn't touch today's.
		if row, _ := storedEntry(t, s.store, a, dayToday); row.TemperatureMilliC.Int32 != 36617 {
			t.Errorf("today's entry changed when 2020-01-01 was saved: %+v", row)
		}
	})

	// 1 January 2000 is the floor itself (FD8).
	t.Run("the earliest date", func(t *testing.T) {
		rec := sendDay(t, s.router, http.MethodPut, "/api/days/2000-01-01", token, entryBody(celsius("3662")), nil)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, body %s; want 200", rec.Code, rec.Body.String())
		}
	})
}

// TestDayQuestions is AC-13, AC-14 and AC-17 against a real database: a
// question stores nothing until it is answered, and a refusal stores
// nothing at all.
func TestDayQuestions(t *testing.T) {
	s := newOnboardingSetup(t)
	a := newUser(t)
	onboard(t, s, a, "07:00")
	token := s.token(a)
	path := "/api/days/" + dayToday

	wantNothingStored := func(t *testing.T) {
		t.Helper()
		if row, found := storedEntry(t, s.store, a, dayToday); found {
			t.Errorf("an entry was stored: %+v", row)
		}
	}
	ask := func(t *testing.T, body, wantCode string) {
		t.Helper()
		rec := sendDay(t, s.router, http.MethodPut, path, token, body, nil)
		if got := decodeAPIError(t, rec, http.StatusConflict); got.Code != wantCode || got.Field != fieldTemperature {
			t.Errorf("got code %q field %q, want %q about the temperature", got.Code, got.Field, wantCode)
		}
	}

	t.Run("AC-17: an impossible temperature is refused and nothing is stored", func(t *testing.T) {
		for _, body := range []string{
			entryBody(celsius("5001")),
			entryBody(celsius("5001"), codeUnusualTemperature),
			entryBody(celsius("36620"), codeUnusualTemperature),
		} {
			rec := sendDay(t, s.router, http.MethodPut, path, token, body, nil)
			if got := decodeAPIError(t, rec, http.StatusUnprocessableEntity); got.Code != codeTemperatureOutOfRange {
				t.Errorf("code = %q, want %q", got.Code, codeTemperatureOutOfRange)
			}
		}
		wantNothingStored(t)
	})

	t.Run("AC-14: 34.20 °C is asked about, and saved only after yes", func(t *testing.T) {
		ask(t, entryBody(celsius("3420")), codeUnusualTemperature)
		wantNothingStored(t)

		// "No, fix it" is simply not sending it again. "Yes, save it":
		rec := sendDay(t, s.router, http.MethodPut, path, token, entryBody(celsius("3420"), codeUnusualTemperature), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s; want 200", rec.Code, rec.Body.String())
		}
		row, found := storedEntry(t, s.store, a, dayToday)
		if !found || row.TemperatureMilliC.Int32 != 34200 {
			t.Errorf("stored: found %v, %d thousandths; want 34200", found, row.TemperatureMilliC.Int32)
		}
	})

	t.Run("AC-13: no temperature is asked about, and saved only after yes", func(t *testing.T) {
		// On another date, which has no entry yet.
		other, otherPath := "2026-11-13", "/api/days/2026-11-13"
		rec := sendDay(t, s.router, http.MethodPut, otherPath, token, entryBody(required), nil)
		if got := decodeAPIError(t, rec, http.StatusConflict); got.Code != codeNoTemperature {
			t.Errorf("code = %q, want %q", got.Code, codeNoTemperature)
		}
		if _, found := storedEntry(t, s.store, a, other); found {
			t.Error("an entry was stored before she answered")
		}

		rec = sendDay(t, s.router, http.MethodPut, otherPath, token, entryBody(required, codeNoTemperature), nil)
		wantDay(t, rec, `{"date":"2026-11-13","fixed_time":"07:00","entry":`+emptyEntryAnswered+`}`)
		row, found := storedEntry(t, s.store, a, other)
		if !found || row.TemperatureMilliC.Valid || row.MeasuredAt.Valid {
			t.Errorf("stored: found %v, temperature %+v, time %+v; want an entry with neither", found, row.TemperatureMilliC, row.MeasuredAt)
		}
	})

	t.Run("a question over an existing entry leaves it as it was", func(t *testing.T) {
		before, _ := storedEntry(t, s.store, a, dayToday)
		ask(t, entryBody(celsius("3901")), codeUnusualTemperature)
		ask(t, entryBody(required), codeNoTemperature)
		after, _ := storedEntry(t, s.store, a, dayToday)
		if !reflect.DeepEqual(before, after) {
			t.Errorf("the entry changed while a question was open:\n was %+v\n now %+v", before, after)
		}
	})

	t.Run("a usual temperature needs no answer, and one given anyway is ignored", func(t *testing.T) {
		for _, body := range []string{
			entryBody(celsius("3662")),
			entryBody(celsius("3500")), // L3: the edge itself
			entryBody(celsius("3900")), // L5
			entryBody(fahrenheit("9500")),
			entryBody(celsius("3662"), codeUnusualTemperature, codeNoTemperature),
		} {
			if rec := sendDay(t, s.router, http.MethodPut, path, token, body, nil); rec.Code != http.StatusOK {
				t.Errorf("status = %d, body %s; want 200 for %s", rec.Code, rec.Body.String(), body)
			}
		}
	})
}

// TestDayNeedsAnOnboardedAccount: the fixed time comes from the row
// onboarding writes, so until then a day can be neither read nor saved,
// and neither request creates anything.
func TestDayNeedsAnOnboardedAccount(t *testing.T) {
	s := newOnboardingSetup(t)
	path := "/api/days/" + dayToday

	refused := func(t *testing.T, id pgtype.UUID) {
		t.Helper()
		for _, request := range []struct{ method, body string }{
			{http.MethodGet, ""},
			{http.MethodPut, entryBody(celsius("3662"))},
			{http.MethodPut, entryBody(required, codeNoTemperature)},
		} {
			rec := sendDay(t, s.router, request.method, path, s.token(id), request.body, nil)
			if got := decodeAPIError(t, rec, http.StatusConflict); got.Code != codeNotOnboarded || got.Field != "" {
				t.Errorf("%s: got code %q field %q, want %q", request.method, got.Code, got.Field, codeNotOnboarded)
			}
		}
	}

	t.Run("an account that has never called the API", func(t *testing.T) {
		a := newUser(t)
		refused(t, a)
		if userExists(t, s.store, a) {
			t.Error("asking about a day created the users row")
		}
	})

	t.Run("an account that exists and hasn't onboarded", func(t *testing.T) {
		a := newUser(t)
		if status, _ := getMe(t, s.router, "/api/me", s.token(a), nil); status != http.StatusOK {
			t.Fatalf("GET /api/me: %d", status)
		}
		refused(t, a)
		if _, found := storedEntry(t, s.store, a, dayToday); found {
			t.Error("an entry was stored for an account that hasn't onboarded")
		}

		// The rules still come first: a bad request is told what is wrong
		// with it, onboarded or not.
		rec := sendDay(t, s.router, http.MethodPut, path, s.token(a), entryBody(celsius("5001")), nil)
		if got := decodeAPIError(t, rec, http.StatusUnprocessableEntity); got.Code != codeTemperatureOutOfRange {
			t.Errorf("code = %q, want %q", got.Code, codeTemperatureOutOfRange)
		}

		// And once she has onboarded, the same requests work.
		onboard(t, s, a, "07:00")
		if rec := sendDay(t, s.router, http.MethodPut, path, s.token(a), entryBody(celsius("3662")), nil); rec.Code != http.StatusOK {
			t.Errorf("after onboarding: status = %d, body %s", rec.Code, rec.Body.String())
		}
	})
}

// TestDayIsolation is AC-16, part one, for the day endpoints
// (docs/spec-v1.md §12; docs/fam-rules.md §12.7, I1 and I2): no request
// from user A can read or change user B's entry. The same cases against
// the database alone are internal/db's TestEntryIsolationByDate.
func TestDayIsolation(t *testing.T) {
	s := newOnboardingSetup(t)
	a, b := newUser(t), newUser(t)
	onboard(t, s, a, "06:30")
	onboard(t, s, b, "08:15")
	tokenA, tokenB := s.token(a), s.token(b)
	path := "/api/days/" + dayToday

	// B's entry, which nothing A does may show or change.
	wantB := `{"date":"2026-11-14","fixed_time":"08:15","entry":` + fullEntryAnswered + `}`
	wantDay(t, sendDay(t, s.router, http.MethodPut, path, tokenB, entryBody(fullEntrySent), nil), wantB)
	bStored, _ := storedEntry(t, s.store, b, dayToday)

	wantBUntouched := func(t *testing.T) {
		t.Helper()
		now, found := storedEntry(t, s.store, b, dayToday)
		if !found || !reflect.DeepEqual(now, bStored) {
			t.Errorf("B's entry changed:\n was %+v\n now %+v (found %v)", bStored, now, found)
		}
	}
	// What A must see on a day she has no entry for: nothing of B's, and
	// her own fixed time, not B's.
	const aEmpty = `{"date":"2026-11-14","fixed_time":"06:30","entry":null}`

	t.Run("I1: A reads B's date and finds no entry", func(t *testing.T) {
		wantDay(t, sendDay(t, s.router, http.MethodGet, path, tokenA, "", nil), aEmpty)
	})

	t.Run("A names B everywhere a request can, and still reads her own", func(t *testing.T) {
		rec := sendDay(t, s.router, http.MethodGet, path+"?user_id="+b.String()+"&id="+b.String(), tokenA, "",
			map[string]string{
				"X-User-Id":        b.String(),
				"X-Forwarded-User": b.String(),
				"Cookie":           "user_id=" + b.String() + "; sb-access-token=" + tokenB,
			})
		wantDay(t, rec, aEmpty)
		if strings.Contains(rec.Body.String(), "clear, stretches") || strings.Contains(rec.Body.String(), "08:15") {
			t.Errorf("A's answer holds something of B's: %s", rec.Body.String())
		}
	})

	t.Run("no token: refused, and nothing changes", func(t *testing.T) {
		decodeAPIError(t, sendDay(t, s.router, http.MethodGet, path, "", "", nil), http.StatusUnauthorized)
		decodeAPIError(t, sendDay(t, s.router, http.MethodPut, path, "", entryBody(celsius("3662")), nil), http.StatusUnauthorized)
		wantBUntouched(t)
	})

	t.Run("B's id in the body is refused", func(t *testing.T) {
		for _, body := range []string{
			`{"entry":{` + celsius("3662") + `},"user_id":"` + b.String() + `"}`,
			`{"entry":{` + celsius("3662") + `,"user_id":"` + b.String() + `"}}`,
			`{"entry":{` + celsius("3662") + `},"id":"` + b.String() + `"}`,
		} {
			decodeAPIError(t, sendDay(t, s.router, http.MethodPut, path, tokenA, body, nil), http.StatusBadRequest)
		}
		wantBUntouched(t)
		if _, found := storedEntry(t, s.store, a, dayToday); found {
			t.Error("a refused request stored an entry for A")
		}
	})

	t.Run("no path leads to another user's day", func(t *testing.T) {
		for _, p := range []string{
			"/api/users/" + b.String() + "/days/" + dayToday,
			"/api/days/" + dayToday + "/" + b.String(),
			"/api/days/" + b.String(),
		} {
			rec := sendDay(t, s.router, http.MethodGet, p, tokenA, "", nil)
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("GET %s: status = %d, want 404 or 422", p, rec.Code)
			}
			if strings.Contains(rec.Body.String(), "clear, stretches") {
				t.Errorf("GET %s shows B's entry: %s", p, rec.Body.String())
			}
		}
	})

	// I2: A saves the date B has an entry on, naming B wherever she can.
	// It creates A's own entry. B's is untouched.
	t.Run("I2: A saves B's date and gets her own entry", func(t *testing.T) {
		rec := sendDay(t, s.router, http.MethodPut, path+"?user_id="+b.String(), tokenA,
			entryBody(required+`,"notes":"A's own"`, codeNoTemperature),
			map[string]string{"X-User-Id": b.String(), "Cookie": "user_id=" + b.String()})
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s; want 200", rec.Code, rec.Body.String())
		}
		wantBUntouched(t)
		row, found := storedEntry(t, s.store, a, dayToday)
		if !found || row.Notes.String != "A's own" {
			t.Errorf("A's entry: found %v, notes %q", found, row.Notes.String)
		}
	})

	t.Run("each reads her own afterwards", func(t *testing.T) {
		wantDay(t, sendDay(t, s.router, http.MethodGet, path, tokenB, "", nil), wantB)
		var body dayResponse
		rec := sendDay(t, s.router, http.MethodGet, path, tokenA, "", nil)
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		if body.FixedTime != "06:30" || body.Entry == nil || body.Entry.Notes == nil || *body.Entry.Notes != "A's own" || body.Entry.Temperature != nil {
			t.Errorf("A reads %s, want her own entry and her own fixed time", rec.Body.String())
		}
	})
}
