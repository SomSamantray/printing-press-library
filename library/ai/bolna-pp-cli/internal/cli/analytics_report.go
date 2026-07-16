// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored analytics workflow for the Bolna CLI.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/bolna-pp-cli/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/bolna-pp-cli/internal/config"
	"github.com/spf13/cobra"
)

type analyticsSource struct {
	Label string
	Token string
}

type analyticsExecution struct {
	Source      string
	AgentID     string
	AgentName   string
	Status      string
	Provider    string
	CreatedAt   time.Time
	Duration    float64
	Cost        float64
	LatencyMS   float64
	Answered    bool
	HasAnswered bool
}

type analyticsBucket struct {
	Calls            int       `json:"calls"`
	Successful       int       `json:"successful"`
	Failed           int       `json:"failed"`
	Answered         int       `json:"answered"`
	AnsweredKnown    int       `json:"answered_known"`
	TotalDuration    float64   `json:"total_duration_seconds"`
	AverageDuration  float64   `json:"average_duration_seconds"`
	MedianDuration   float64   `json:"median_duration_seconds"`
	TotalCost        float64   `json:"total_cost"`
	AverageCost      float64   `json:"average_cost"`
	AverageLatencyMS float64   `json:"average_latency_ms"`
	CompletionRate   float64   `json:"completion_rate"`
	SuccessRate      float64   `json:"success_rate"`
	AnswerRate       float64   `json:"answer_rate"`
	DurationSamples  []float64 `json:"-"`
	LatencySamples   []float64 `json:"-"`
}

type analyticsReport struct {
	ReportType        string                     `json:"report_type"`
	From              string                     `json:"from"`
	To                string                     `json:"to"`
	Sources           []string                   `json:"sources"`
	AgentFilter       []string                   `json:"agent_filter,omitempty"`
	MetricFilter      []string                   `json:"metric_filter"`
	ExecutionCount    int                        `json:"execution_count"`
	Accounts          map[string]analyticsBucket `json:"accounts"`
	Agents            map[string]analyticsBucket `json:"agents"`
	Trends            []analyticsTrend           `json:"trends"`
	Insights          []string                   `json:"insights"`
	Warnings          []string                   `json:"warnings,omitempty"`
	MetricDefinitions map[string]string          `json:"metric_definitions"`
}

type analyticsTrend struct {
	Period string                     `json:"period"`
	Values map[string]analyticsBucket `json:"values"`
}

