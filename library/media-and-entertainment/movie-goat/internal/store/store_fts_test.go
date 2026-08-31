// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written tests for the FTS5 query-escaping fix: a user query containing
// FTS5 syntax (e.g. "Space: 1999", a bare AND/OR/NOT, quotes, wildcards) must
// search literally and never raise a MATCH parse error, while ordinary
// multi-word queries keep implicit-AND semantics.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// seedFTSStore opens a temp-dir file-backed store with known titles.
func seedFTSStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "fts_test.db"))
	if err != nil {
		t.Fatalf("opening temp store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	fixtures := []struct {
		id   string
		data string
	}{
		{"1", `{"id":1,"title":"Space: 1999","type":"movie"}`},
		{"2", `{"id":2,"title":"Inception","type":"movie"}`},
		{"3", `{"id":3,"title":"Inception: Music from the Motion Picture","type":"movie"}`},
		{"4", `{"id":4,"title":"A \"quoted\" title","type":"movie"}`},
		{"5", `{"id":5,"title":"the movie inception is a film","type":"movie"}`},
	}
	for _, f := range fixtures {
		if err := s.Upsert("movies", f.id, json.RawMessage(f.data)); err != nil {
			t.Fatalf("upserting fixture %s: %v", f.id, err)
		}
	}
	return s
}

// TestSearchFTSQueryEscapingNeverErrors asserts that queries containing FTS5
// syntax characters or bare keywords return rows-or-empty, never a MATCH parse
// error.
func TestSearchFTSQueryEscapingNeverErrors(t *testing.T) {
	s := seedFTSStore(t)
	for _, query := range []string{
		"Space: 1999",
		`A "quoted" title`,
		`"`,
		"*",
		"AND",
		"OR",
		"NOT",
		"inception OR batman",
		"NEAR/",
		"^",
		"{a b}",
		"  ",
	} {
		got, err := s.Search(query, 50)
		if err != nil {
			t.Fatalf("Search(%q) errored: %v", query, err)
		}
		if got == nil {
			t.Fatalf("Search(%q) returned nil slice", query)
		}
	}
}

// TestSearchFTSQueryLiteralMatch asserts FTS-syntax queries match the literal
// text they denote.
func TestSearchFTSQueryLiteralMatch(t *testing.T) {
	s := seedFTSStore(t)

	got, err := s.Search("Space: 1999", 50)
	if err != nil {
		t.Fatalf("Search(Space: 1999) errored: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Search(Space: 1999) = %d rows, want 1", len(got))
	}

	got, err = s.Search(`A "quoted" title`, 50)
	if err != nil {
		t.Fatalf("Search(quoted title) errored: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Search(A \"quoted\" title) = %d rows, want 1", len(got))
	}
}

// TestSearchFTSQueryMultiWordAND asserts ordinary multi-word queries keep
// implicit-AND semantics: tokens may match anywhere in a row, not only as an
// adjacent phrase.
func TestSearchFTSQueryMultiWordAND(t *testing.T) {
	s := seedFTSStore(t)

	got, err := s.Search("inception movie", 50)
	if err != nil {
		t.Fatalf("Search(inception movie) errored: %v", err)
	}
	// Row 3 ("Inception: Music from the Motion Picture") and row 5 ("the movie
	// inception is a film") both contain both tokens non-adjacently.
	if len(got) < 2 {
		t.Fatalf("Search(inception movie) = %d rows, want >= 2 (implicit-AND preserved): %v", len(got), got)
	}
}

// TestSearchFTSEmptyQuery asserts an empty/whitespace query returns an empty
// result rather than constructing MATCH '""'.
func TestSearchFTSEmptyQuery(t *testing.T) {
	s := seedFTSStore(t)
	for _, query := range []string{"", "   "} {
		got, err := s.Search(query, 50)
		if err != nil {
			t.Fatalf("Search(%q) errored: %v", query, err)
		}
		if len(got) != 0 {
			t.Fatalf("Search(%q) = %d rows, want 0", query, len(got))
		}
	}
}
