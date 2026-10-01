// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written helpers shared by the Philonet novel commands (today, rhythm,
// digest, owed, voices, queue, resonance). Kept in its own file so regeneration
// preserves it.

package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/philonet/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/philonet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/philonet/internal/config"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/philonet/internal/store"
)

// ---- tolerant JSON accessors -------------------------------------------------

// pnMap decodes a JSON object body. Anything else (HTML error page, array,
// empty body) is an error so it is never mistaken for a successful empty result.
func pnMap(raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unexpected response (not a JSON object): %w", err)
	}
	if m == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}
	if e, ok := m["error"].(string); ok && e != "" && m["success"] != true {
		return nil, fmt.Errorf("API error: %s", e)
	}
	return m, nil
}

// pnDig walks nested maps by key. A missing or non-map step yields nil.
func pnDig(v any, path ...string) any {
	cur := v
	for _, k := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[k]
	}
	return cur
}

func pnStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func pnInt(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	}
	return 0
}

func pnBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func pnList(v any) []any {
	l, _ := v.([]any)
	return l
}

// pnFirstOf returns the first non-nil value.
func pnFirstOf(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// pnFirst returns the first present, non-nil value among keys.
func pnFirst(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func pnTrunc(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ---- transport ---------------------------------------------------------------

func pnGet(ctx context.Context, c *client.Client, path string, params map[string]string) (map[string]any, error) {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	return pnMap(raw)
}

func pnPost(ctx context.Context, c *client.Client, path string, body any) (map[string]any, error) {
	raw, _, err := c.Post(ctx, path, body)
	if err != nil {
		return nil, err
	}
	return pnMap(raw)
}

// pnUserID reads the `uid` claim from the configured bearer token. The token is
// only decoded, never logged or stored.
func pnUserID(flags *rootFlags) (string, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return "", configErr(err)
	}
	return pnUIDFromAuthHeader(cfg.AuthHeader())
}

func pnUIDFromAuthHeader(h string) (string, error) {
	tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return "", configErr(fmt.Errorf("no Philonet token configured; set PHILONET_TOKEN (see 'philonet-pp-cli doctor')"))
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return "", configErr(fmt.Errorf("token is not a JWT; copy accessToken from philonet.ai local storage"))
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", configErr(fmt.Errorf("token payload unreadable"))
	}
	uid := pnStr(claims["uid"])
	if uid == "" {
		return "", configErr(fmt.Errorf("token has no uid claim"))
	}
	return uid, nil
}

// ---- local store for history -------------------------------------------------

// Every history row carries the owning account's uid, so two accounts sharing
// one database file (for example through the generator's legacy unscoped
// data.db fallback) can never read each other's history.
const pnSchema = `
CREATE TABLE IF NOT EXISTS pn_feed_cards (
	uid TEXT NOT NULL,
	card_key TEXT NOT NULL,
	article_id TEXT, article_title TEXT, article_url TEXT, category TEXT, tags TEXT,
	reading_minutes INTEGER,
	thought_id TEXT, thought_text TEXT, insightful INTEGER, thought_created_at TEXT,
	starter_id TEXT, starter_name TEXT, starter_is_friend INTEGER,
	alma_mater TEXT, employer TEXT,
	source_feed TEXT, first_seen TEXT NOT NULL,
	PRIMARY KEY (uid, card_key)
);
CREATE INDEX IF NOT EXISTS pn_feed_cards_seen ON pn_feed_cards(uid, first_seen);
CREATE TABLE IF NOT EXISTS pn_reading_days (
	uid TEXT NOT NULL,
	day TEXT NOT NULL, seconds INTEGER NOT NULL, captured_at TEXT NOT NULL,
	PRIMARY KEY (uid, day)
);
CREATE TABLE IF NOT EXISTS pn_account_meta (
	uid TEXT NOT NULL PRIMARY KEY,
	timezone TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS pn_snapshots (
	uid TEXT NOT NULL,
	day TEXT NOT NULL, captured_at TEXT NOT NULL,
	streak_current INTEGER, streak_max INTEGER, my_rank INTEGER, participants INTEGER,
	PRIMARY KEY (uid, day)
);`

// pnHistoryDBPath is the default history file for one account. It is keyed by
// the account id, not by the bearer token: tokens are short-lived and get
// re-copied from the browser, and the generator's default path would point a
// refreshed token at a brand-new empty database and orphan its history.
func pnHistoryDBPath(uid string) string {
	sum := sha256.Sum256([]byte(uid))
	name := "history-" + hex.EncodeToString(sum[:])[:12] + ".db"
	dir, err := cliutil.DataDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return name
		}
		dir = filepath.Join(home, ".local", "share", "philonet-pp-cli")
	}
	return filepath.Join(dir, name)
}

