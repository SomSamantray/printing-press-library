// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/spf13/cobra"
)

func newNotebookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "notebook",
		Aliases: []string{"notebooks"},
		Short:   "Manage notebooks",
	}
	cmd.AddCommand(newNotebookListCmd(flags))
	return cmd
}

func newNotebookListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recently viewed notebooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			if cfg.AuthHeader == "" {
				return fmt.Errorf("not authenticated: run notebooklm-pp-cli auth login --chrome")
			}
			httpClient, err := cfg.HTTPClient()
			if err != nil {
				return err
			}
			client, err := nlm.NewClient(context.Background(), httpClient)
			if err != nil {
				return err
			}
			notebooks, err := client.ListNotebooks(context.Background())
			if err != nil {
				return err
			}
			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(notebooks)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tSOURCES")
			for _, nb := range notebooks {
				title := nb.Title
				if nb.Emoji != "" {
					title = title + " " + nb.Emoji
				}
				fmt.Fprintf(w, "%s\t%s\t%d\n", nb.ID, title, nb.SourceCount)
			}
			return w.Flush()
		},
	}
}
