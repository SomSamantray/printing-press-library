package scrapers

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type ColossusScraper struct{ http *client.HTTPClient }

func NewColossusScraper() *ColossusScraper {
	return &ColossusScraper{http: client.New()}
}

func (s *ColossusScraper) Source() string { return "colossus" }
func (s *ColossusScraper) Name() string   { return "Business Breakdowns" }

var reColossusJSONLD = regexp.MustCompile(`(?s)<script[^>]*type="application/(?:json|ld\+json)"[^>]*>(.*?)</script>`)
var reColossusLink = regexp.MustCompile(`(?s)<a[^>]*href="(/[^"]*(?:podcast|episode|breakdown|article)[^"]*)"[^>]*>(.*?)</a>`)
var reColossusArticle = regexp.MustCompile(`(?s)<article[^>]*>(.*?)</article>`)
var reColossusContent = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*(?:content|body|post-content|show-notes)[^"]*"[^>]*>(.*?)</div>`)
var reColossusMain = regexp.MustCompile(`(?s)<main[^>]*>(.*?)</main>`)

func (s *ColossusScraper) getContentList() ([]types.Episode, error) {
	var episodes []types.Episode

	for _, baseURL := range []string{"https://colossus.com", "https://www.joincolossus.com"} {
		html, err := s.http.GetWithRetry(baseURL, 2)
		if err != nil {
			continue
		}

		if m := reColossusJSONLD.FindStringSubmatch(html); m != nil {
			var data interface{}
			if json.Unmarshal([]byte(m[1]), &data) == nil {
				items, ok := data.([]interface{})
				if !ok {
					if dm, ok := data.(map[string]interface{}); ok {
						if a, ok := dm["@graph"]; ok {
							items, _ = a.([]interface{})
						}
					}
				}
				for _, item := range items {
					if ep, ok := item.(map[string]interface{}); ok {
						title, _ := ep["name"].(string)
						if title == "" {
							title, _ = ep["headline"].(string)
						}
						epURL, _ := ep["url"].(string)
						date, _ := ep["datePublished"].(string)
						desc, _ := ep["description"].(string)
						if title != "" && epURL != "" {
							episodes = append(episodes, types.Episode{
								Title:   title,
								URL:     epURL,
								Date:    date,
								Excerpt: Capped(desc, 300),
							})
						}
					}
				}
				if len(episodes) > 0 {
					return episodes, nil
				}
			}
		}

		seen := map[string]bool{}
		for _, m := range reColossusLink.FindAllStringSubmatch(html, -1) {
			path := m[1]
			fullURL := path
			if !strings.HasPrefix(path, "http") {
				fullURL = baseURL + path
			}
			if seen[fullURL] {
				continue
			}
			seen[fullURL] = true
			text := strings.TrimSpace(client.StripHTML(m[2]))
			if text != "" && len(text) > 5 {
				episodes = append(episodes, types.Episode{Title: text, URL: fullURL})
			}
		}
		if len(episodes) > 0 {
			break
		}
	}

	return episodes, nil
}

func (s *ColossusScraper) getPageContent(pageURL string) string {
	html, err := s.http.GetWithRetry(pageURL, 2)
	if err != nil {
		return ""
	}

	selectors := []*regexp.Regexp{reColossusArticle, reColossusContent, reColossusMain}
	for _, re := range selectors {
		if m := re.FindStringSubmatch(html); m != nil {
			text := m[1]
			text = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`).ReplaceAllString(text, " ")
			text = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`).ReplaceAllString(text, " ")
			cleaned := client.StripHTML(text)
			if len(cleaned) > 100 {
				return Capped(cleaned, 10000)
			}
		}
	}
	return ""
}

func (s *ColossusScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
	episodes, err := s.getContentList()
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
			content := s.getPageContent(ep.URL)
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
			Source:          "colossus",
		})
	}
	sortResults(results)
	return results, nil
}

func (s *ColossusScraper) Sync(_ int) ([]types.Episode, error) {
	return s.getContentList()
}
