package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/scrapers"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

func newResearchCmd(flags *rootFlags) *cobra.Command {
	var outputFile string
	var saveDir string
	var maxEpisodes int

	cmd := &cobra.Command{
		Use:   "research [topic]",
		Short: "Search podcast transcripts and blogs for a topic",
		Long:  "Search all 9 podcast and blog sources in parallel: Gastropod, Freakonomics Radio, Planet Money, The Indicator, 20K Hz, 99% Invisible, SatPost, Acquired, and Business Breakdowns.\n\nEach source is scored by keyword relevance. High-scoring matches trigger transcript deep-fetch with 30/70 weighted rescoring. Results are merged and sorted across all sources.\n\nTopic can be provided as a positional argument or via stdin.",
		Example: `  trivia-research-pp-cli research "economics"
  trivia-research-pp-cli research "AI intelligence" --sources planetmoney,indicator,freakonomics --json
  trivia-research-pp-cli research "supply chain" --emit compact --max-episodes 10
  trivia-research-pp-cli research "fermentation" --csv --select title,url,score
  echo "market economics" | trivia-research-pp-cli research --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var topic string
			if len(args) == 0 {
				if flags.noInput {
					return fmt.Errorf("topic required; provide as positional argument or via stdin")
				}
				var buf [4096]byte
				n, err := os.Stdin.Read(buf[:])
				if n == 0 {
					return cmd.Help()
				}
				topic = string(buf[:n])
				topic = scrapers.Capped(topic, 256)
				_ = err
			} else {
				topic = args[0]
			}

			keywords := scrapers.NormalizeTopic(topic)

			activeScrapers := getActiveSources(flags.sources)
			if len(activeScrapers) == 0 {
				return fmt.Errorf("no active sources; check --sources flag")
			}

			type sourceWResult struct {
				src     scrapers.Scraper
				results []types.SearchResult
				err     error
			}

			resultsChan := make(chan sourceWResult, len(activeScrapers))
			for _, src := range activeScrapers {
				go func(s scrapers.Scraper) {
					res, err := s.Search(keywords, maxEpisodes)
					resultsChan <- sourceWResult{src: s, results: res, err: err}
				}(src)
			}

			var sourceResults []types.SourceResult
			totalItems := 0
			for range activeScrapers {
				wr := <-resultsChan
				sr := types.SourceResult{
					Source: wr.src.Source(),
					Name:   wr.src.Name(),
				}
				if wr.err != nil {
					sr.Error = wr.err.Error()
				} else {
					sr.Results = wr.results
					sr.Count = len(wr.results)
					totalItems += sr.Count
				}
				sourceResults = append(sourceResults, sr)
			}

			srcOrder := map[string]int{}
			for i, src := range types.AllSources {
				srcOrder[src] = i
			}
			for i := 0; i < len(sourceResults); i++ {
				for j := i + 1; j < len(sourceResults); j++ {
					if srcOrder[sourceResults[j].Source] < srcOrder[sourceResults[i].Source] {
						sourceResults[i], sourceResults[j] = sourceResults[j], sourceResults[i]
					}
				}
			}

			output := types.ResearchOutput{
				Topic:        topic,
				Keywords:     keywords,
				Sources:      sourceResults,
				TotalItems:   totalItems,
				TotalSources: countActive(sourceResults),
				Elapsed:      time.Now().Format(time.RFC3339),
			}

			if flags.emit == "compact" && !flags.asJSON && !flags.csvFormat && !flags.table && !flags.plain && !flags.quiet {
				outputCompact(cmd, output, outputFile, saveDir)
				return nil
			}

			return printOutput(cmd, output)
		},
	}

	cmd.Flags().StringVar(&outputFile, "output", "", "Save output to specific file path")
	cmd.Flags().StringVar(&saveDir, "save-dir", "", "Save raw output to directory")
	cmd.Flags().IntVar(&maxEpisodes, "max-episodes", 50, "Max episodes to scan per source (default: 50)")
	return cmd
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var maxPages int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pre-fetch episode archives into local store",
		Example: `  trivia-research-pp-cli sync
  trivia-research-pp-cli sync --sources gastropod,npr --max-pages 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			activeScrapers := getActiveSources(flags.sources)
			if maxPages <= 0 {
				maxPages = 5
			}

			for _, src := range activeScrapers {
				fmt.Fprintf(cmd.OutOrStdout(), "Syncing %s...", src.Name())
				episodes, err := src.Sync(maxPages)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStderr(), " FAIL: %v\n", err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), " OK (%d episodes)\n", len(episodes))
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Max archive pages to crawl per source (default: 5)")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print version",
		Example: "  trivia-research-pp-cli version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "trivia-research-pp-cli v%s\n", version)
			return nil
		},
	}
}

func _rc() { _ = newResearchCmd }