func newAnalyticsReportCmd(flags *rootFlags) *cobra.Command {
	var sourceFlags []string
	var from, to, agentName, provider, status, groupBy string
	var agentIDs, metrics []string
	var pageSize, maxPages int

	cmd := &cobra.Command{
		Use:     "report",
		Aliases: []string{"compare", "deep-dive"},
		Short:   "Compare call history and metrics across Bolna accounts",
		Long: `Build a cross-account Bolna report from agent execution history.

Each --source is LABEL=ENV_VAR, for example --source sales=BOLNA_SALES_KEY.
The environment variable contains the bearer key and is never written to the report.
Date boundaries are inclusive and interpreted as UTC dates. Use --metric all for
the complete metric set, --group-by day for daily trends, and --agent-id or
--agent-name to focus the report on selected agents.`,
		Example: `  bolna-pp-cli-pp-cli analytics report \
    --source account-a=BOLNA_ACCOUNT_A_KEY \
    --source account-b=BOLNA_ACCOUNT_B_KEY \
    --from 2026-06-01 --to 2026-07-16 --metric all --group-by day --agent --no-cache`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel-feature": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" && to == "" && len(sourceFlags) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"command":  "analytics report",
						"required": []string{"--source LABEL=ENV_VAR", "--from YYYY-MM-DD", "--to YYYY-MM-DD"},
						"hint":     "Provide one or more bearer-key sources and an inclusive UTC date interval.",
					}, flags)
				}
				return cmd.Help()
			}
			if from == "" || to == "" {
				return usageErr(fmt.Errorf("--from and --to are required (YYYY-MM-DD)"))
			}
			start, end, err := parseAnalyticsWindow(from, to)
			if err != nil {
				return usageErr(err)
			}
			sources, err := resolveAnalyticsSources(sourceFlags)
			if err != nil {
				return usageErr(err)
			}
			selectedMetrics := normalizeMetrics(metrics)
			report, err := collectAnalyticsReport(cmd.Context(), flags, sources, start, end, agentIDs, agentName, provider, status, groupBy, pageSize, maxPages, selectedMetrics)
			if err != nil {
				return err
			}
			if wantsMachineOutput(flags) {
				raw, marshalErr := json.Marshal(report)
				if marshalErr != nil {
					return marshalErr
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live"})
			}
			printAnalyticsReport(cmd, report)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&sourceFlags, "source", nil, "Account source as LABEL=ENV_VAR; repeat for multiple accounts")
	cmd.Flags().StringVar(&from, "from", "", "Inclusive UTC start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Inclusive UTC end date (YYYY-MM-DD)")
	cmd.Flags().StringArrayVar(&metrics, "metric", nil, "Metric to include; repeat or use comma-separated values (all, calls, success, answer, duration, cost, latency, status, provider)")
	cmd.Flags().StringArrayVar(&agentIDs, "agent-id", nil, "Restrict to an agent ID; repeatable")
	cmd.Flags().StringVar(&agentName, "agent-name", "", "Restrict to agents whose name contains this text")
	cmd.Flags().StringVar(&provider, "provider", "", "Restrict to a provider name")
	cmd.Flags().StringVar(&status, "status", "", "Restrict to a status value")
	cmd.Flags().StringVar(&groupBy, "group-by", "account", "Trend grouping: account, agent, day, week, status, provider")
	cmd.Flags().IntVar(&pageSize, "page-size", 50, "Execution page size (maximum 50)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 500, "Maximum execution pages per agent; date-aware stopping normally ends earlier")
	return cmd
}

func resolveAnalyticsSources(raw []string) ([]analyticsSource, error) {
	if len(raw) == 0 {
		if token := os.Getenv("BOLNA_PP_CLI_BEARER_AUTH"); token != "" {
			return []analyticsSource{{Label: "current", Token: token}}, nil
		}
		return nil, fmt.Errorf("at least one --source LABEL=ENV_VAR is required, or set BOLNA_PP_CLI_BEARER_AUTH")
	}
	nameRE := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	seen := map[string]bool{}
	result := make([]analyticsSource, 0, len(raw))
	for _, spec := range raw {
		parts := strings.SplitN(spec, "=", 2)
		if len(parts) != 2 || parts[0] == "" || !nameRE.MatchString(parts[1]) {
			return nil, fmt.Errorf("invalid --source %q; expected LABEL=ENV_VAR", spec)
		}
		if seen[parts[0]] {
			return nil, fmt.Errorf("duplicate source label %q", parts[0])
		}
		seen[parts[0]] = true
		token := os.Getenv(parts[1])
		if token == "" {
			return nil, fmt.Errorf("environment variable %s for source %q is empty", parts[1], parts[0])
		}
		result = append(result, analyticsSource{Label: parts[0], Token: token})
	}
	return result, nil
}

func parseAnalyticsWindow(from, to string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --from %q; use YYYY-MM-DD", from)
	}
	endDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --to %q; use YYYY-MM-DD", to)
	}
	if endDate.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to must be on or after --from")
	}
	return start.UTC(), endDate.UTC().Add(24 * time.Hour), nil
}

func normalizeMetrics(raw []string) map[string]bool {
	result := map[string]bool{"calls": true, "success": true, "answer": true, "duration": true, "cost": true, "latency": true, "status": true, "provider": true}
	if len(raw) == 0 {
		return result
	}
	result = map[string]bool{}
	for _, item := range raw {
		for _, metric := range strings.Split(item, ",") {
			metric = strings.ToLower(strings.TrimSpace(metric))
			if metric == "all" {
				return map[string]bool{"calls": true, "success": true, "answer": true, "duration": true, "cost": true, "latency": true, "status": true, "provider": true}
			}
			if metric != "" {
				result[metric] = true
			}
		}
	}
	return result
}

