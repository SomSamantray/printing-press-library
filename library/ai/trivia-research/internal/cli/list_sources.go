package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
	"github.com/spf13/cobra"
)

func newListSourcesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list-sources",
		Short:   "List available podcast and blog sources",
		Example: "  trivia-research-pp-cli list-sources\n  trivia-research-pp-cli list-sources --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON {
				type sourceInfo struct {
					Key   string `json:"key"`
					Name  string `json:"name"`
					Emoji string `json:"emoji"`
					Type  string `json:"type"`
				}
				var sources []sourceInfo
				for i, key := range types.AllSources {
					t := "podcast"
					if i == 8 {
						t = "blog"
					}
					sources = append(sources, sourceInfo{
						Key: key, Name: types.SourceNames[key],
						Emoji: types.SourceEmoji[key], Type: t,
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), sources, flags)
			}

			w := tableWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "EMOJI\tSOURCE\tNAME")
			for _, key := range types.AllSources {
				fmt.Fprintf(w, "%s\t%s\t%s\n", types.SourceEmoji[key], key, types.SourceNames[key])
			}
			return w.Flush()
		},
	}
}

func _lsc() { _ = newListSourcesCmd }
