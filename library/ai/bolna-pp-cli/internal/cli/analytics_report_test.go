package cli

import (
	"testing"
	"time"
)

func TestParseAnalyticsWindowInclusiveEnd(t *testing.T) {
	start, end, err := parseAnalyticsWindow("2026-06-01", "2026-07-16")
	if err != nil {
		t.Fatal(err)
	}
	if start.Format(time.RFC3339) != "2026-06-01T00:00:00Z" {
		t.Fatalf("unexpected start: %s", start)
	}
	if end.Format(time.RFC3339) != "2026-07-17T00:00:00Z" {
		t.Fatalf("unexpected exclusive end: %s", end)
	}
}

func TestNormalizeAnalyticsExecutionAndBucket(t *testing.T) {
	row := map[string]any{
		"id":                     "execution-1",
		"created_at":             "2026-07-16T06:32:47.383283+00:00",
		"status":                 "completed",
		"conversation_duration":  42.0,
		"total_cost":             1.25,
		"answered_by_voice_mail": false,
	}
	exec, ok := normalizeAnalyticsExecution(row, "account-a", analyticsAgent{ID: "agent-1", Name: "Agent One"})
	if !ok || exec.CreatedAt.IsZero() {
		t.Fatal("expected normalized execution")
	}
	bucket := analyticsBucket{}
	addAnalyticsBucket(&bucket, exec)
	finalizeAnalyticsBucket(&bucket)
	if bucket.Calls != 1 || bucket.Successful != 1 || bucket.AverageDuration != 42 || bucket.TotalCost != 1.25 {
		t.Fatalf("unexpected bucket: %+v", bucket)
	}
}

func TestAnalyticsDuplicatePageStopsAndDeduplicates(t *testing.T) {
	rows := []map[string]any{{"id": "one"}, {"id": "two"}}
	seen := map[string]bool{}
	fresh := 0
	for _, row := range rows {
		id := stringValue(row, "id", "execution_id")
		if !seen[id] {
			fresh++
		}
		seen[id] = true
	}
	for _, row := range rows {
		id := stringValue(row, "id", "execution_id")
		if !seen[id] {
			fresh++
		}
	}
	if fresh != 2 {
		t.Fatalf("duplicate page should not add records: %d", fresh)
	}
}
