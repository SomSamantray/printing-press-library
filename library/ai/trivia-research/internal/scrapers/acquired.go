package scrapers

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type AcquiredScraper struct{ http *client.HTTPClient }

func NewAcquiredScraper() *AcquiredScraper {
	return &AcquiredScraper{http: client.New()}
}

func (s *AcquiredScraper) Source() string { return "acquired" }
func (s *AcquiredScraper) Name() string   { return "Acquired" }

var reAcquiredNextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*type="application/json"[^>]*>(.*?)</script>`)
var reAcquiredEpLink = regexp.MustCompile(`<a[^>]*href="(/episodes/[\w-]+)"[^>]*>(.*?)</a>`)
var reAcquiredContent = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*(?:show-notes|transcript|content|body)[^"]*"[^>]*>(.*?)</div>`)
var reAcquiredMain = regexp.MustCompile(`(?s)<main[^>]*>(.*?)</main>`)

func (s *AcquiredScraper) getEpisodeList() ([]types.Episode, error) {
	html, err := s.http.GetWithRetry("https://www.acquired.fm/episodes", 2)
	if err != nil {
		return nil, err
	}

	var episodes []types.Episode

	m := reAcquiredNextData.FindStringSubmatch(html)
	if m != nil {
		var data map[string]interface{}
		if json.Unmarshal([]byte(m[1]), &data) == nil {
			props, _ := data["props"].(map[string]interface{})
			if props != nil {
				pageProps, _ := props["pageProps"].(map[string]interface{})
				if pageProps != nil {
					eps, _ := pageProps["episodes"].([]interface{})
					if eps == nil {
						eps, _ = pageProps["posts"].([]interface{})
					}
					for _, e := range eps {
						ep, ok := e.(map[string]interface{})
						if !ok {
							continue
						}
						title, _ := ep["title"].(string)
						if title == "" {
							title, _ = ep["name"].(string)
						}
						slug, _ := ep["slug"].(string)
						date, _ := ep["publishedAt"].(string)
						if date == "" {
							date, _ = ep["date"].(string)
						}
						desc, _ := ep["description"].(string)
						if desc == "" {
							desc, _ = ep["summary"].(string)
						}
						if title != "" && slug != "" {
							episodes = append(episodes, types.Episode{
								Title:   title,
								URL:     "https://www.acquired.fm/episodes/" + slug,
								Date:    date,
								Excerpt: Capped(desc, 300),
							})
						}
					}
					if len(episodes) > 0 {
						return episodes, nil
					}
				}
			}
		}
	}

	seen := map[string]bool{}
	for _, m := range reAcquiredEpLink.FindAllStringSubmatch(html, -1) {
		if m[1] == "/episodes" || m[1] == "/episodes/" {
			continue
		}
		fullURL := "https://www.acquired.fm" + m[1]
		if seen[fullURL] {
			continue
		}
		seen[fullURL] = true
		text := strings.TrimSpace(client.StripHTML(m[2]))
		if text != "" {
			episodes = append(episodes, types.Episode{Title: text, URL: fullURL})
		}
	}

	return episodes, nil
}

func (s *AcquiredScraper) getContent(epURL string) string {
	html, err := s.http.GetWithRetry(epURL, 2)
	if err != nil {
		return ""
	}

	if m := reAcquiredContent.FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}
	if m := reAcquiredMain.FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}
	return ""
}

func (s *AcquiredScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
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
			content := s.getContent(ep.URL)
			if content != "" {
				score = RescoreContent(score, content, keywords)
				if snippet := ExtractRelevantSnippet(content, keywords); snippet != "" {
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
			Source:          "acquired",
		})
	}
	sortResults(results)
	return results, nil
}

func (s *AcquiredScraper) Sync(_ int) ([]types.Episode, error) {
	return s.getEpisodeList()
}
