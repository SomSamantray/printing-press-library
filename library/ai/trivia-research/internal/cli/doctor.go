package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health and source connectivity",
		Long:  "Checks connectivity to all 9 podcast and blog sources. Use --fail-on to exit non-zero on warnings or errors.",
		Example: `  trivia-research-pp-cli doctor
  trivia-research-pp-cli doctor --json
  trivia-research-pp-cli doctor --fail-on warn
  trivia-research-pp-cli doctor --fail-on error`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{
				"version": version,
				"config":  "ok",
			}

			sources := getActiveSources("all")
			sourceReport := make(map[string]any)
			allOK := true
			totalEpisodes := 0
			var errors []string

			for _, src := range sources {
				episodes, err := src.Sync(1)
				if err != nil {
					sourceReport[src.Source()] = map[string]any{
						"name":   src.Name(),
						"status": "error",
						"error":  err.Error(),
					}
					allOK = false
					errors = append(errors, src.Name())
				} else {
					sourceReport[src.Source()] = map[string]any{
						"name":     src.Name(),
						"status":   "ok",
						"episodes": len(episodes),
					}
					totalEpisodes += len(episodes)
				}
			}

			report["sources"] = sourceReport
			report["sources_total"] = totalEpisodes

			if allOK {
				report["status"] = "healthy"
			} else {
				report["status"] = "degraded"
				report["degraded_sources"] = errors
			}

			if flags.asJSON {
				printJSONFiltered(cmd.OutOrStdout(), report, flags)
				return doctorExitForFailOn(failOn, report)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s CLI Health\n", bold("trivia-research-pp-cli"))
			fmt.Fprintf(w, "  Version: %s\n", version)

			if allOK {
				fmt.Fprintf(w, "  %s All 9 sources reachable (%d total episodes)\n", green("OK"), totalEpisodes)
			} else {
				fmt.Fprintf(w, "  %s %d/%d sources reachable\n", yellow("WARN"), 9-len(errors), 9)
			}

			fmt.Fprintln(w, "  Sources:")
			for _, src := range types.AllSources {
				if info, ok := sourceReport[src].(map[string]any); ok {
					status := info["status"].(string)
					eps := info["episodes"].(int)
					if status == "ok" {
						fmt.Fprintf(w, "    %s %s: %d episodes\n", green("\u2713"), info["name"], eps)
					} else {
						fmt.Fprintf(w, "    %s %s: %s\n", red("\u2717"), info["name"], info["error"])
					}
				}
			}

			return doctorExitForFailOn(failOn, report)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit non-zero for: warn (warnings + errors), error (errors only)")
	return cmd
}

func doctorExitForFailOn(failOn string, report map[string]any) error {
	if failOn == "" {
		return nil
	}
	status, _ := report["status"].(string)
	if status == "degraded" {
		return fmt.Errorf("doctor: %s", status)
	}
	return nil
}

func _dc() { _ = newDoctorCmd }
