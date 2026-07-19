package scrapers

import (
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type SatPostScraper struct{ http *client.HTTPClient }

func NewSatPostScraper() *SatPostScraper {
	return &SatPostScraper{http: client.New()}
}

func (s *SatPostScraper) Source() string { return "satpost" }
func (s *SatPostScraper) Name() string   { return "SatPost Blog" }

var reSatPostLink = regexp.MustCompile(`(?s)<a[^>]*href="(https://www\.readtrung\.com/p/[\w-]+)[^"]*"[^>]*>(.*?)</a>`)
var reSatPostArticle = regexp.MustCompile(`(?s)<article[^>]*>(.*?)</article>`)
var reSatPostBody = regexp.MustCompile(`(?s)<div[^>]*class="[^"]*(?:body|content|post-content)[^"]*"[^>]*>(.*?)</div>`)

func (s *SatPostScraper) getPostList() ([]types.Episode, error) {
	var posts []types.Episode
	seen := map[string]bool{}

	for page := 1; page <= 7; page++ {
		url := "https://www.readtrung.com/archive"
		if page > 1 {
			url = "https://www.readtrung.com/archive?page=" + string(rune('0'+page/10)) + string(rune('0'+page%10))
		}

		html, err := s.http.GetWithRetry(url, 2)
		if err != nil {
			break
		}

		newPosts := 0
		for _, m := range reSatPostLink.FindAllStringSubmatch(html, -1) {
			foundURL := m[1]
			if seen[foundURL] {
				continue
			}
			seen[foundURL] = true
			title := client.StripHTML(strings.TrimSpace(m[2]))

			skipWords := []string{"Share", "Like", "Comment", "Subscribe", "Sign in", "SatPost by Trung Phan"}
			isSkip := false
			for _, sw := range skipWords {
				if title == sw {
					isSkip = true
					break
				}
			}
			if isSkip || len(title) < 5 {
				slug := strings.Replace(foundURL, "https://www.readtrung.com/p/", "", 1)
				slug = strings.ReplaceAll(slug, "-", " ")
				slug = strings.ToLower(slug)
				if len(slug) > 0 {
					slug = strings.ToUpper(slug[:1]) + slug[1:]
				}
				title = slug
			}

			posts = append(posts, types.Episode{
				Title: title,
				URL:   foundURL,
			})
			newPosts++
		}
		if newPosts == 0 {
			break
		}
	}

	return posts, nil
}

func (s *SatPostScraper) getArticleContent(url string) string {
	html, err := s.http.GetWithRetry(url, 2)
	if err != nil {
		return ""
	}

	if m := reSatPostArticle.FindStringSubmatch(html); m != nil {
		text := m[1]
		text = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`).ReplaceAllString(text, " ")
		text = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`).ReplaceAllString(text, " ")
		return Capped(client.StripHTML(text), 10000)
	}

	if m := reSatPostBody.FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}

	return ""
}

func (s *SatPostScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
	posts, err := s.getPostList()
	if err != nil {
		return nil, err
	}

	var results []types.SearchResult
	for _, post := range posts {
		if len(results) >= maxEpisodes {
			break
		}
		score, matched := ScoreTitleExcerpt(post.Title, post.Excerpt, keywords)
		if score <= 0 {
			continue
		}
		excerpt := post.Excerpt
		if score >= 1 {
			content := s.getArticleContent(post.URL)
			if content != "" {
				score = RescoreContent(score, content, keywords)
				if snippet := ExtractRelevantSnippet(content, keywords); snippet != "" {
					excerpt = snippet
				}
			}
		}
		results = append(results, types.SearchResult{
			Title:           post.Title,
			URL:             post.URL,
			Date:            post.Date,
			Excerpt:         Capped(excerpt, 300),
			Score:           float64(int(score*10)) / 10,
			MatchedKeywords: matched,
			Source:          "satpost",
		})
	}
	sortResults(results)
	return results, nil
}

func (s *SatPostScraper) Sync(_ int) ([]types.Episode, error) {
	return s.getPostList()
}
