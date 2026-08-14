// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestDocaiBatchFailureReason locks down that rejected jobs and jobs that
// never reach a terminal status after polling are treated as failures, not
// silent successes with no saved result.
func TestDocaiBatchFailureReason(t *testing.T) {
	cases := []struct {
		name     string
		status   string
		terminal bool
		wantErr  bool
	}{
		{"completed", "completed", true, false},
		{"partially_completed", "partially_completed", true, false},
		{"failed status", "failed", true, true},
		{"rejected status", "rejected", true, true},
		{"poll exhausted", "processing", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := docaiBatchFailureReason(tc.status, tc.terminal)
			if (got != "") != tc.wantErr {
				t.Fatalf("docaiBatchFailureReason(%q, terminal=%v) = %q, want non-empty=%v", tc.status, tc.terminal, got, tc.wantErr)
			}
		})
	}
}

// TestNovelDocaiBatchHelpWires smoke-tests that the docai batch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelDocaiBatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"docai", "batch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("docai batch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "batch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("docai batch --help missing %q in output:\n%s", want, help)
		}
	}
}
