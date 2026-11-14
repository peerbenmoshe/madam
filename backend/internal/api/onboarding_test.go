package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/peerbenmoshe/madam/backend/internal/db"
	"github.com/peerbenmoshe/madam/backend/internal/fam"
)

// onboardingNow is the clock every onboarding test runs at: 22:30 UTC on
// 13 November 2026, which is 00:30 on the 14th in Israel. Her today (the
// 14th) is then a day ahead of the server's UTC date, which is the case
// docs/fam-rules.md §9.1 is written for (FD2).
var onboardingNow = time.Date(2026, time.November, 13, 22, 30, 0, 0, time.UTC)

func fixedClock() time.Time { return onboardingNow }

// postOnboarding sends POST /api/onboarding with body as JSON and the
// bearer token (none if empty).
func postOnboarding(t *testing.T, router http.Handler, path, token, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
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

// untouchedStore fails the test if a handler reaches the database: a
// request that validation refuses must never get that far.
type untouchedStore struct{ t *testing.T }

func (s untouchedStore) InUserTx(context.Context, pgtype.UUID, func(*db.Queries) error) error {
	s.t.Helper()
	s.t.Error("the database was reached")
	return errors.New("the database was reached")
}

// TestOnboardingRefuses is every refusal of docs/tech-stack.md §3, each
// with its status, code and field, and none of them reaching the database.
// It needs no database, so plain `make test` runs it.
func TestOnboardingRefuses(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantField  string
	}{
		// 400: not what the endpoint reads.
		{"not JSON", `cycle start: yesterday`, 400, codeBadRequest, ""},
		{"an empty body", ``, 400, codeBadRequest, ""},
		{"a list", `[]`, 400, codeBadRequest, ""},
		{"two objects", `{"cycle_start_unknown":true,"measurement_time":"07:00"}{"cycle_start_unknown":false}`, 400, codeBadRequest, ""},
		{"an unknown field", `{"cycle_start_unknown":true,"measurement_time":"07:00","user_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}`, 400, codeBadRequest, ""},
		{"the flag as text", `{"cycle_start_unknown":"yes","measurement_time":"07:00"}`, 400, codeBadRequest, ""},
		{"the date as a number", `{"cycle_start_unknown":false,"cycle_start_date":20261101,"measurement_time":"07:00"}`, 400, codeBadRequest, ""},
		{"the time as a number", `{"cycle_start_unknown":true,"measurement_time":700}`, 400, codeBadRequest, ""},
		// The flag must be said, never guessed: a dropped field must not
		// become "I don't know" or "I know".
		{"no flag", `{"cycle_start_date":"2026-11-01","measurement_time":"07:00"}`, 400, codeBadRequest, "cycle_start_unknown"},
		{"the flag null", `{"cycle_start_unknown":null,"measurement_time":"07:00"}`, 400, codeBadRequest, "cycle_start_unknown"},
		{"an empty object", `{}`, 400, codeBadRequest, "cycle_start_unknown"},
		{"null", `null`, 400, codeBadRequest, "cycle_start_unknown"},

		// 422: well-formed, and refused by a rule.
		{"I don't know, with a date", `{"cycle_start_unknown":true,"cycle_start_date":"2026-11-01","measurement_time":"07:00"}`, 422, codeUnexpectedField, "cycle_start_date"},
		{"a known start without a date", `{"cycle_start_unknown":false,"measurement_time":"07:00"}`, 422, codeMissingField, "cycle_start_date"},
		{"a known start with a null date", `{"cycle_start_unknown":false,"cycle_start_date":null,"measurement_time":"07:00"}`, 422, codeMissingField, "cycle_start_date"},
		{"no time", `{"cycle_start_unknown":true}`, 422, codeMissingField, "measurement_time"},
		{"a null time", `{"cycle_start_unknown":true,"measurement_time":null}`, 422, codeMissingField, "measurement_time"},

		{"a date not written YYYY-MM-DD", `{"cycle_start_unknown":false,"cycle_start_date":"2026-2-3","measurement_time":"07:00"}`, 422, codeInvalidDate, "cycle_start_date"},
		{"dd/mm/yyyy", `{"cycle_start_unknown":false,"cycle_start_date":"01/11/2026","measurement_time":"07:00"}`, 422, codeInvalidDate, "cycle_start_date"},
		{"an empty date", `{"cycle_start_unknown":false,"cycle_start_date":"","measurement_time":"07:00"}`, 422, codeInvalidDate, "cycle_start_date"},
		{"30 February", `{"cycle_start_unknown":false,"cycle_start_date":"2026-02-30","measurement_time":"07:00"}`, 422, codeInvalidDate, "cycle_start_date"},
		// The clock says 00:30 on the 14th in Israel (FD5): the 15th is
		// tomorrow everywhere.
		{"tomorrow", `{"cycle_start_unknown":false,"cycle_start_date":"2026-11-15","measurement_time":"07:00"}`, 422, codeDateInFuture, "cycle_start_date"},
		{"next year", `{"cycle_start_unknown":false,"cycle_start_date":"2027-11-01","measurement_time":"07:00"}`, 422, codeDateInFuture, "cycle_start_date"},
		{"the day before the floor", `{"cycle_start_unknown":false,"cycle_start_date":"1999-12-31","measurement_time":"07:00"}`, 422, codeDateTooEarly, "cycle_start_date"},
		{"year 0002", `{"cycle_start_unknown":false,"cycle_start_date":"0002-11-01","measurement_time":"07:00"}`, 422, codeDateTooEarly, "cycle_start_date"},

		{"a one-digit hour", `{"cycle_start_unknown":true,"measurement_time":"7:05"}`, 422, codeInvalidTime, "measurement_time"},
		{"a time with seconds", `{"cycle_start_unknown":true,"measurement_time":"07:05:00"}`, 422, codeInvalidTime, "measurement_time"},
		{"24:00", `{"cycle_start_unknown":true,"measurement_time":"24:00"}`, 422, codeInvalidTime, "measurement_time"},
		{"12-hour", `{"cycle_start_unknown":true,"measurement_time":"7:05 AM"}`, 422, codeInvalidTime, "measurement_time"},
		{"an empty time", `{"cycle_start_unknown":true,"measurement_time":""}`, 422, codeInvalidTime, "measurement_time"},
		// A good date doesn't excuse a bad time.
		{"a good date, a bad time", `{"cycle_start_unknown":false,"cycle_start_date":"2026-11-01","measurement_time":"25:00"}`, 422, codeInvalidTime, "measurement_time"},
	}
	router := newRouter(fakeDB{}, untouchedStore{t}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := postOnboarding(t, router, "/api/onboarding", "", tc.body, nil)
			got := decodeAPIError(t, rec, tc.wantStatus)
			if got.Code != tc.wantCode || got.Field != tc.wantField {
				t.Errorf("got code %q field %q, want code %q field %q", got.Code, got.Field, tc.wantCode, tc.wantField)
			}
			if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
			// What was typed never comes back, and so never reaches a log
			// through the message either.
			for _, typed := range []string{"2026-2-3", "01/11/2026", "7:05 AM", "bbbbbbbb"} {
				if strings.Contains(tc.body, typed) && strings.Contains(rec.Body.String(), typed) {
					t.Errorf("the response repeats what was sent: %s", rec.Body.String())
				}
			}
		})
	}
}

