// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/philonet/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/philonet/internal/store"
)

// fakeToken builds an unsigned JWT-shaped token carrying a uid claim; the CLI
// only decodes the claim and the fake API never verifies it.
func fakeToken(uid string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(`{"uid":"`+uid+`"}`)) + ".sig"
}

type fakePhilonet struct {
	srv           *httptest.Server
	hits          atomic.Int64
	failStandings atomic.Bool
}

func newFakePhilonet(t *testing.T) *fakePhilonet {
	t.Helper()
	f := &fakePhilonet{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "feed2/forme") {
			_, _ = w.Write([]byte(`{"success":true,"has_more":false,"items":[{"article":{"id":1,"title":"Secret A article","url":"https://e.com/a","tags":["private"]},"conversation_starters":[{"id":11,"content":"account A private thought","starter":{"user_id":"uA","name":"Alice","is_friend":true}}]}]}`))
			return
		}
		if strings.Contains(r.URL.Path, "myprofilestats") {
			_, _ = w.Write([]byte(`{"success":true,"data":{"timezone":"Pacific/Auckland","streak":{"current":3,"max":9},"reading":{"last_7_days_seconds":600},"daily_last_7":[{"date":"2026-09-29","seconds":600}]}}`))
			return
		}
		if strings.Contains(r.URL.Path, "friendsstandings") {
			if f.failStandings.Load() {
				w.WriteHeader(http.StatusBadRequest) // 4xx fails fast; 5xx would burn the client's retry budget
				_, _ = w.Write([]byte(`{"error":"boom"}`))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"my_rank":2,"total_participants":5,"entries":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// runCLI executes the root command with the given token and returns stdout.
func runCLI(t *testing.T, f *fakePhilonet, token string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("PHILONET_TOKEN", token)
	t.Setenv("PHILONET_BASE_URL", f.srv.URL)
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), err
}

func peopleIn(t *testing.T, out string) int {
	t.Helper()
	var env struct {
		Results struct {
			People []any `json:"people"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	return len(env.Results.People)
}

// Finding 1: two accounts sharing one database file (the generator's legacy
// unscoped data.db fallback) must never see each other's history.
func TestHistoryIsolatedBetweenAccounts(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	db := filepath.Join(t.TempDir(), "shared.db")

	out, err := runCLI(t, f, fakeToken("uA"), "digest", "--all", "--agent", "--db", db)
	if err != nil || peopleIn(t, out) != 1 {
		t.Fatalf("account A should see its own card: err=%v out=%s", err, out)
	}
	out, err = runCLI(t, f, fakeToken("uB"), "digest", "--all", "--agent", "--no-refresh", "--db", db)
	if err != nil {
		t.Fatal(err)
	}
	if n := peopleIn(t, out); n != 0 {
		t.Fatalf("account B must not see account A's stored history, saw %d person(s)", n)
	}
	out, err = runCLI(t, f, fakeToken("uB"), "voices", "--agent", "--no-refresh", "--db", db)
	if err != nil || strings.Contains(out, "Alice") {
		t.Fatalf("voices leaked account A history to B: err=%v out=%s", err, out)
	}
	// Account A still sees its own history afterwards.
	out, err = runCLI(t, f, fakeToken("uA"), "digest", "--all", "--agent", "--no-refresh", "--db", db)
	if err != nil || peopleIn(t, out) != 1 {
		t.Fatalf("account A lost its history: err=%v out=%s", err, out)
	}
}

// Finding 2: an explicit --data-source local must not touch the network, and a
// live-only command must reject it.
func TestDataSourceLocalNeverContactsAPI(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	db := filepath.Join(t.TempDir(), "d.db")
	for _, args := range [][]string{
		{"digest", "--agent", "--data-source", "local", "--db", db},
		{"voices", "--agent", "--data-source", "local", "--db", db},
		{"rhythm", "--agent", "--data-source", "local", "--db", db},
	} {
		before := f.hits.Load()
		if _, err := runCLI(t, f, fakeToken("uA"), args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if got := f.hits.Load() - before; got != 0 {
			t.Fatalf("%v made %d API call(s) despite --data-source local", args, got)
		}
	}
}

func TestLiveOnlyCommandsRejectLocalDataSource(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	for _, c := range []string{"today", "owed", "queue", "resonance"} {
		before := f.hits.Load()
		_, err := runCLI(t, f, fakeToken("uA"), c, "--agent", "--data-source", "local")
		if err == nil || !strings.Contains(err.Error(), "no local data source") {
			t.Fatalf("%s should reject --data-source local with a clear error, got %v", c, err)
		}
		if f.hits.Load() != before {
			t.Fatalf("%s hit the API before rejecting", c)
		}
	}
}

// --data-source live must not silently fall back to stored data.
func TestDataSourceLiveFailsLoudlyWhenRefreshFails(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	db := filepath.Join(t.TempDir(), "d.db")
	if _, err := runCLI(t, f, fakeToken("uA"), "digest", "--all", "--agent", "--db", db); err != nil {
		t.Fatal(err)
	}
	f.srv.Close() // API now unreachable
	_, err := runCLI(t, f, fakeToken("uA"), "digest", "--all", "--agent", "--data-source", "live", "--timeout", "1s", "--db", db)
	if err == nil || !strings.Contains(err.Error(), "--data-source live") {
		t.Fatalf("--data-source live must fail naming the mode when the API refresh fails, got %v", err)
	}
}

// Databases created before rows carried an account id must be set aside, not
// served to whichever account opens them first.
func TestLegacyUnscopedHistoryIsSetAsideNotServed(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	st, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := `CREATE TABLE pn_reading_days (day TEXT PRIMARY KEY, seconds INTEGER NOT NULL, captured_at TEXT NOT NULL);
INSERT INTO pn_reading_days VALUES ('2026-09-28', 3600, 'x');
CREATE TABLE pn_snapshots (day TEXT PRIMARY KEY, captured_at TEXT NOT NULL, streak_current INTEGER, streak_max INTEGER, my_rank INTEGER, participants INTEGER);
CREATE TABLE pn_feed_cards (card_key TEXT PRIMARY KEY, starter_id TEXT, first_seen TEXT NOT NULL);`
	if _, err := st.DB().Exec(legacy); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()

	db, err := pnOpenStore(ctx, path, "u1")
	if err != nil {
		t.Fatalf("opening a legacy database must migrate it, got %v", err)
	}
	defer db.Close()
	var v rhythmView
	if err := rhythmAggregate(ctx, db.DB(), "uA", 2, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), &v); err != nil {
		t.Fatal(err)
	}
	if v.DaysRecorded != 0 {
		t.Fatalf("legacy unscoped rows must not be attributed to an account, got %d day(s)", v.DaysRecorded)
	}
	var kept int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM pn_reading_days_legacy_unscoped`).Scan(&kept); err != nil || kept != 1 {
		t.Fatalf("legacy rows must be preserved under *_legacy_unscoped, got %d err=%v", kept, err)
	}
	// Re-opening is idempotent.
	db2, err := pnOpenStore(ctx, path, "u1")
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	_ = db2.Close()
}

// Several CLI processes may open the same legacy database at once (a cron job
// and an interactive run, say). The check-then-rename migration must be atomic
// so none of them fails.
func TestConcurrentLegacyMigrationAllSucceed(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	for round := 0; round < 3; round++ {
		path := filepath.Join(t.TempDir(), "legacy.db")
		st, err := store.OpenWithContext(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.DB().Exec(`CREATE TABLE pn_reading_days (day TEXT PRIMARY KEY, seconds INTEGER NOT NULL, captured_at TEXT NOT NULL);
CREATE TABLE pn_snapshots (day TEXT PRIMARY KEY, captured_at TEXT NOT NULL, streak_current INTEGER, streak_max INTEGER, my_rank INTEGER, participants INTEGER);
CREATE TABLE pn_feed_cards (card_key TEXT PRIMARY KEY, starter_id TEXT, first_seen TEXT NOT NULL);`); err != nil {
			t.Fatal(err)
		}
		_ = st.Close()

		const workers = 6
		start := make(chan struct{})
		errs := make(chan error, workers)
		for i := 0; i < workers; i++ {
			go func() {
				<-start
				db, err := pnOpenStore(ctx, path, "u1")
				if err == nil {
					_ = db.Close()
				}
				errs <- err
			}()
		}
		close(start)
		for i := 0; i < workers; i++ {
			if err := <-errs; err != nil {
				t.Fatalf("round %d: concurrent open failed: %v", round, err)
			}
		}
	}
}

// A legacy-named leftover must not make the migration fail forever.
func TestMigrationSurvivesExistingLegacyTable(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "twice.db")
	st, err := store.OpenWithContext(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`CREATE TABLE pn_reading_days (day TEXT PRIMARY KEY, seconds INTEGER NOT NULL, captured_at TEXT NOT NULL);
CREATE TABLE pn_reading_days_legacy_unscoped (day TEXT PRIMARY KEY);`); err != nil {
		t.Fatal(err)
	}
	_ = st.Close()
	db, err := pnOpenStore(ctx, path, "uA")
	if err != nil {
		t.Fatalf("migration must pick a free legacy name, got %v", err)
	}
	defer db.Close()
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'pn_reading_days_legacy_unscoped_2'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("second legacy table should be renamed with a numeric suffix, found %d err=%v", n, err)
	}
}

func TestIsBusyDetection(t *testing.T) {
	for _, s := range []string{"database is locked (5) (SQLITE_BUSY)", "SQLITE_BUSY", "database table is locked"} {
		if !pnIsBusy(errors.New(s)) {
			t.Errorf("%q should count as busy", s)
		}
	}
	if pnIsBusy(nil) || pnIsBusy(errors.New("no such table")) {
		t.Error("non-busy errors must not be retried")
	}
}

// The default history file follows the account, not the bearer token: a
// refreshed token for the same account keeps its history, a different account
// gets its own file.
func TestDefaultHistoryFollowsAccountNotToken(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	run := func(token string, args ...string) string {
		out, err := runCLI(t, f, token, append([]string{"digest", "--all", "--agent"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out
	}
	first := fakeToken("uA")
	refreshed := first + "-refreshed" // same uid claim, different token
	if peopleIn(t, run(first)) != 1 {
		t.Fatal("account A should store and see its card")
	}
	if n := peopleIn(t, run(refreshed, "--no-refresh")); n != 1 {
		t.Fatalf("a refreshed token for the same account must keep its history, saw %d", n)
	}
	if n := peopleIn(t, run(fakeToken("uB"), "--no-refresh")); n != 0 {
		t.Fatalf("a different account must get its own empty history, saw %d", n)
	}
	if pnHistoryDBPath("uA") == pnHistoryDBPath("uB") {
		t.Fatal("history files for different accounts must differ")
	}
}

func TestLiveWithNoRefreshIsUsageError(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	db := filepath.Join(t.TempDir(), "d.db")
	for _, c := range []string{"digest", "voices", "rhythm"} {
		before := f.hits.Load()
		_, err := runCLI(t, f, fakeToken("uA"), c, "--agent", "--data-source", "live", "--no-refresh", "--db", db)
		if err == nil || !strings.Contains(err.Error(), "conflicts with --data-source live") {
			t.Fatalf("%s: want a usage error naming the conflict, got %v", c, err)
		}
		if f.hits.Load() != before {
			t.Fatalf("%s contacted the API before rejecting the flag combination", c)
		}
	}
}

func rankTrendLen(t *testing.T, out string) int {
	t.Helper()
	var env struct {
		Results struct {
			RankTrend []any `json:"rank_trend"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return len(env.Results.RankTrend)
}

// A failed standings call must never surface as "rank 0 of 0"; in strict mode
// it must fail the refresh instead of recording a partial snapshot.
func TestRhythmStandingsFailure(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	f.failStandings.Store(true)
	db := filepath.Join(t.TempDir(), "r.db")

	out, err := runCLI(t, f, fakeToken("uA"), "rhythm", "--agent", "--db", db)
	if err != nil {
		t.Fatalf("auto mode tolerates a standings failure: %v", err)
	}
	if n := rankTrendLen(t, out); n != 0 {
		t.Fatalf("a snapshot without a rank must not appear in rank_trend, saw %d entries", n)
	}
	_, err = runCLI(t, f, fakeToken("uA"), "rhythm", "--agent", "--data-source", "live", "--db", db)
	if err == nil || !strings.Contains(err.Error(), "friends standings") {
		t.Fatalf("strict mode must fail naming the standings call, got %v", err)
	}

	f.failStandings.Store(false)
	out, err = runCLI(t, f, fakeToken("uA"), "rhythm", "--agent", "--db", db)
	if err != nil || rankTrendLen(t, out) != 1 {
		t.Fatalf("a good rank must appear once standings succeed: err=%v out=%s", err, out)
	}
}

// The account's own time zone is remembered so later local-only runs bucket
// weeks the same way the refresh did.
func TestRhythmRemembersAccountTimezone(t *testing.T) {
	testenv.Isolate(t)
	f := newFakePhilonet(t)
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "tz.db")
	if _, err := runCLI(t, f, fakeToken("uA"), "rhythm", "--agent", "--db", db); err != nil {
		t.Fatal(err)
	}
	st, err := pnOpenStore(ctx, db, "uA")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if loc := pnStoredLocation(ctx, st.DB(), "uA"); loc.String() != "Pacific/Auckland" {
		t.Fatalf("stored time zone = %q, want Pacific/Auckland", loc)
	}
	if loc := pnStoredLocation(ctx, st.DB(), "someone-else"); loc != time.Local {
		t.Fatalf("an account with no recorded zone must fall back to local, got %q", loc)
	}
}

// If another process holds the write lock and the caller gives up, the error
// must be the cancellation, not a misleading busy message.
func TestPrepareHistoryReportsCancellationWhileBusy(t *testing.T) {
	testenv.Isolate(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "busy.db")
	st, err := pnOpenStore(ctx, path, "uA")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	holder, err := st.DB().BeginTx(ctx, nil) // takes the immediate write lock
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback() }()

	short, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()
	err = pnPrepareHistory(short, st.DB())
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want the context deadline surfaced, got %v", err)
	}
}
