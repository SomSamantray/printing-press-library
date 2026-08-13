// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// init wires `search check` as a subcommand of the framework `search` command.
// The framework search owns the `search <query>` surface; `check` adds the
// assertion contract without touching the generated search.go.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		searchCmd, _, err := root.Find([]string{"search"})
		if err == nil {
			addNovelCommandIfAbsent(searchCmd, newNovelSearchCheckCmd(flags))
		}
	})
}

type searchCheckResult struct {
	Index    string   `json:"index"`
	Query    string   `json:"query"`
	Expected []string `json:"expected"`
	Found    []string `json:"found"`
	Missing  []string `json:"missing"`
	Passed   bool     `json:"passed"`
	Note     string   `json:"note,omitempty"`
}

func newNovelSearchCheckCmd(flags *rootFlags) *cobra.Command {
	var flagIndex string
	var flagQuery string
	var flagExpect string

	cmd := &cobra.Command{
		Use:         "check",
		Short:       "Assert that a query returns expected objectIDs, with a typed exit code for CI pipelines.",
		Example:     "  algolia-pp-cli search check --index algolia_movie_sample_dataset --query dune --expect media-sample-data-438631",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search check")
			}
			if flagIndex == "" || flagQuery == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--index and --query are required"))
			}
			if flagExpect == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--expect is required (comma-separated objectIDs)"))
			}
			expected := splitCSV(flagExpect)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Use the Browse API (cursor-based) instead of search pagination:
			// Algolia caps a single search query at 1,000 hits regardless of
			// page size, so expected IDs ranking beyond the top 1,000 are
			// unreachable by paging. Browse iterates every matching record.
			path := "/1/indexes/" + flagIndex + "/browse"
			expectedSet := make(map[string]bool, len(expected))
			for _, e := range expected {
				expectedSet[e] = true
			}
			seen := make(map[string]bool, len(expected))
			found := make([]string, 0, len(expected))
			missing := make([]string, 0)
			scanCapped := false
			const maxBatches = 500 // bounded: 500k records maximum scanned
			cursor := ""
			for batch := 0; batch < maxBatches && len(seen) < len(expectedSet); batch++ {
				body := map[string]any{
					"query":                flagQuery,
					"attributesToRetrieve": []string{"objectID"},
				}
				if cursor != "" {
					body["cursor"] = cursor
				}
				data, _, getErr := c.Post(ctx, path, body)
				if getErr != nil {
					return classifyAPIError(getErr, flags)
				}
				var envelope struct {
					Hits   []map[string]any `json:"hits"`
					Cursor string           `json:"cursor"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					return fmt.Errorf("parsing browse response: %w", err)
				}
				for _, h := range envelope.Hits {
					oid, _ := h["objectID"].(string)
					if oid == "" || !expectedSet[oid] || seen[oid] {
						continue
					}
					seen[oid] = true
					found = append(found, oid)
				}
				if envelope.Cursor == "" || len(envelope.Hits) == 0 {
					break
				}
				cursor = envelope.Cursor
			}
			if len(seen) < len(expectedSet) {
				scanCapped = true
			}
			for _, e := range expected {
				if !seen[e] {
					missing = append(missing, e)
				}
			}
			res := searchCheckResult{
				Index:    flagIndex,
				Query:    flagQuery,
				Expected: expected,
				Found:    found,
				Missing:  missing,
				Passed:   len(missing) == 0,
			}
			if scanCapped {
				res.Note = fmt.Sprintf("scan cap reached at %d browse batches; expected IDs may rank deeper than 500k records — widen the query or raise the batch bound", maxBatches)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), res, flags); err != nil {
					return err
				}
			} else if res.Passed {
				fmt.Fprintf(cmd.OutOrStdout(), "PASS: query %q on %s returned all %d expected objectIDs\n", flagQuery, flagIndex, len(expected))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "FAIL: query %q on %s missing %d expected objectIDs: %s\n", flagQuery, flagIndex, len(missing), strings.Join(missing, ", "))
			}
			if !res.Passed {
				// Typed exit: 1 for assertion failure (not a usage/API error).
				return newAssertionError()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagIndex, "index", "", "Index name to search")
	cmd.Flags().StringVar(&flagQuery, "query", "", "Search query to run")
	cmd.Flags().StringVar(&flagExpect, "expect", "", "Comma-separated objectIDs that must appear in the results")
	return cmd
}

// assertionError signals a failed assertion with a distinct typed exit code (1).
type assertionError struct{}

func (assertionError) Error() string { return "assertion failed" }

func newAssertionError() error { return assertionError{} }

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
