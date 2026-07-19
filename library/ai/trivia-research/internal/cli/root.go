package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/config"
)

const version = "1.0.0"

type rootFlags struct {
	asJSON        bool
	compact       bool
	csvFormat     bool
	plain         bool
	table         bool
	quiet         bool
	dryRun        bool
	agent         bool
	noColor       bool
	noInput       bool
	yes           bool
	selectFields  string
	timeout       time.Duration
	emit          string
	sources       string
	configPath    string
	homePath      string
	humanFriendly bool
}

var flagDef rootFlags
var noColor bool
var humanFriendly bool

func RootCmd() *cobra.Command {
	var flags rootFlags
	return newRootCmd(&flags)
}

func Execute() error {
	var flags rootFlags
	cmd := newRootCmd(&flags)

	executedCmd, err := cmd.ExecuteC()
	if err != nil && strings.Contains(err.Error(), "unknown flag") {
		msg := err.Error()
		if idx := strings.Index(msg, "unknown flag: "); idx >= 0 {
			flagStr := strings.TrimSpace(msg[idx+len("unknown flag: "):])
			fmt.Fprintf(os.Stderr, "hint: flag %s is not recognized; try --help to see available flags\n", flagStr)
		}
	}
	_ = executedCmd
	return err
}

func newRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "trivia-research-pp-cli",
		Version: version,
		Short:   "Parallel deep research across 9 podcast transcript and blog archives",
		Long: `Search 9 podcast and blog sources in parallel: Gastropod, Freakonomics Radio, Planet Money,
The Indicator, 20K Hz, 99% Invisible, SatPost, Acquired, and Business Breakdowns.

Output formats: --json (structured JSON), --csv (CSV export), --table (formatted table),
--compact (agent-synthesis markdown), --plain (tab-separated), --quiet (no output).

Agent-native: --agent sets --json --no-color --no-input. Read topic from stdin.
Structured exit codes: 0=ok, 1=error, 2=usage, 3=not-found, 4=auth, 5=api, 7=rate-limit.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			noColor = flags.noColor
			if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
				noColor = true
			}
			humanFriendly = flags.humanFriendly
			if flags.agent {
				flags.asJSON = true
				flags.noInput = true
				noColor = true
			}
			flagDef = *flags
			return nil
		},
	}

	flagsFlag := cmd.PersistentFlags()
	flagsFlag.BoolVar(&flags.asJSON, "json", false, "Output as JSON")
	flagsFlag.BoolVar(&flags.compact, "compact", false, "Compact JSON output (no indentation)")
	flagsFlag.BoolVar(&flags.csvFormat, "csv", false, "Output as CSV")
	flagsFlag.BoolVar(&flags.table, "table", false, "Output as formatted table")
	flagsFlag.BoolVar(&flags.plain, "plain", false, "Output as tab-separated plain text")
	flagsFlag.BoolVar(&flags.quiet, "quiet", false, "Suppress all output")
	flagsFlag.BoolVar(&flags.dryRun, "dry-run", false, "Show what would be done without network calls")
	flagsFlag.BoolVar(&flags.agent, "agent", false, "Agent-friendly defaults (--json --no-color --no-input)")
	flagsFlag.BoolVar(&flags.noColor, "no-color", false, "Disable color output")
	flagsFlag.BoolVar(&flags.humanFriendly, "human-friendly", false, "Enable human-friendly colored output")
	flagsFlag.BoolVar(&flags.noInput, "no-input", false, "Non-interactive mode")
	flagsFlag.BoolVar(&flags.yes, "yes", false, "Auto-confirm all prompts")
	flagsFlag.StringVar(&flags.selectFields, "select", "", "Select specific output fields (comma-separated, e.g. title,score,source)")
	flagsFlag.DurationVar(&flags.timeout, "timeout", 300*time.Second, "Operation timeout")
	flagsFlag.StringVar(&flags.emit, "emit", "compact", "Output format: compact (markdown for synthesis) or json")
	flagsFlag.StringVar(&flags.sources, "sources", "all", "Comma-separated source list or 'all' (e.g. gastropod,freakonomics)")
	flagsFlag.StringVar(&flags.configPath, "config", "", "Config file path (default: ~/.config/trivia-research-pp-cli/config.toml)")
	flagsFlag.StringVar(&flags.homePath, "home", "", "Home directory override")

	cmd.AddCommand(newResearchCmd(flags))
	cmd.AddCommand(newSyncCmd(flags))
	cmd.AddCommand(newDoctorCmd(flags))
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newStatsCmd(flags))
	cmd.AddCommand(newListSourcesCmd(flags))

	return cmd
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ce, ok := err.(*cliError); ok {
		return ce.code
	}
	return 1
}

var _ = config.Config{}
var _ = errors.As