// A body must say it is JSON, and must be small.
func TestOnboardingRefusesTheWrongKindOfBody(t *testing.T) {
	router := newRouter(fakeDB{}, untouchedStore{t}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	good := `{"cycle_start_unknown":true,"measurement_time":"07:00"}`

	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded", "application/json-patch+json", "not a media type;;"} {
		t.Run("Content-Type "+contentType, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(good))
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
		big := `{"cycle_start_unknown":true,"measurement_time":"07:00"` + strings.Repeat(" ", onboardingMaxBytes) + `}`
		rec := postOnboarding(t, router, "/api/onboarding", "", big, nil)
		if got := decodeAPIError(t, rec, http.StatusRequestEntityTooLarge); got.Code != codeTooLarge {
			t.Errorf("code = %q, want %q", got.Code, codeTooLarge)
		}
	})

	t.Run("GET is the router's own 405", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/onboarding", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", rec.Code)
		}
	})
}

func TestOnboardingNeedsAToken(t *testing.T) {
	router := newRouter(fakeDB{}, untouchedStore{t}, noAuth, noHook, testConfig, fakeFrontend, fixedClock)
	rec := postOnboarding(t, router, "/api/onboarding", "", `{"cycle_start_unknown":true,"measurement_time":"07:00"}`, nil)
	if got := decodeAPIError(t, rec, http.StatusUnauthorized); got.Code != codeUnauthorized {
		t.Errorf("code = %q, want %q", got.Code, codeUnauthorized)
	}
}

