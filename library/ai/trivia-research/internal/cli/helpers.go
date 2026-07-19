package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/scrapers"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
	"github.com/spf13/cobra"
)

var As = errors.As

func colorEnabled() bool {
	if noColor {
		return false
	}
	if !humanFriendly {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		fi, err := f.Stat()
		if err != nil {
			return false
		}
		return (fi.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

func bold(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

func green(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func yellow(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func red(s string) string {
	if !colorEnabled() {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func tableWriter(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

func outputJSON(cmd *cobra.Command, output types.ResearchOutput) error {
	if flagDef.compact {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
	}

	var data interface{} = output
	if flagDef.selectFields != "" {
		filtered := make([]map[string]interface{}, 0)
		for _, src := range output.Sources {
			for _, r := range src.Results {
				item := filterResult(r, flagDef.selectFields)
				filtered = append(filtered, item)
			}
		}
		data = filtered
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func outputCSV(cmd *cobra.Command, output types.ResearchOutput) error {
	writer := csv.NewWriter(cmd.OutOrStdout())
	writer.Write([]string{"title", "url", "date", "excerpt", "score", "source"})
	for _, src := range output.Sources {
		for _, r := range src.Results {
			writer.Write([]string{r.Title, r.URL, r.Date, Capped(r.Excerpt, 300), fmt.Sprintf("%.1f", r.Score), r.Source})
		}
	}
	writer.Flush()
	return writer.Error()
}

func outputTable(cmd *cobra.Command, output types.ResearchOutput) error {
	w := tableWriter(cmd.OutOrStdout())
	fmt.Fprintln(w, "SCORE\tTITLE\tSOURCE\tDATE")
	for _, src := range output.Sources {
		for _, r := range src.Results {
			fmt.Fprintf(w, "%.1f\t%s\t%s\t%s\n", r.Score, Capped(r.Title, 60), r.Source, Capped(r.Date, 20))
		}
	}
	return w.Flush()
}

func outputPlain(cmd *cobra.Command, output types.ResearchOutput) error {
	for _, src := range output.Sources {
		for _, r := range src.Results {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%.1f\n", r.Title, r.URL, r.Source, r.Score)
		}
	}
	return nil
}

func printOutput(cmd *cobra.Command, output types.ResearchOutput) error {
	if flagDef.asJSON {
		return outputJSON(cmd, output)
	}
	if flagDef.csvFormat {
		return outputCSV(cmd, output)
	}
	if flagDef.table {
		return outputTable(cmd, output)
	}
	if flagDef.plain {
		return outputPlain(cmd, output)
	}
	if flagDef.quiet {
		return nil
	}
	return outputTable(cmd, output)
}

func outputCompact(cmd *cobra.Command, output types.ResearchOutput, outputFile, saveDir string) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\U0001f52c DeepResearch v%s \u00b7 synced %s\n\n", version, time.Now().Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Topic:** %s\n", output.Topic))
	sb.WriteString(fmt.Sprintf("**Keywords:** %s\n\n", strings.Join(output.Keywords, ", ")))

	for _, src := range output.Sources {
		emoji := types.SourceEmoji[src.Source]
		if src.Error != "" {
			sb.WriteString(fmt.Sprintf("### %s %s: ERROR - %s\n\n", emoji, src.Name, src.Error))
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s %s: %d results\n\n", emoji, src.Name, src.Count))
		for i, item := range src.Results {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("\n  ... and %d more results\n", len(src.Results)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("\n**%d. %s** (score: %.1f)\n", i+1, item.Title, item.Score))
			if item.Date != "" {
				sb.WriteString(fmt.Sprintf("  - Date: %s\n", item.Date))
			}
			sb.WriteString(fmt.Sprintf("  - URL: %s\n", item.URL))
			if item.Excerpt != "" {
				sb.WriteString(fmt.Sprintf("  - Excerpt: %s\n", Capped(item.Excerpt, 300)))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("---\n")
	sb.WriteString("<!-- EVIDENCE FOR SYNTHESIS: read this, do not emit verbatim. -->\n")
	sb.WriteString("## Ranked Evidence Clusters\n\n")
	allItems := scrapers.MergeAndSort(allResults(output))
	for i, item := range allItems {
		if i >= 20 {
			break
		}
		sb.WriteString(fmt.Sprintf("### %d. %s (score %.1f, source: %s)\n", i+1, item.Title, item.Score, item.Source))
		sb.WriteString(fmt.Sprintf("  - [%s] %s\n", item.Source, item.Title))
		if item.Date != "" {
			sb.WriteString(fmt.Sprintf("  - %s | score:%.1f\n", item.Date, item.Score))
		}
		if item.Excerpt != "" {
			sb.WriteString(fmt.Sprintf("  - \"%s\"\n", Capped(item.Excerpt, 200)))
		}
		sb.WriteString(fmt.Sprintf("  - %s\n\n", item.URL))
	}
	sb.WriteString("<!-- END EVIDENCE FOR SYNTHESIS -->\n\n")

	active := activeSourcesList(output)
	sb.WriteString("---\n")
	sb.WriteString("\u2705 All agents reported back!\n")
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f944 Gastropod: %d items\n", srcCount(output, "gastropod")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f4c8 Freakonomics: %d items\n", srcCount(output, "freakonomics")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f30d Planet Money: %d items\n", srcCount(output, "planetmoney")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f4a1 The Indicator: %d items\n", srcCount(output, "indicator")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f3a7 20K Hz: %d items\n", srcCount(output, "twentykhz")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f3db\ufe0f 99%% Invisible: %d items\n", srcCount(output, "99pi")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f4f0 SatPost: %d items\n", srcCount(output, "satpost")))
	sb.WriteString(fmt.Sprintf("\u251c\u2500 \U0001f4e1 Acquired: %d items\n", srcCount(output, "acquired")))
	sb.WriteString(fmt.Sprintf("\u2514\u2500 \U0001f3e6 Business Breakdowns: %d items\n", srcCount(output, "colossus")))
	sb.WriteString(fmt.Sprintf("Total: %d items across %d sources\n", output.TotalItems, len(active)))

	result := sb.String()
	fmt.Fprint(cmd.OutOrStdout(), result)

	savedPath := saveOutput(result, output.Topic, outputFile, saveDir)
	if savedPath != "" {
		fmt.Fprintf(os.Stderr, "[trivia-research] Saved output to %s\n", savedPath)
	}
}

func saveOutput(content, topic, outputFile, saveDir string) string {
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[trivia-research] warning: failed to save output to %s: %v\n", outputFile, err)
			return ""
		}
		return outputFile
	}
	dir := saveDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		if alt := os.Getenv("DEEPRESEARCH_MEMORY_DIR"); alt != "" {
			dir = alt
		} else {
			dir = filepath.Join(home, "Documents", "DeepResearch")
		}
	}
	cliutil.EnsureDir(dir)
	slug := scrapers.SlugifyTopic(topic)
	timestamp := time.Now().Format("20060102-150405")
	filename := filepath.Join(dir, fmt.Sprintf("%s-%s-raw.md", slug, timestamp))
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[trivia-research] warning: failed to save output to %s: %v\n", filename, err)
		return ""
	}
	return filename
}

func printJSONFiltered(w io.Writer, data any, flags *rootFlags) error {
	var out interface{} = data
	if flags.selectFields != "" && flags.selectFields != "*" {
		if m, ok := data.(map[string]any); ok {
			filtered := make(map[string]any)
			for _, f := range strings.Split(flags.selectFields, ",") {
				f = strings.TrimSpace(f)
				if v, ok := m[f]; ok {
					filtered[f] = v
				}
			}
			out = filtered
		}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func allResults(output types.ResearchOutput) map[string][]types.SearchResult {
	m := make(map[string][]types.SearchResult)
	for _, src := range output.Sources {
		m[src.Source] = src.Results
	}
	return m
}

func srcCount(output types.ResearchOutput, name string) int {
	for _, src := range output.Sources {
		if src.Source == name {
			return src.Count
		}
	}
	return 0
}

func activeSourcesList(output types.ResearchOutput) []string {
	var active []string
	for _, src := range output.Sources {
		if src.Count > 0 {
			active = append(active, src.Source)
		}
	}
	return active
}

func filterResult(r types.SearchResult, fields string) map[string]interface{} {
	item := make(map[string]interface{})
	for _, f := range strings.Split(fields, ",") {
		switch strings.TrimSpace(f) {
		case "title":
			item["title"] = r.Title
		case "url":
			item["url"] = r.URL
		case "date":
			item["date"] = r.Date
		case "excerpt":
			item["excerpt"] = r.Excerpt
		case "score":
			item["score"] = r.Score
		case "source":
			item["source"] = r.Source
		case "matched_keywords":
			item["matched_keywords"] = r.MatchedKeywords
		}
	}
	return item
}

func Capped(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var sentinelUsage = fmt.Errorf("")
var _ = bytes.Buffer{}
var _ = sentinelUsage