func collectAnalyticsReport(ctx context.Context, flags *rootFlags, sources []analyticsSource, start, end time.Time, agentIDs []string, agentName, provider, status, groupBy string, pageSize, maxPages int, metrics map[string]bool) (analyticsReport, error) {
	if pageSize < 1 || pageSize > 50 {
		return analyticsReport{}, fmt.Errorf("--page-size must be between 1 and 50")
	}
	if maxPages < 1 {
		return analyticsReport{}, fmt.Errorf("--max-pages must be positive")
	}
	report := analyticsReport{ReportType: "cross_account_call_history", From: start.Format("2006-01-02"), To: end.Add(-24 * time.Hour).Format("2006-01-02"), Accounts: map[string]analyticsBucket{}, Agents: map[string]analyticsBucket{}, MetricFilter: sortedMetricNames(metrics), Trends: []analyticsTrend{}, MetricDefinitions: analyticsMetricDefinitions()}
	for _, source := range sources {
		report.Sources = append(report.Sources, source.Label)
		c, err := analyticsClient(flags, source.Token)
		if err != nil {
			report.Warnings = append(report.Warnings, source.Label+": could not configure client")
			continue
		}
		agents, err := fetchAnalyticsAgents(ctx, c, pageSize)
		if err != nil {
			report.Warnings = append(report.Warnings, source.Label+": "+err.Error())
			continue
		}
		allowed := map[string]bool{}
		for _, id := range agentIDs {
			allowed[id] = true
		}
		for _, agent := range agents {
			if len(allowed) > 0 && !allowed[agent.ID] {
				continue
			}
			if agentName != "" && !strings.Contains(strings.ToLower(agent.Name), strings.ToLower(agentName)) {
				continue
			}
			rows, capped, fetchErr := fetchAnalyticsExecutions(ctx, c, agent.ID, pageSize, start, maxPages)
			if fetchErr != nil {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s/%s: %v", source.Label, safeLabel(agent.Name, agent.ID), fetchErr))
				continue
			}
			if capped {
				report.Warnings = append(report.Warnings, fmt.Sprintf("%s/%s: execution pagination reached --max-pages=%d", source.Label, safeLabel(agent.Name, agent.ID), maxPages))
			}
			for _, raw := range rows {
				exec, ok := normalizeAnalyticsExecution(raw, source.Label, agent)
				if !ok || exec.CreatedAt.Before(start) || !exec.CreatedAt.Before(end) {
					continue
				}
				if provider != "" && !strings.Contains(strings.ToLower(exec.Provider), strings.ToLower(provider)) {
					continue
				}
				if status != "" && !strings.EqualFold(exec.Status, status) {
					continue
				}
				report.ExecutionCount++
				addAnalyticsMapBucket(report.Accounts, source.Label, exec)
				agentKey := source.Label + "/" + safeLabel(exec.AgentName, exec.AgentID)
				addAnalyticsMapBucket(report.Agents, agentKey, exec)
				addAnalyticsTrend(&report.Trends, groupBy, exec)
			}
		}
	}
	finalizeAnalyticsBuckets(report.Accounts)
	finalizeAnalyticsBuckets(report.Agents)
	finalizeAnalyticsTrends(report.Trends)
	report.Insights = deriveAnalyticsInsights(report)
	return report, nil
}

type analyticsAgent struct {
	ID   string
	Name string
}

func analyticsClient(flags *rootFlags, token string) (*client.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	cfg.BolnaPpCliBearerAuth = token
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	c.DryRun, c.NoCache = flags.dryRun, flags.noCache
	return c, nil
}

func fetchAnalyticsAgents(ctx context.Context, c *client.Client, pageSize int) ([]analyticsAgent, error) {
	data, err := c.Get(ctx, "/v2/agent/all", map[string]string{"page_number": "1", "page_size": strconv.Itoa(pageSize)})
	if err != nil {
		return nil, err
	}
	rows := jsonRows(data)
	result := make([]analyticsAgent, 0, len(rows))
	for _, row := range rows {
		id := stringValue(row, "id", "agent_id")
		if id == "" {
			continue
		}
		result = append(result, analyticsAgent{ID: id, Name: stringValue(row, "agent_name", "name")})
	}
	return result, nil
}