func TestOnboardingHidesDatabaseErrors(t *testing.T) {
	router := newRouter(fakeDB{}, failingStore{}, stubAuthenticator{id: stubUserID}, noHook, testConfig, fakeFrontend, fixedClock)
	rec := postOnboarding(t, router, "/api/onboarding", "", `{"cycle_start_unknown":true,"measurement_time":"07:00"}`, nil)
	if got := decodeAPIError(t, rec, http.StatusInternalServerError); got.Code != codeInternal {
		t.Errorf("code = %q, want %q", got.Code, codeInternal)
	}
	if body := rec.Body.String(); strings.Contains(body, "secret-host") || strings.Contains(body, "refused") {
		t.Errorf("the response leaks the database error: %q", body)
	}
}

// A handler mounted without RequireUser must fail, never guess a user.
func TestOnboardingWithoutRequireUser(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(`{"cycle_start_unknown":true,"measurement_time":"07:00"}`))
	r.Header.Set("Content-Type", "application/json")
	handleOnboarding(untouchedStore{t}, fixedClock)(rec, r)
	if got := decodeAPIError(t, rec, http.StatusInternalServerError); got.Code != codeInternal {
		t.Errorf("code = %q, want %q", got.Code, codeInternal)
	}
}

// --- with a real database, as the application role ---

// stored is everything onboarding writes for one user, read back as her.
type stored struct {
	exists  bool
	user    db.User
	starts  []pgtype.Date
	periods []db.ListMeasurementTimePeriodsRow
}