func pnOpenStore(ctx context.Context, dbPath, uid string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = pnHistoryDBPath(uid)
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := pnPrepareHistory(ctx, db.DB()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// pnPrepareHistory migrates legacy tables and creates the scoped schema in ONE
// transaction. The store opens SQLite with _txlock=immediate, so BeginTx takes
// the write lock up front; a second process opening the same database waits
// (busy_timeout) and then sees the finished migration instead of racing the
// check-then-rename and failing.
func pnPrepareHistory(ctx context.Context, db *sql.DB) error {
	var err error
	for attempt := 1; attempt <= pnBusyAttempts; attempt++ {
		if err = pnPrepareHistoryOnce(ctx, db); err == nil || !pnIsBusy(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), err)
		case <-time.After(time.Duration(attempt) * 75 * time.Millisecond):
		}
	}
	return err
}

// pnBusyAttempts bounds retries when another process holds the write lock for
// longer than SQLite's own busy timeout (a slow disk, a long sync).
const pnBusyAttempts = 12

func pnIsBusy(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "database is locked") || strings.Contains(m, "sqlite_busy") || strings.Contains(m, "database table is locked")
}

func pnPrepareHistoryOnce(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("preparing history tables: %w", err)
	}
	if err := pnMigrateUnscoped(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migrating history tables: %w", err)
	}
	if _, err := tx.ExecContext(ctx, pnSchema); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("preparing history tables: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("preparing history tables: %w", err)
	}
	return nil
}

