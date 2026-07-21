// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

// Execute runs the CLI.
func Execute() error {
	var flags rootFlags
	rootCmd := newRootCmd(&flags)
	return rootCmd.Execute()
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "notebooklm-pp-cli",
		Short:        "CLI for Gemini Notebook (NotebookLM) via unofficial batchexecute RPC",
		SilenceUsage: true,
		Version:      version,
	}
	rootCmd.SetVersionTemplate("notebooklm-pp-cli {{ .Version }}\n")
	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Emit machine-readable JSON")
	rootCmd.PersistentFlags().StringVar(&flags.configPath, "config", "", "Path to config file")

	rootCmd.AddCommand(newNotebookCmd(flags))
	rootCmd.AddCommand(newAuthCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	rootCmd.AddCommand(newVersionCliCmd())
	return rootCmd
}

// ExitCode extracts exit code from an error.
func ExitCode(err error) int {
	return 1
}

func newVersionCliCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("notebooklm-pp-cli %s\n", version)
		},
	}
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health and auth",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			checks := []map[string]any{
				{"name": "go", "status": "ok", "detail": runtime.Version()},
				{"name": "os", "status": "ok", "detail": fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)},
			}
			if err != nil {
				checks = append(checks, map[string]any{"name": "config", "status": "error", "detail": err.Error()})
			} else {
				authStatus := "missing"
				if cfg.AuthHeader != "" {
					authStatus = "ok"
				}
				checks = append(checks, map[string]any{
					"name":   "auth",
					"status": authStatus,
					"detail": cfg.Path,
				})
				if cfg.AuthHeader != "" {
					if hc, err := cfg.HTTPClient(); err == nil {
						if sess, err := nlm.Bootstrap(cmd.Context(), hc); err == nil {
							checks = append(checks, map[string]any{
								"name":   "session",
								"status": map[bool]string{true: "ok", false: "missing"}[sess.AT != ""],
								"detail": "bootstrap tokens",
							})
						}
					}
				}
			}
			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"checks": checks})
			}
			fmt.Println("notebooklm-pp-cli doctor")
			for _, c := range checks {
				fmt.Printf("  %s: %s (%v)\n", c["name"], c["status"], c["detail"])
			}
			return nil
		},
	}
}
