package scrapers

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type FreakonomicsScraper struct{ http *client.HTTPClient }

func NewFreakonomicsScraper() *FreakonomicsScraper {
	return &FreakonomicsScraper{http: client.New()}
}

func (s *FreakonomicsScraper) Source() string { return "freakonomics" }
func (s *FreakonomicsScraper) Name() string   { return "Freakonomics Radio" }

var reFreakoLink = regexp.MustCompile(`<a[^>]*href="(https://freakonomics\.com/podcast/[^"]+)"[^>]*>(.*?)</a>`)
var reFreakoTranscript = regexp.MustCompile(`(?s)<h[23][^>]*>\s*Episode\s+Transcript\s*</h[23]>\s*(.*)`)

func (s *FreakonomicsScraper) getEpisodeList() ([]types.Episode, error) {
	html, err := s.http.GetWithRetry("https://freakonomics.com/series-full/freakonomics-radio/", 2)
	if err != nil {
		html, err = s.http.GetWithRetry("https://freakonomics.com/series/freakonomics-radio/", 2)
		if err != nil {
			return nil, err
		}
	}

	linkDedup := map[string]string{}
	for _, m := range reFreakoLink.FindAllStringSubmatch(html, -1) {
		href := m[1]
		rawText := strings.TrimSpace(client.StripHTML(m[2]))
		if existing, ok := linkDedup[href]; !ok || len(rawText) > len(existing) {
			linkDedup[href] = rawText
		}
	}

	var episodes []types.Episode
	for href, rawText := range linkDedup {
		if href == "" {
			continue
		}
		slug := href
		idx := strings.LastIndex(slug, "/")
		if idx >= 0 {
			slug = slug[:idx]
			idx2 := strings.LastIndex(slug, "/")
			if idx2 >= 0 {
				slug = slug[idx2+1:]
			}
		}
		slugTitle := strings.ReplaceAll(slug, "-", " ")
		slugTitle = strings.Title(strings.ToLower(slugTitle))

		isLabel := regexp.MustCompile(`^(NO\.\s*\d+|EXTRA|PLUS|BONUS)$`).MatchString(rawText)

		title := rawText
		if isLabel || len(rawText) < 5 {
			title = slugTitle
			if numMatch := regexp.MustCompile(`NO\.\s*(\d+)`).FindStringSubmatch(rawText); numMatch != nil {
				title = "#" + numMatch[1] + ": " + slugTitle
			}
		}

		episodes = append(episodes, types.Episode{
			Title: title,
			URL:   href,
		})
	}

	return episodes, nil
}

func (s *FreakonomicsScraper) getTranscript(epURL string) string {
	html, err := s.http.GetWithRetry(epURL, 2)
	if err != nil {
		return ""
	}

	if m := reFreakoTranscript.FindStringSubmatch(html); m != nil {
		text := m[1]
		for _, cut := range []string{"<h2", "<h3", "<footer", "<!--"} {
			if idx := strings.Index(text, cut); idx >= 0 {
				text = text[:idx]
				break
			}
		}
		return Capped(client.StripHTML(text), 10000)
	}

	for _, class := range []string{"transcript", "blog-post-content", "entry-content", "podcast-content"} {
		re := regexp.MustCompile(`(?s)<div[^>]*class="[^"]*` + regexp.QuoteMeta(class) + `[^"]*"[^>]*>(.*?)</div>`)
		if m := re.FindStringSubmatch(html); m != nil {
			return Capped(client.StripHTML(m[1]), 10000)
		}
	}

	return ""
}

func (s *FreakonomicsScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
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
			Source:          "freakonomics",
		})
	}

	sortResults(results)
	return results, nil
}

func (s *FreakonomicsScraper) Sync(_ int) ([]types.Episode, error) {
	return s.getEpisodeList()
}
