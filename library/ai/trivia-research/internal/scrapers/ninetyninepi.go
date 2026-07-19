package scrapers

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type NinetyNinePIScraper struct{ http *client.HTTPClient }

func NewNinetyNinePIScraper() *NinetyNinePIScraper {
	return &NinetyNinePIScraper{http: client.New()}
}

func (s *NinetyNinePIScraper) Source() string { return "99pi" }
func (s *NinetyNinePIScraper) Name() string   { return "99% Invisible" }

var re99piEpisodeLink = regexp.MustCompile(`(?s)<a[^>]*href="(https://99percentinvisible\.org/episode/[\w-]+/?[^"]*)"[^>]*>(.*?)</a>`)
var re99piEntryContent = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*entry-content[^"]*"[^>]*>(.*)`)

func (s *NinetyNinePIScraper) getEpisodeList(maxPages int) ([]types.Episode, error) {
	var episodes []types.Episode
	seen := map[string]bool{}

	for page := 1; page <= maxPages; page++ {
		url := "https://99percentinvisible.org/episodes/"
		if page > 1 {
			url = fmt.Sprintf("%spage/%d/", url, page)
		}

		html, err := s.http.GetWithRetry(url, 2)
		if err != nil {
			break
		}

		for _, m := range re99piEpisodeLink.FindAllStringSubmatch(html, -1) {
			rawURL := strings.TrimRight(m[1], "/")
			// Handle both full URLs and relative paths
			fullURL := rawURL
			if !strings.HasPrefix(rawURL, "http") {
				fullURL = "https://99percentinvisible.org" + rawURL
			}
			if seen[fullURL] {
				continue
			}
			seen[fullURL] = true
			text := client.StripHTML(strings.TrimSpace(m[2]))
			text = strings.Join(strings.Fields(text), " ")
			if text != "" && len(text) > 10 && text != "Play" && text != "Download" && text != "Transcript" && text != "Share" {
				episodes = append(episodes, types.Episode{
					Title: text,
					URL:   fullURL,
				})
			}
		}
	}

	return episodes, nil
}

func (s *NinetyNinePIScraper) getTranscript(epURL string) string {
	tURL := strings.TrimRight(epURL, "/") + "/transcript"
	html, err := s.http.GetWithRetry(tURL, 2)
	if err != nil {
		return ""
	}

	if m := re99piEntryContent.FindStringSubmatch(html); m != nil {
		text := m[1]
		if idx := strings.Index(text, "<footer"); idx >= 0 {
			text = text[:idx]
		}
		if idx := strings.Index(text, "</article"); idx >= 0 {
			text = text[:idx]
		}
		return Capped(client.StripHTML(text), 10000)
	}

	if m := regexp.MustCompile(`(?s)<article[^>]*>(.*?)</article>`).FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}

	return ""
}

func (s *NinetyNinePIScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
	maxPages := max(1, maxEpisodes/10)
	episodes, err := s.getEpisodeList(maxPages)
	if err != nil {
		return nil, err
	}

	var results []types.SearchResult
	for _, ep := range episodes {
		if len(results) >= maxEpisodes {
			break
		}
		score, matched := ScoreTitleExcerpt(ep.Title, ep.Excerpt, keywords)
		if score <= 0 {
			continue
		}
		excerpt := ep.Excerpt
		if score >= 1 {
			transcript := s.getTranscript(ep.URL)
			if transcript != "" {
				score = RescoreContent(score, transcript, keywords)
				if snippet := ExtractRelevantSnippet(transcript, keywords); snippet != "" {
					excerpt = snippet
				}
			}
		}
		results = append(results, types.SearchResult{
			Title:           ep.Title,
			URL:             ep.URL,
			Date:            ep.Date,
			Excerpt:         Capped(excerpt, 300),
			Score:           float64(int(score*10)) / 10,
			MatchedKeywords: matched,
			Source:          "99pi",
		})
	}
	sortResults(results)
	return results, nil
}

func (s *NinetyNinePIScraper) Sync(maxPages int) ([]types.Episode, error) {
	return s.getEpisodeList(maxPages)
}
