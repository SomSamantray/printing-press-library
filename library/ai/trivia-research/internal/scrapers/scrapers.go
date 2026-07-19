package scrapers

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type Scraper interface {
	Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error)
	Source() string
	Name() string
	Sync(maxPages int) ([]types.Episode, error)
}

var (
	reNonAlpha   = regexp.MustCompile(`[^a-zA-Z0-9\s]+`)
	reMultispace = regexp.MustCompile(`\s+`)
)

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "can": true, "shall": true,
	"to": true, "of": true, "in": true, "for": true, "on": true, "with": true,
	"at": true, "by": true, "from": true, "and": true, "or": true, "but": true,
	"not": true, "no": true, "so": true, "if": true, "then": true, "than": true,
	"that": true, "this": true, "these": true, "those": true, "it": true, "its": true,
	"we": true, "they": true, "he": true, "she": true, "his": true, "her": true,
	"their": true, "our": true, "my": true, "your": true,
}

func NormalizeTopic(topic string) []string {
	lower := strings.ToLower(topic)
	words := strings.Fields(reNonAlpha.ReplaceAllString(lower, " "))
	var keywords []string
	for _, w := range words {
		if stopwords[w] || len(w) <= 1 {
			continue
		}
		keywords = append(keywords, w)
	}
	return keywords
}

func KeywordMatchesText(text string, keyword string) bool {
	textLower := strings.ToLower(text)
	kwLower := strings.ToLower(keyword)
	if len(keyword) < 4 {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(kwLower) + `\b`)
		return re.MatchString(textLower)
	}
	return strings.Contains(textLower, kwLower)
}

func ScoreText(text string, keywords []string) (float64, []string) {
	textLower := strings.ToLower(text)
	score := 0.0
	var matched []string
	for _, kw := range keywords {
		if KeywordMatchesText(textLower, kw) {
			score += 1.0
			matched = append(matched, kw)
		}
	}
	phrase := strings.Join(keywords, " ")
	if strings.Contains(textLower, phrase) {
		score += float64(len(keywords)) * 0.5
	}
	return score, matched
}

func ExtractRelevantSnippet(text string, keywords []string) string {
	if len(keywords) == 0 || len(text) == 0 {
		return ""
	}
	textLower := strings.ToLower(text)
	bestPos := -1
	bestCount := 0

	for _, kw := range keywords {
		pos := strings.Index(textLower, strings.ToLower(kw))
		if pos < 0 {
			continue
		}
		windowSize := 100
		start := max(0, pos-windowSize*2)
		end := min(len(textLower), pos+windowSize*2)
		windowText := textLower[start:end]
		count := 0
		for _, k := range keywords {
			if strings.Contains(windowText, strings.ToLower(k)) {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			bestPos = pos
		}
	}

	if bestPos < 0 {
		return ""
	}
	windowSize := 100
	start := max(0, bestPos-windowSize)
	end := min(len(text), bestPos+windowSize)
	snippet := text[start:end]
	snippet = reMultispace.ReplaceAllString(snippet, " ")
	return strings.TrimSpace(snippet)
}

func RescoreContent(titleScore float64, content string, keywords []string) float64 {
	if content == "" {
		return titleScore
	}
	contentLower := strings.ToLower(content)
	tscore := 0.0
	for _, kw := range keywords {
		if KeywordMatchesText(contentLower, kw) {
			tscore += 1.0
		}
	}
	phrase := strings.Join(keywords, " ")
	if strings.Contains(contentLower, phrase) {
		tscore += float64(len(keywords)) * 0.5
	}
	return titleScore*0.3 + tscore*0.7
}

func MergeAndSort(allResults map[string][]types.SearchResult) []types.SearchResult {
	var merged []types.SearchResult
	for _, results := range allResults {
		merged = append(merged, results...)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})
	return merged
}

func Capped(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func SlugifyTopic(topic string) string {
	slug := strings.ToLower(topic)
	slug = reNonAlpha.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 64 {
		slug = slug[:64]
	}
	if slug == "" {
		slug = "research"
	}
	return slug
}

func ScoreTitleExcerpt(title, excerpt string, keywords []string) (float64, []string) {
	text := fmt.Sprintf("%s %s", title, excerpt)
	return ScoreText(text, keywords)
}