// pnQuerier is the part of *sql.DB and *sql.Tx the migration needs.
type pnQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// pnMigrateUnscoped sets aside history tables created before rows carried an
// account id. Those rows cannot be attributed to an account, so they are kept
// under a *_legacy_unscoped name (never read) rather than shown to anyone.
func pnMigrateUnscoped(ctx context.Context, db pnQuerier) error {
	// Table names below are constants plus an integer suffix; nothing is built
	// from input.
	for _, table := range []string{"pn_feed_cards", "pn_reading_days", "pn_snapshots"} {
		exists, hasUID, err := pnTableShape(ctx, db, table)
		if err != nil {
			return err
		}
		if !exists || hasUID {
			continue
		}
		// Pick a legacy name that is free, so a second unscoped table (for
		// example re-created by an older build) never blocks the migration.
		target := table + "_legacy_unscoped"
		for n := 2; ; n++ {
			taken, _, err := pnTableShape(ctx, db, target)
			if err != nil {
				return err
			}
			if !taken {
				break
			}
			target = table + "_legacy_unscoped_" + strconv.Itoa(n)
		}
		if table == "pn_feed_cards" {
			if _, err := db.ExecContext(ctx, `DROP INDEX IF EXISTS pn_feed_cards_seen`); err != nil {
				return err
			}
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE `+table+` RENAME TO `+target); err != nil {
			return err
		}
	}
	return nil
}

// pnTableShape reports whether table exists and whether it has a uid column.
func pnTableShape(ctx context.Context, db pnQuerier, table string) (exists, hasUID bool, err error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return false, false, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return false, false, err
		}
		exists = true
		if name == "uid" {
			hasUID = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return false, false, err
	}
	return exists, hasUID, rows.Close()
}

// pnRefreshMode turns the global --data-source flag and the command's own
// --no-refresh into one decision for store-backed commands:
//
//	auto  -> refresh from the API unless --no-refresh; refresh failures are warnings
//	local -> never contact the API
//	live  -> always refresh and fail rather than fall back to stored data
func pnRefreshMode(flags *rootFlags, noRefresh bool) (refresh, strict bool, err error) {
	switch flags.dataSource {
	case "local":
		return false, false, nil
	case "live":
		if noRefresh {
			return false, false, usageErr(fmt.Errorf("--no-refresh conflicts with --data-source live"))
		}
		return true, true, nil
	default:
		return !noRefresh, false, nil
	}
}

// pnRequireLive rejects --data-source local on commands that have no stored
// data to serve it from.
func pnRequireLive(flags *rootFlags) error {
	if err := validateDataSourceStrategy(flags, "live"); err != nil {
		return usageErr(err)
	}
	return nil
}

// ---- feed cards --------------------------------------------------------------

type pnCard struct {
	Key            string
	ArticleID      string
	ArticleTitle   string
	ArticleURL     string
	Category       string
	Tags           []string
	ReadingMinutes int64
	ThoughtID      string
	ThoughtText    string
	Insightful     int64
	ThoughtAt      string
	StarterID      string
	StarterName    string
	StarterFriend  bool
	AlmaMater      string
	Employer       string
}

// pnCardsFromItem flattens one feed item into one card per conversation starter.
func pnCardsFromItem(item map[string]any) []pnCard {
	art, _ := item["article"].(map[string]any)
	if art == nil {
		return nil
	}
	var tags []string
	for _, t := range pnList(art["tags"]) {
		if s := pnStr(t); s != "" {
			tags = append(tags, s)
		}
	}
	base := pnCard{
		ArticleID:      pnStr(art["id"]),
		ArticleTitle:   pnStr(pnFirst(art, "title", "headline")),
		ArticleURL:     pnStr(art["url"]),
		Category:       pnStr(art["category"]),
		Tags:           tags,
		ReadingMinutes: pnInt(pnDig(art, "reading_time", "minutes")),
	}
	var out []pnCard
	for _, raw := range pnList(item["conversation_starters"]) {
		s, _ := raw.(map[string]any)
		if s == nil {
			continue
		}
		st, _ := s["starter"].(map[string]any)
		c := base
		c.ThoughtID = pnStr(s["id"])
		c.ThoughtText = pnStr(pnFirst(s, "content", "quote_headline", "quote"))
		c.Insightful = pnInt(s["insightful_count"]) + pnInt(s["star_count"]) + pnInt(s["resonate_count"])
		c.ThoughtAt = pnStr(s["created_at"])
		if st != nil {
			c.StarterID = pnStr(st["user_id"])
			c.StarterName = pnStr(st["name"])
			c.StarterFriend = pnBool(st["is_friend"])
			c.AlmaMater = pnStr(pnDig(st, "verifications", "alma_mater", "entity_name"))
			c.Employer = pnStr(pnDig(st, "verifications", "professional", "entity_name"))
		}
		if c.ThoughtID == "" {
			continue // without a thought id the card cannot be told apart from its siblings
		}
		c.Key = c.ArticleID + ":" + c.ThoughtID
		out = append(out, c)
	}
	return out
}

func pnStoreCards(ctx context.Context, db *sql.DB, uid string, cards []pnCard, feed string, now time.Time) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO pn_feed_cards
		(uid,card_key,article_id,article_title,article_url,category,tags,reading_minutes,thought_id,thought_text,insightful,thought_created_at,starter_id,starter_name,starter_is_friend,alma_mater,employer,source_feed,first_seen)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uid, card_key) DO UPDATE SET insightful=excluded.insightful,
		starter_is_friend=MAX(pn_feed_cards.starter_is_friend, excluded.starter_is_friend),
		article_title=excluded.article_title, thought_text=excluded.thought_text, starter_name=excluded.starter_name,
		alma_mater=excluded.alma_mater, employer=excluded.employer`)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()
	n := 0
	for _, c := range cards {
		fr := 0
		if c.StarterFriend {
			fr = 1
		}
		if _, err := stmt.ExecContext(ctx, uid, c.Key, c.ArticleID, c.ArticleTitle, c.ArticleURL, c.Category, strings.Join(c.Tags, "|"), c.ReadingMinutes,
			c.ThoughtID, c.ThoughtText, c.Insightful, c.ThoughtAt, c.StarterID, c.StarterName, fr, c.AlmaMater, c.Employer, feed, now.UTC().Format(time.RFC3339)); err != nil {
			_ = tx.Rollback()
			return n, err
		}
		n++
	}
	return n, tx.Commit()
}

// pnRefreshFeeds pulls up to maxPages pages of each feed and stores the cards.
// It returns cards stored and per-page failures (kept, never silently dropped).
// In strict mode it stops at the first failure instead of walking the remaining
// feeds (each failing page already costs the client's full retry budget).
func pnRefreshFeeds(ctx context.Context, c *client.Client, db *sql.DB, uid string, feeds []string, maxPages int, strict bool) (int, []string) {
	total := 0
	var failures []string
	for _, feed := range feeds {
		session := ""
		for page := 1; page <= maxPages; page++ {
			params := map[string]string{"filter": feed, "page": strconv.Itoa(page), "pageSize": "10"}
			if session != "" {
				params["sessionId"] = session
			}
			m, err := pnGet(ctx, c, "/v1/feed2/forme", params)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s page %d: %v", feed, page, err))
				if strict {
					return total, failures
				}
				break
			}
			if s := pnStr(pnDig(m, "meta", "session_id")); s != "" {
				session = s
			}
			var cards []pnCard
			for _, it := range pnList(m["items"]) {
				if im, ok := it.(map[string]any); ok {
					cards = append(cards, pnCardsFromItem(im)...)
				}
			}
			n, err := pnStoreCards(ctx, db, uid, cards, feed, time.Now())
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s page %d store: %v", feed, page, err))
				if strict {
					return total, failures
				}
				break
			}
			total += n
			if !pnBool(m["has_more"]) || len(pnList(m["items"])) == 0 {
				break
			}
		}
	}
	return total, failures
}

// ---- small shared views ------------------------------------------------------

type pnFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

func pnWeekStart(t time.Time) time.Time {
	d := int(t.Weekday()+6) % 7 // Monday = 0
	y, m, day := t.Date()
	return time.Date(y, m, day-d, 0, 0, 0, 0, t.Location())
}

func pnSortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pnStoredLocation returns the account time zone recorded by the last refresh,
// or time.Local when none is recorded or it cannot be loaded.
func pnStoredLocation(ctx context.Context, db *sql.DB, uid string) *time.Location {
	var tz string
	if err := db.QueryRowContext(ctx, `SELECT timezone FROM pn_account_meta WHERE uid = ?`, uid).Scan(&tz); err != nil || tz == "" {
		return time.Local
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.Local
}
