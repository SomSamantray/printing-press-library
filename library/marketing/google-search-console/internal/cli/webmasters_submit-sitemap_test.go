// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written regression test for the submit-sitemap feedpath URL-encode fix.
// A feedpath like "https://example.com/sitemap.xml" must be percent-encoded as
// a single path segment before it reaches the wire, or Google returns 404.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebmastersSubmitSitemapEncodesFeedpath(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	t.Setenv("GSC_ACCESS_TOKEN", "test-token")
	t.Setenv("GOOGLE_SEARCH_CONSOLE_BASE_URL", srv.URL)

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--config", filepath.Join(t.TempDir(), "missing-config.toml"),
		"--json",
		"webmasters",
		"submit-sitemap",
		"sc-domain:usenoreply.com",
		"https://usenoreply.com/sitemap.xml",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("submit-sitemap returned error: %v\noutput:\n%s", err, out.String())
	}

	// The routing defect is the scheme's slashes: "https://" inserted a raw "/"
	// that split the feedpath into extra path segments, 404ing the request.
	// Go keeps ":" raw (a legal path sub-delimiter) but percent-encodes the
	// slashes, so the whole feedpath travels as a single segment. Assert the
	// slashes are encoded and no raw scheme reaches the wire.
	if !strings.Contains(gotRequestURI, "%2F%2Fusenoreply.com%2Fsitemap.xml") {
		t.Fatalf("feedpath slashes were not URL-encoded on the wire: got request URI %q, want it to contain %q", gotRequestURI, "%2F%2Fusenoreply.com%2Fsitemap.xml")
	}
	if strings.Contains(gotRequestURI, "sitemaps/https://") {
		t.Fatalf("feedpath scheme reached the wire raw: %q", gotRequestURI)
	}
	if !strings.Contains(gotRequestURI, "sc-domain:usenoreply.com") {
		t.Fatalf("siteUrl should be left unencoded but was altered: got request URI %q", gotRequestURI)
	}
}

func TestWebmastersSubmitSitemapLeavesPlainFeedpathUnchanged(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	t.Setenv("GSC_ACCESS_TOKEN", "test-token")
	t.Setenv("GOOGLE_SEARCH_CONSOLE_BASE_URL", srv.URL)

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--config", filepath.Join(t.TempDir(), "missing-config.toml"),
		"--json",
		"webmasters",
		"submit-sitemap",
		"sc-domain:usenoreply.com",
		"sitemap.xml",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("submit-sitemap returned error: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(gotRequestURI, "/sitemaps/sitemap.xml") {
		t.Fatalf("plain feedpath was unexpectedly altered: got request URI %q", gotRequestURI)
	}
}