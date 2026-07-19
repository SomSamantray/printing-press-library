package scrapers

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type TwentyKHzScraper struct{ http *client.HTTPClient }

func NewTwentyKHzScraper() *TwentyKHzScraper {
	return &TwentyKHzScraper{http: client.New()}
}

func (s *TwentyKHzScraper) Source() string { return "twentykhz" }
func (s *TwentyKHzScraper) Name() string   { return "Twenty Thousand Hertz" }

var re20kEpisodeLink = regexp.MustCompile(`(?s)<a[^>]*href="(/episodes/[\w-]+)"[^>]*data-title="([^"]*)"[^>]*>`)
var re20kTranscriptLink = regexp.MustCompile(`<a[^>]*href="([^"]*transcript[^"]*)"[^>]*>`)
var re20kTranscriptDiv = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*transcript[^"]*"[^>]*>(.*?)</div>`)
var re20kArticle = regexp.MustCompile(`(?s)<article[^>]*>(.*?)</article>`)

func (s *TwentyKHzScraper) getEpisodeList() ([]types.Episode, error) {
	html, err := s.http.GetWithRetry("https://www.20k.org/episodes-archive", 2)
	if err != nil {
		return nil, err
	}

	var episodes []types.Episode
	var jsonPatterns = []string{
		`"episodes"\s*:\s*(\[.*?\])`,
	}

	for _, pat := range jsonPatterns {
		re := regexp.MustCompile(pat)
		if m := re.FindStringSubmatch(html); m != nil {
			// Try to find JSON structure — skip length check for inline
			break
		}
	}

	seen := map[string]bool{}
	for _, m := range re20kEpisodeLink.FindAllStringSubmatch(html, -1) {
		fullURL := "https://www.20k.org" + m[1]
		if seen[fullURL] || m[1] == "/episodes" || m[1] == "/episodes-archive" || m[1] == "/episodes/" {
			continue
		}
		seen[fullURL] = true
		title := m[2] // data-title attribute
		title = strings.ReplaceAll(title, "&amp;", "&")
		title = strings.ReplaceAll(title, "&quot;", "\"")
		if title != "" {
			episodes = append(episodes, types.Episode{
				Title: title,
				URL:   fullURL,
			})
		}
	}

	return episodes, nil
}

func (s *TwentyKHzScraper) getTranscript(epURL string) string {
	html, err := s.http.GetWithRetry(epURL, 2)
	if err != nil {
		return ""
	}

	if m := re20kTranscriptLink.FindStringSubmatch(html); m != nil {
		tURL := m[1]
		if !strings.HasPrefix(tURL, "http") {
			tURL = "https://www.20k.org" + tURL
		}
		if tHTML, err := s.http.GetWithRetry(tURL, 2); err == nil {
			return Capped(client.StripHTML(tHTML), 10000)
		}
	}

	if m := re20kTranscriptDiv.FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}

	if m := re20kArticle.FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}

	return ""
}

func (s *TwentyKHzScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
	episodes, err := s.getEpisodeList()
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
			Source:          "twentykhz",
		})
	}
	sortResults(results)
	return results, nil
}

func (s *TwentyKHzScraper) Sync(_ int) ([]types.Episode, error) {
	return s.getEpisodeList()
}