func readStored(t *testing.T, store *db.Store, id pgtype.UUID) stored {
	t.Helper()
	ctx := t.Context()
	var s stored
	err := store.InUserTx(ctx, id, func(q *db.Queries) error {
		var err error
		if s.user, err = q.GetUser(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		s.exists = true
		if s.starts, err = q.ListCycleStarts(ctx, id); err != nil {
			return err
		}
		s.periods, err = q.ListMeasurementTimePeriods(ctx, id)
		return err
	})
	if err != nil {
		t.Fatalf("read what is stored for %s: %v", id, err)
	}
	return s
}

// onboardingSetup is the real router with the real verifier, a real
// database and the fixed clock.
type onboardingSetup struct {
	store  *db.Store
	router http.Handler
	token  func(pgtype.UUID) string
}

func newOnboardingSetup(t *testing.T) onboardingSetup {
	t.Helper()
	store := testStore(t)
	key, pub := newKey(t, testKid)
	tv := newTestVerifier(t, pub)
	return onboardingSetup{
		store:  store,
		router: newRouter(store, store, tv.JWTVerifier, noHook, testConfig, fakeFrontend, fixedClock),
		token:  func(id pgtype.UUID) string { return tokenFor(t, key, id) },
	}
}

func mustFamDate(t *testing.T, s string) fam.Date {
	t.Helper()
	d, err := fam.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

// wantOnboarded checks a 200 from onboarding: the same body as GET /api/me,
// now with onboarded_at.
func wantOnboarded(t *testing.T, rec *httptest.ResponseRecorder, id pgtype.UUID) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s; want 200", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body meResponse
	dec := json.NewDecoder(rec.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("the body is not the /api/me shape: %v", err)
	}
	if body.ID != id.String() || body.TemperatureUnit != "celsius" || body.OnboardedAt == nil {
		t.Errorf("body = %+v, want id %s, celsius, and onboarded_at set", body, id)
	}
}

// A known start: one cycle_starts row, the answer stored as "known", and
// the fixed time from -infinity.
func TestOnboardingWithAKnownStart(t *testing.T) {
	s := newOnboardingSetup(t)
	a := newUser(t)

	// 14 November is her today at this clock (00:30 in Israel) and still
	// the 13th in UTC: fam-rules §12.9, FD2. The charset is what a browser's
	// fetch may add.
	rec := postOnboarding(t, s.router, "/api/onboarding", s.token(a),
		`{"cycle_start_unknown":false,"cycle_start_date":"2026-11-14","measurement_time":"06:45"}`,
		map[string]string{"Content-Type": "application/json; charset=utf-8"})
	wantOnboarded(t, rec, a)

	got := readStored(t, s.store, a)
	if !got.user.OnboardedAt.Valid || got.user.CycleStartUnknown {
		t.Errorf("users row: onboarded %v, cycle_start_unknown %v; want onboarded and known", got.user.OnboardedAt.Valid, got.user.CycleStartUnknown)
	}
	if len(got.starts) != 1 {
		t.Fatalf("%d cycle starts, want 1", len(got.starts))
	}
	start, err := db.DateFromPg(got.starts[0])
	if err != nil || start != mustFamDate(t, "2026-11-14") {
		t.Errorf("cycle start = %v, %v; want 2026-11-14", start, err)
	}
	if len(got.periods) != 1 || got.periods[0].ValidFrom.InfinityModifier != pgtype.NegativeInfinity {
		t.Fatalf("measurement time periods = %+v, want one row from -infinity", got.periods)
	}

	// What the stored rows mean to the domain logic.
	schedule, err := db.ScheduleFromRows(got.periods)
	if err != nil {
		t.Fatalf("ScheduleFromRows: %v", err)
	}
	if at := schedule.FixedTime(mustFamDate(t, "2020-01-01")); at != 6*60+45 {
		t.Errorf("fixed time on a date before the account = %v, want 06:45", at)
	}
	cycles, err := fam.NewCycles(fam.CycleFacts{Starts: []fam.Date{start}, StartUnknown: got.user.CycleStartUnknown})
	if err != nil {
		t.Fatalf("NewCycles: %v", err)
	}
	if day := cycles.DayOf(start); day.Kind != fam.InCycle || day.Day != 1 || day.Number != 1 {
		t.Errorf("the start date is %+v, want day 1 of cycle 1", day)
	}

	// GET /api/me agrees.
	if status, me := getMe(t, s.router, "/api/me", s.token(a), nil); status != http.StatusOK || me.OnboardedAt == nil {
		t.Errorf("GET /api/me after onboarding: %d, onboarded_at %v", status, me.OnboardedAt)
	}
}

// AC-12, the setup half (docs/spec-v1.md §12): after "I don't know", the
// stored facts are the ones that make her entries show "?" and the stats
// say "Not enough data yet".
func TestOnboardingWithAnUnknownStart(t *testing.T) {
	s := newOnboardingSetup(t)
	a := newUser(t)

	// No GET /api/me first: onboarding creates the users row itself.
	rec := postOnboarding(t, s.router, "/api/onboarding", s.token(a),
		`{"cycle_start_unknown":true,"measurement_time":"07:00"}`, nil)
	wantOnboarded(t, rec, a)

	got := readStored(t, s.store, a)
	if !got.user.OnboardedAt.Valid || !got.user.CycleStartUnknown {
		t.Errorf("users row: onboarded %v, cycle_start_unknown %v; want onboarded and unknown", got.user.OnboardedAt.Valid, got.user.CycleStartUnknown)
	}
	if len(got.starts) != 0 {
		t.Errorf("%d cycle starts, want none", len(got.starts))
	}
	schedule, err := db.ScheduleFromRows(got.periods)
	if err != nil {
		t.Fatalf("ScheduleFromRows: %v", err)
	}
	if at := schedule.FixedTime(mustFamDate(t, "2026-11-14")); at != 7*60 {
		t.Errorf("fixed time = %v, want 07:00", at)
	}

	// An explicit null date is the same answer as no date.
	b := newUser(t)
	rec = postOnboarding(t, s.router, "/api/onboarding", s.token(b),
		`{"cycle_start_unknown":true,"cycle_start_date":null,"measurement_time":"07:00"}`, nil)
	wantOnboarded(t, rec, b)

	// The stored answer, given to the domain logic with a first entry
	// (entries arrive in Slice 4): fam-rules §12.3 C14 and §12.6 A7.
	firstEntry := mustFamDate(t, "2026-11-14")
	cycles, err := fam.NewCycles(fam.CycleFacts{
		StartUnknown: got.user.CycleStartUnknown,
		HasEntries:   true,
		FirstEntry:   firstEntry,
	})
	if err != nil {
		t.Fatalf("NewCycles: %v", err)
	}
	if day := cycles.DayOf(firstEntry.AddDays(3)); day.Kind != fam.Unknown {
		t.Errorf("an entry's cycle day is %v, want unknown (shown as ?)", day.Kind)
	}
	if _, enough := cycles.Stats(); enough {
		t.Errorf("the stats have enough data; want \"Not enough data yet\"")
	}
}

// Onboarding is answered once. A second submit is a 409 and changes
// nothing, also when two arrive at the same moment.
func TestOnboardingHappensOnce(t *testing.T) {
	s := newOnboardingSetup(t)

	t.Run("a second submit is refused and changes nothing", func(t *testing.T) {
		a := newUser(t)
		first := `{"cycle_start_unknown":false,"cycle_start_date":"2026-11-01","measurement_time":"07:00"}`
		wantOnboarded(t, postOnboarding(t, s.router, "/api/onboarding", s.token(a), first, nil), a)
		before := readStored(t, s.store, a)

		for _, second := range []string{
			first, // the same answers
			`{"cycle_start_unknown":false,"cycle_start_date":"2026-11-05","measurement_time":"06:00"}`,
			`{"cycle_start_unknown":true,"measurement_time":"06:00"}`,
		} {
			rec := postOnboarding(t, s.router, "/api/onboarding", s.token(a), second, nil)
			if got := decodeAPIError(t, rec, http.StatusConflict); got.Code != codeAlreadyOnboarded {
				t.Errorf("code = %q, want %q", got.Code, codeAlreadyOnboarded)
			}
		}

		after := readStored(t, s.store, a)
		if after.user != before.user {
			t.Errorf("the users row changed:\n before %+v\n after  %+v", before.user, after.user)
		}
		if len(after.starts) != 1 || after.starts[0] != before.starts[0] {
			t.Errorf("cycle starts changed: %+v, were %+v", after.starts, before.starts)
		}
		if len(after.periods) != 1 || after.periods[0] != before.periods[0] {
			t.Errorf("measurement times changed: %+v, were %+v", after.periods, before.periods)
		}
	})

	t.Run("a refused submit doesn't stop a valid one", func(t *testing.T) {
		a := newUser(t)
		rec := postOnboarding(t, s.router, "/api/onboarding", s.token(a), `{"cycle_start_unknown":true,"measurement_time":"24:00"}`, nil)
		decodeAPIError(t, rec, http.StatusUnprocessableEntity)
		if got := readStored(t, s.store, a); got.exists && got.user.OnboardedAt.Valid {
			t.Fatalf("a refused submit onboarded the user")
		}
		wantOnboarded(t, postOnboarding(t, s.router, "/api/onboarding", s.token(a), `{"cycle_start_unknown":true,"measurement_time":"07:00"}`, nil), a)
	})

	// Two taps, or two devices. Half of the users already have a row and
	// half don't, because the two cases wait on different locks.
	t.Run("many submits at once: exactly one succeeds", func(t *testing.T) {
		const submits = 8
		for round := range 6 {
			a := newUser(t)
			if round%2 == 0 {
				if status, _ := getMe(t, s.router, "/api/me", s.token(a), nil); status != http.StatusOK {
					t.Fatalf("GET /api/me: %d", status)
				}
			}
			token := s.token(a)
			codes := make([]int, submits)
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := range submits {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					// Each submit gives a different answer, so a second
					// success would leave a second row behind.
					body := `{"cycle_start_unknown":false,"cycle_start_date":"2026-11-0` + string(rune('1'+i)) + `","measurement_time":"07:00"}`
					r := httptest.NewRequest(http.MethodPost, "/api/onboarding", strings.NewReader(body))
					r.Header.Set("Content-Type", "application/json")
					r.Header.Set("Authorization", "Bearer "+token)
					rec := httptest.NewRecorder()
					s.router.ServeHTTP(rec, r)
					codes[i] = rec.Code
				}()
			}
			close(start)
			wg.Wait()

			ok, conflict := 0, 0
			for _, code := range codes {
				switch code {
				case http.StatusOK:
					ok++
				case http.StatusConflict:
					conflict++
				}
			}
			if ok != 1 || conflict != submits-1 {
				t.Errorf("round %d: statuses %v; want one 200 and %d 409s", round, codes, submits-1)
			}
			got := readStored(t, s.store, a)
			if len(got.starts) != 1 || len(got.periods) != 1 {
				t.Errorf("round %d: %d cycle starts and %d measurement times, want 1 and 1", round, len(got.starts), len(got.periods))
			}
		}
	})
}

