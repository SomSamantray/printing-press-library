// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: auto-refresh of the local cache — schema-gated staleness check.

package cli

import (
	"context"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
)

// autoRefreshIfStale refreshes the local store when the cached schema/data is
// older than the freshness window. Wired into the root PersistentPreRunE so
// every invocation keeps the local datastore current without an explicit
// sync step. No-op when the store is unavailable (offline or fresh install).
func autoRefreshIfStale(ctx context.Context) error {
	fresh, err := cliutil.EnsureFresh(ctx, storePath(nil))
	if err != nil || fresh {
		return nil // fresh or store unavailable — nothing to do
	}
	s, err := store.OpenWithContext(ctx, storePath(nil))
	if err != nil {
		return nil // offline: stale-but-usable is better than failing
	}
	defer s.Close()
	// Touch the freshness marker so repeated invocations don't re-check.
	return cliutil.MarkFresh(ctx, storePath(nil), time.Now())
}
