package scrapers

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type GastropodScraper struct{ http *client.HTTPClient }

func NewGastropodScraper() *GastropodScraper {
	return &GastropodScraper{http: client.New()}
}

func (s *GastropodScraper) Source() string { return "gastropod" }
func (s *GastropodScraper) Name() string   { return "Gastropod" }

var (
	reGPodArticle = regexp.MustCompile(`(?s)<article[^>]*>(.*?)</article>`)
	reGPodTitle   = regexp.MustCompile(`(?s)<(?:h[12])[^>]*class="[^"]*entry-title[^"]*"[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	reGPodExcerpt = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*(?:entry-content|entry-summary|post-excerpt)[^"]*"[^>]*>(.*?)</div>`)
	reGPodDate    = regexp.MustCompile(`(?s)<(?:time|span)[^>]*class="[^"]*(?:entry-date|entry-pubdate|date)[^"]*"[^>]*>(.*?)</(?:time|span)>`)
	reGPodContent = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*entry-content[^"]*"[^>]*>(.*)`)
)

func (s *GastropodScraper) getEpisodeList(maxPages int) ([]types.Episode, error) {
	var episodes []types.Episode
	seen := map[string]bool{}

	for page := 1; page <= maxPages; page++ {
		epURL := "https://gastropod.com/category/podcasts/"
		if page > 1 {
			epURL = fmt.Sprintf("%spage/%d/", epURL, page)
		}

		html, err := s.http.GetWithRetry(epURL, 2)
		if err != nil {
			break
		}

		articles := reGPodArticle.FindAllStringSubmatch(html, -1)
		for _, articleMatch := range articles {
			articleHTML := articleMatch[1]
			titleMatch := reGPodTitle.FindStringSubmatch(articleHTML)
			if titleMatch == nil {
				continue
			}

			href := titleMatch[1]
			if seen[href] || strings.Contains(href, "/category/") || strings.Contains(href, "/page/") {
				continue
			}
			seen[href] = true

			title := client.StripHTML(strings.TrimSpace(titleMatch[2]))
			title = strings.ReplaceAll(title, "&#8217;", "'")
			title = strings.ReplaceAll(title, "&amp;", "&")
			title = strings.ReplaceAll(title, "&#038;", "&")

			excerpt := ""
			if excMatch := reGPodExcerpt.FindStringSubmatch(articleHTML); excMatch != nil {
				excerpt = Capped(client.StripHTML(excMatch[1]), 300)
			}

			date := ""
			if dateMatch := reGPodDate.FindStringSubmatch(articleHTML); dateMatch != nil {
				date = client.StripHTML(strings.TrimSpace(dateMatch[1]))
			}

			episodes = append(episodes, types.Episode{
				Title:   title,
				URL:     strings.TrimRight(href, "/"),
				Date:    date,
				Excerpt: excerpt,
			})
		}
	}

	return episodes, nil
}

func (s *GastropodScraper) getTranscript(epURL string) string {
	parsed, err := url.Parse(epURL)
	if err != nil {
		return ""
	}
	slug := parsed.Path
	slug = strings.Trim(slug, "/")
	idx := strings.LastIndex(slug, "/")
	if idx >= 0 {
		slug = slug[idx+1:]
	}

	transcriptURL := fmt.Sprintf("https://gastropod.com/transcript-%s/", slug)
	if strings.Contains(epURL, "transcript-") {
		transcriptURL = epURL
	}

	html, err := s.http.GetWithRetry(transcriptURL, 2)
	if err != nil {
		return ""
	}

	if m := reGPodContent.FindStringSubmatch(html); m != nil {
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

func (s *GastropodScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
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
		if score >= 2 {
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
			Source:          "gastropod",
		})
	}

	sortResults(results)
	return results, nil
}

func (s *GastropodScraper) Sync(maxPages int) ([]types.Episode, error) {
	return s.getEpisodeList(maxPages)
}

func sortResults(results []types.SearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