// TestOnboardingIsolation is AC-16, part one, for this endpoint: nothing A
// sends can onboard B or write a row of B's.
func TestOnboardingIsolation(t *testing.T) {
	s := newOnboardingSetup(t)
	a, b := newUser(t), newUser(t)
	tokenA, tokenB := s.token(a), s.token(b)

	// B exists, not onboarded, before A does anything.
	if status, _ := getMe(t, s.router, "/api/me", tokenB, nil); status != http.StatusOK {
		t.Fatalf("GET /api/me as B: %d", status)
	}

	wantUntouched := func(t *testing.T, id pgtype.UUID) {
		t.Helper()
		got := readStored(t, s.store, id)
		if got.user.OnboardedAt.Valid || got.user.CycleStartUnknown || len(got.starts) != 0 || len(got.periods) != 0 {
			t.Errorf("user %s was touched: onboarded %v, unknown %v, %d starts, %d times",
				id, got.user.OnboardedAt.Valid, got.user.CycleStartUnknown, len(got.starts), len(got.periods))
		}
	}

	t.Run("no token: refused, and nothing is written", func(t *testing.T) {
		rec := postOnboarding(t, s.router, "/api/onboarding", "", `{"cycle_start_unknown":true,"measurement_time":"07:00"}`, nil)
		decodeAPIError(t, rec, http.StatusUnauthorized)
		wantUntouched(t, b)
	})

	t.Run("B's id in the body is refused, for A too", func(t *testing.T) {
		for _, field := range []string{"user_id", "id", "sub"} {
			rec := postOnboarding(t, s.router, "/api/onboarding", tokenA,
				`{"cycle_start_unknown":true,"measurement_time":"07:00","`+field+`":"`+b.String()+`"}`, nil)
			decodeAPIError(t, rec, http.StatusBadRequest)
		}
		wantUntouched(t, b)
		if got := readStored(t, s.store, a); got.exists && got.user.OnboardedAt.Valid {
			t.Errorf("a refused request onboarded A")
		}
	})

	t.Run("A names B everywhere else and onboards only A", func(t *testing.T) {
		rec := postOnboarding(t, s.router, "/api/onboarding?user_id="+b.String()+"&id="+b.String(), tokenA,
			`{"cycle_start_unknown":false,"cycle_start_date":"2026-11-01","measurement_time":"06:30"}`,
			map[string]string{
				"X-User-Id":        b.String(),
				"X-Forwarded-User": b.String(),
				"Cookie":           "user_id=" + b.String() + "; sb-access-token=" + tokenB,
			})
		wantOnboarded(t, rec, a)
		wantUntouched(t, b)

		got := readStored(t, s.store, a)
		if len(got.starts) != 1 || len(got.periods) != 1 {
			t.Errorf("A has %d cycle starts and %d measurement times, want 1 and 1", len(got.starts), len(got.periods))
		}
	})

	t.Run("no path leads to another user", func(t *testing.T) {
		for _, path := range []string{"/api/onboarding/" + b.String(), "/api/users/" + b.String() + "/onboarding"} {
			rec := postOnboarding(t, s.router, path, tokenA, `{"cycle_start_unknown":true,"measurement_time":"07:00"}`, nil)
			decodeAPIError(t, rec, http.StatusNotFound)
		}
		wantUntouched(t, b)
	})

	t.Run("B onboards differently, and each has their own", func(t *testing.T) {
		rec := postOnboarding(t, s.router, "/api/onboarding", tokenB, `{"cycle_start_unknown":true,"measurement_time":"08:15"}`, nil)
		wantOnboarded(t, rec, b)

		gotA, gotB := readStored(t, s.store, a), readStored(t, s.store, b)
		if gotA.user.CycleStartUnknown || len(gotA.starts) != 1 {
			t.Errorf("A after B's onboarding: unknown %v, %d starts; want known, 1", gotA.user.CycleStartUnknown, len(gotA.starts))
		}
		if !gotB.user.CycleStartUnknown || len(gotB.starts) != 0 {
			t.Errorf("B: unknown %v, %d starts; want unknown, 0", gotB.user.CycleStartUnknown, len(gotB.starts))
		}
		timeOf := func(got stored) fam.Minutes {
			t.Helper()
			if len(got.periods) != 1 {
				t.Fatalf("%d measurement times, want 1", len(got.periods))
			}
			m, err := db.MinutesFromPg(got.periods[0].MeasurementTime)
			if err != nil {
				t.Fatalf("MinutesFromPg: %v", err)
			}
			return m
		}
		if at := timeOf(gotA); at != 6*60+30 {
			t.Errorf("A's time = %v, want 06:30", at)
		}
		if at := timeOf(gotB); at != 8*60+15 {
			t.Errorf("B's time = %v, want 08:15", at)
		}
	})
}