func fetchAnalyticsExecutions(ctx context.Context, c *client.Client, agentID string, pageSize int, start time.Time, maxPages int) ([]map[string]any, bool, error) {
	var result []map[string]any
	seenIDs := map[string]bool{}
	for page := 1; page <= maxPages; page++ {
		path := "/v2/agent/" + agentID + "/executions"
		data, err := c.Get(ctx, path, map[string]string{"page_number": strconv.Itoa(page), "page_size": strconv.Itoa(pageSize)})
		if err != nil {
			return result, false, err
		}
		rows := jsonRows(data)
		newRows := 0
		freshRows := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			id := stringValue(row, "id", "execution_id")
			if id == "" || !seenIDs[id] {
				newRows++
				freshRows = append(freshRows, row)
			}
			if id != "" {
				seenIDs[id] = true
			}
		}
		result = append(result, freshRows...)
		allBeforeStart := len(rows) > 0
		for _, row := range rows {
			if firstTime(row, "created_at", "initiated_at", "scheduled_at", "updated_at").IsZero() || !firstTime(row, "created_at", "initiated_at", "scheduled_at", "updated_at").Before(start) {
				allBeforeStart = false
				break
			}
		}
		if len(rows) < pageSize || (page > 1 && newRows == 0) || allBeforeStart {
			break
		}
	}
	return result, len(result) > 0 && len(result) >= maxPages*pageSize, nil
}

func jsonRows(data []byte) []map[string]any {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return nil
	}
	if obj, ok := value.(map[string]any); ok {
		for _, key := range []string{"results", "data", "executions", "items"} {
			if rows, ok := obj[key].([]any); ok {
				return anyRows(rows)
			}
		}
		return []map[string]any{obj}
	}
	if rows, ok := value.([]any); ok {
		return anyRows(rows)
	}
	return nil
}

func anyRows(rows []any) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		if obj, ok := item.(map[string]any); ok {
			result = append(result, obj)
		}
	}
	return result
}

func normalizeAnalyticsExecution(row map[string]any, source string, agent analyticsAgent) (analyticsExecution, bool) {
	created := firstTime(row, "created_at", "initiated_at", "scheduled_at", "updated_at")
	if created.IsZero() {
		return analyticsExecution{}, false
	}
	status := strings.ToLower(stringValue(row, "smart_status", "status", "processing_status"))
	answered, known := boolValue(row, "answered_by_voice_mail")
	if value, ok := nestedBool(row, "telephony_data", "answered"); ok {
		answered, known = value, true
	}
	return analyticsExecution{Source: source, AgentID: agent.ID, AgentName: agent.Name, Status: status, Provider: stringValue(row, "provider"), CreatedAt: created, Duration: numberValue(row, "conversation_duration", "duration"), Cost: numberValue(row, "total_cost", "cost"), LatencyMS: nestedNumber(row, "latency_data", "total_latency", "latency_ms"), Answered: answered, HasAnswered: known}, true
}

func addAnalyticsBucket(bucket *analyticsBucket, exec analyticsExecution) {
	bucket.Calls++
	if isSuccessfulStatus(exec.Status) {
		bucket.Successful++
	}
	if isFailedStatus(exec.Status) {
		bucket.Failed++
	}
	if exec.HasAnswered {
		bucket.AnsweredKnown++
		if exec.Answered {
			bucket.Answered++
		}
	}
	if exec.Duration > 0 {
		bucket.TotalDuration += exec.Duration
		bucket.DurationSamples = append(bucket.DurationSamples, exec.Duration)
	}
	if exec.Cost > 0 {
		bucket.TotalCost += exec.Cost
	}
	if exec.LatencyMS > 0 {
		bucket.LatencySamples = append(bucket.LatencySamples, exec.LatencyMS)
	}
}

