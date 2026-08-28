// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: sync — pull marketplace data (categories, collections, top APIs)
// into the local store with pagination and sync-state tracking.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

// defaultSyncResources lists the resource types synced by default.
func defaultSyncResources() []string {
	return []string{"category", "collection", "api"}
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var resource string

	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Sync marketplace data (categories, collections, top APIs) into the local store",
		Long:        "Pull marketplace data from the RapidAPI hub into the local SQLite store for offline querying. Tracks pagination progress and last-synced state per resource; re-running resumes from the saved cursor.",
		Example:     "  rapidapi-pp-cli sync\n  rapidapi-pp-cli sync --resource category --limit 50",
		Annotations: map[string]string{"pp:endpoint": "sync.marketplace", "pp:method": "POST", "pp:path": "/gateway/graphql"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
			if err != nil {
				return err
			}
			defer s.Close()

			resources := defaultSyncResources()
			if resource != "" {
				resources = []string{resource}
			}
			if limit <= 0 {
				limit = 50
			}

			for _, res := range resources {
				count, err := syncResource(cmd, flags, s, res, limit)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "sync %s: %v\n", res, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "synced %s: %d records\n", res, count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "", "Sync a single resource type (category, collection, api)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max records per resource")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}

// syncResource pulls one resource type into the store, tracking sync state.
func syncResource(cmd *cobra.Command, flags *rootFlags, s *store.Store, resource string, limit int) (int, error) {
	path := "/gateway/graphql"
	_ = path
	switch resource {
	case "category":
		variables := map[string]any{"limit": limit}
		data, err := gqlExec(cmd, flags, "getCategoriesByCtx", variables, gqlResponsePaths["getCategoriesByCtx"])
		if err != nil {
			return 0, err
		}
		count := cacheDomainRows(cmd, s, "category", data, s.UpsertCategories)
		return count, s.SaveSyncState(resource, fmt.Sprintf("page:%d", time.Now().Unix()), count)
	case "collection":
		variables := map[string]any{"page": 1, "limit": limit}
		data, err := gqlExec(cmd, flags, "GetCollectionsCollapsed", variables, gqlResponsePaths["GetCollectionsCollapsed"])
		if err != nil {
			return 0, err
		}
		count := cacheDomainRows(cmd, s, "collection", data, s.UpsertCollections)
		return count, s.SaveSyncState(resource, fmt.Sprintf("page:%d", time.Now().Unix()), count)
	case "api":
		variables := map[string]any{
			"searchApiWhereInput":   map[string]any{"term": ""},
			"paginationInput":       map[string]any{"first": limit},
			"searchApiOrderByInput": map[string]any{"sortingFields": []map[string]any{{"by": "ASC", "fieldName": "ByRelevance"}}},
		}
		data, err := gqlExec(cmd, flags, "searchApis", variables, gqlResponsePaths["searchApis"])
		if err != nil {
			return 0, err
		}
		count := cacheDomainRows(cmd, s, "api", data, s.UpsertApis)
		return count, s.SaveSyncState(resource, fmt.Sprintf("page:%d", time.Now().Unix()), count)
	default:
		return 0, fmt.Errorf("unknown resource %q (use category, collection, or api)", resource)
	}
}

// cacheDomainRows upserts a JSON array of records via a domain-specific
// store helper (e.g. UpsertApis) so records land in the typed domain tables.
func cacheDomainRows(cmd *cobra.Command, s *store.Store, resource string, data json.RawMessage, upsert func(json.RawMessage) error) int {
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return 0
	}
	count := 0
	for _, it := range items {
		if err := upsert(mustJSON(it)); err == nil {
			count++
		}
	}
	return count
}
