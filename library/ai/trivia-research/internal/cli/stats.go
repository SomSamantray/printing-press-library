package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "stats",
		Short:   "Show episode counts across all sources",
		Example: "  trivia-research-pp-cli stats\n  trivia-research-pp-cli stats --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			sources := getActiveSources("all")
			stats := make(map[string]any)
			totalEpisodes := 0

			for _, src := range sources {
				episodes, _ := src.Sync(1)
				stats[src.Source()] = map[string]any{
					"name":     src.Name(),
					"episodes": len(episodes),
				}
				totalEpisodes += len(episodes)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"total_episodes": totalEpisodes,
					"source_count":   len(sources),
					"sources":        stats,
				}, flags)
			}

			w := tableWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "SOURCE\tEPISODES")
			for _, src := range types.AllSources {
				if info, ok := stats[src].(map[string]any); ok {
					fmt.Fprintf(w, "%s\t%d\n", info["name"], info["episodes"])
				}
			}
			w.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d episodes across %d sources\n", totalEpisodes, len(sources))
			return nil
		},
	}
}

func _sc() { _ = newStatsCmd }