func addAnalyticsTrend(trends *[]analyticsTrend, groupBy string, exec analyticsExecution) {
	if groupBy == "" || groupBy == "account" || groupBy == "agent" {
		return
	}
	key := exec.Source
	switch groupBy {
	case "day":
		key = exec.CreatedAt.Format("2006-01-02")
	case "week":
		year, week := exec.CreatedAt.ISOWeek()
		key = fmt.Sprintf("%04d-W%02d", year, week)
	case "status":
		key = exec.Status
	case "provider":
		key = exec.Provider
	}
	for i := range *trends {
		if (*trends)[i].Period == key {
			addAnalyticsMapBucket((*trends)[i].Values, exec.Source, exec)
			return
		}
	}
	*trends = append(*trends, analyticsTrend{Period: key, Values: map[string]analyticsBucket{exec.Source: {}}})
	addAnalyticsMapBucket((*trends)[len(*trends)-1].Values, exec.Source, exec)
}

func addAnalyticsMapBucket(buckets map[string]analyticsBucket, key string, exec analyticsExecution) {
	bucket := buckets[key]
	addAnalyticsBucket(&bucket, exec)
	buckets[key] = bucket
}

func finalizeAnalyticsBuckets(buckets map[string]analyticsBucket) {
	for key, bucket := range buckets {
		finalizeAnalyticsBucket(&bucket)
		bucket.DurationSamples, bucket.LatencySamples = nil, nil
		buckets[key] = bucket
	}
}
func finalizeAnalyticsTrends(trends []analyticsTrend) {
	sort.Slice(trends, func(i, j int) bool { return trends[i].Period < trends[j].Period })
	for i := range trends {
		finalizeAnalyticsBuckets(trends[i].Values)
	}
}
func finalizeAnalyticsBucket(bucket *analyticsBucket) {
	if bucket.Calls > 0 {
		bucket.SuccessRate = percent(bucket.Successful, bucket.Calls)
		bucket.CompletionRate = percent(bucket.Calls-bucket.Failed, bucket.Calls)
	}
	if bucket.AnsweredKnown > 0 {
		bucket.AnswerRate = percent(bucket.Answered, bucket.AnsweredKnown)
	}
	if len(bucket.DurationSamples) > 0 {
		bucket.AverageDuration = bucket.TotalDuration / float64(len(bucket.DurationSamples))
		bucket.MedianDuration = median(bucket.DurationSamples)
	}
	if bucket.Calls > 0 {
		bucket.AverageCost = bucket.TotalCost / float64(bucket.Calls)
	}
	if len(bucket.LatencySamples) > 0 {
		var sum float64
		for _, v := range bucket.LatencySamples {
			sum += v
		}
		bucket.AverageLatencyMS = sum / float64(len(bucket.LatencySamples))
	}
}
func percent(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return math.Round(float64(n) * 10000 / float64(d) / 100)
}
func median(values []float64) float64 {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	n := len(copyValues)
	if n%2 == 1 {
		return copyValues[n/2]
	}
	return (copyValues[n/2-1] + copyValues[n/2]) / 2
}

func deriveAnalyticsInsights(report analyticsReport) []string {
	var result []string
	if len(report.Accounts) >= 2 {
		keys := make([]string, 0, len(report.Accounts))
		for k := range report.Accounts {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return report.Accounts[keys[i]].SuccessRate > report.Accounts[keys[j]].SuccessRate
		})
		best, worst := report.Accounts[keys[0]], report.Accounts[keys[len(keys)-1]]
		if best.Calls > 0 && worst.Calls > 0 && best.SuccessRate != worst.SuccessRate {
			result = append(result, fmt.Sprintf("%s has the highest success rate at %.2f%%; %s is lowest at %.2f%%.", keys[0], best.SuccessRate, keys[len(keys)-1], worst.SuccessRate))
		}
	}
	for key, bucket := range report.Agents {
		if bucket.Calls >= 10 && bucket.SuccessRate < 50 {
			result = append(result, fmt.Sprintf("Agent %s has %.2f%% success across %d calls; investigate its failures.", key, bucket.SuccessRate, bucket.Calls))
		}
	}
	if len(report.Trends) >= 2 {
		first, last := report.Trends[0], report.Trends[len(report.Trends)-1]
		firstCalls, lastCalls := 0, 0
		for _, b := range first.Values {
			firstCalls += b.Calls
		}
		for _, b := range last.Values {
			lastCalls += b.Calls
		}
		if firstCalls > 0 && lastCalls > firstCalls {
			result = append(result, fmt.Sprintf("The latest %s period has %d calls versus %d in the earliest period.", "trend", lastCalls, firstCalls))
		}
	}
	if len(result) == 0 {
		result = append(result, "No statistically strong comparison signal was detected from the selected records; widen the date range or request --metric all.")
	}
	return result
}

