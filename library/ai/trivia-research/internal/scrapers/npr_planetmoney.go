package scrapers

import (
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type NPRPlanetMoneyScraper struct{ http *client.HTTPClient }

func NewNPRPlanetMoneyScraper() *NPRPlanetMoneyScraper {
	return &NPRPlanetMoneyScraper{http: client.New()}
}

func (s *NPRPlanetMoneyScraper) Source() string { return "planetmoney" }
func (s *NPRPlanetMoneyScraper) Name() string   { return "Planet Money" }

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

var reNPRTranscript = regexp.MustCompile(`(?s)<div[^>]*(?:id|class)="[^"]*transcript[^"]*"[^>]*>(.*?)</div>`)
var reNPRStoryText = regexp.MustCompile(`(?s)<div[^>]*id="storytext"[^>]*>(.*)`)

func (s *NPRPlanetMoneyScraper) getEpisodesFromRSS(feedURL string) ([]types.Episode, error) {
	body, err := s.http.GetWithRetry(feedURL, 2)
	if err != nil {
		return nil, err
	}

	var feed rssFeed
	if err := xml.NewDecoder(strings.NewReader(body)).Decode(&feed); err != nil {
		return nil, err
	}

	var episodes []types.Episode
	for _, item := range feed.Channel.Items {
		if item.Title == "" || item.Link == "" {
			continue
		}
		desc := client.StripHTML(item.Description)
		desc = Capped(desc, 300)

		storyID := ""
		if m := regexp.MustCompile(`/(nx-s1-\d+|[\w-]+)$`).FindStringSubmatch(strings.TrimRight(item.Link, "/")); m != nil {
			storyID = m[1]
		}

		episodes = append(episodes, types.Episode{
			Title:   item.Title,
			URL:     item.Link,
			Date:    item.PubDate,
			Excerpt: desc,
			StoryID: storyID,
		})
	}

	return episodes, nil
}

func (s *NPRPlanetMoneyScraper) getTranscript(storyID string) string {
	if storyID == "" {
		return ""
	}

	url := "https://www.npr.org/transcripts/" + storyID
	html, err := s.http.GetWithRetry(url, 2)
	if err != nil {
		return ""
	}

	if m := reNPRTranscript.FindStringSubmatch(html); m != nil {
		return Capped(client.StripHTML(m[1]), 10000)
	}
	if m := reNPRStoryText.FindStringSubmatch(html); m != nil {
		text := m[1]
		if idx := strings.Index(text, `<div`); idx >= 0 && strings.Contains(text[idx:], "tags") {
			text = text[:idx]
		}
		if idx := strings.Index(text, "</article"); idx >= 0 {
			text = text[:idx]
		}
		return Capped(client.StripHTML(text), 10000)
	}

	return ""
}

func (s *NPRPlanetMoneyScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
	episodes, err := s.getEpisodesFromRSS("https://feeds.npr.org/510289/podcast.xml")
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
		if score >= 1 && ep.StoryID != "" {
			transcript := s.getTranscript(ep.StoryID)
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
			Source:          "planetmoney",
		})
	}
	sortResults(results)
	return results, nil
}

func (s *NPRPlanetMoneyScraper) Sync(_ int) ([]types.Episode, error) {
	return s.getEpisodesFromRSS("https://feeds.npr.org/510289/podcast.xml")
}