func printAnalyticsReport(cmd *cobra.Command, report analyticsReport) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Bolna call-history report: %s to %s\n", report.From, report.To)
	fmt.Fprintf(out, "Executions: %d | Sources: %s\n\n", report.ExecutionCount, strings.Join(report.Sources, ", "))
	fmt.Fprintln(out, "Account\tCalls\tSuccess %\tAnswer %\tAvg Duration\tCost")
	for key, b := range report.Accounts {
		fmt.Fprintf(out, "%s\t%d\t%.2f\t%.2f\t%.1fs\t%.4f\n", key, b.Calls, b.SuccessRate, b.AnswerRate, b.AverageDuration, b.TotalCost)
	}
	fmt.Fprintln(out, "\nInsights:")
	for _, insight := range report.Insights {
		fmt.Fprintf(out, "- %s\n", insight)
	}
	if len(report.Warnings) > 0 {
		fmt.Fprintln(out, "\nWarnings:")
		for _, warning := range report.Warnings {
			fmt.Fprintf(out, "- %s\n", warning)
		}
	}
}

func analyticsMetricDefinitions() map[string]string {
	return map[string]string{"calls": "execution records in the selected window", "success": "records whose status is completed/success/complete", "answer": "answered_by_voice_mail or telephony_data.answered when present", "duration": "conversation_duration or duration, in seconds", "cost": "total_cost or cost", "latency": "latency_data.total_latency or latency_ms", "status": "status distribution retained in trend groups", "provider": "provider distribution retained in trend groups"}
}
func sortedMetricNames(metrics map[string]bool) []string {
	result := make([]string, 0, len(metrics))
	for k := range metrics {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
func safeLabel(name, id string) string {
	if name != "" {
		return name
	}
	return id
}
func stringValue(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			if v != nil {
				return fmt.Sprint(v)
			}
		}
	}
	return ""
}
func numberValue(obj map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			if n := toNumber(v); n != 0 {
				return n
			}
		}
	}
	return 0
}
func nestedNumber(obj map[string]any, parent string, keys ...string) float64 {
	if nested, ok := obj[parent].(map[string]any); ok {
		return numberValue(nested, keys...)
	}
	return 0
}
func boolValue(obj map[string]any, key string) (bool, bool) {
	v, ok := obj[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}
func nestedBool(obj map[string]any, parent, key string) (bool, bool) {
	if nested, ok := obj[parent].(map[string]any); ok {
		return boolValue(nested, key)
	}
	return false, false
}
func toNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	case string:
		n, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n
	}
	return 0
}
func firstTime(obj map[string]any, keys ...string) time.Time {
	for _, key := range keys {
		if s := stringValue(obj, key); s != "" {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05", "2006-01-02"} {
				if t, err := time.Parse(layout, s); err == nil {
					return t.UTC()
				}
			}
		}
	}
	return time.Time{}
}
func isSuccessfulStatus(status string) bool {
	switch strings.ToLower(status) {
	case "completed", "complete", "success", "successful", "done", "answered":
		return true
	}
	return false
}
func isFailedStatus(status string) bool {
	switch strings.ToLower(status) {
	case "failed", "failure", "error", "cancelled", "canceled", "no-answer", "no_answer":
		return true
	}
	return false
}
