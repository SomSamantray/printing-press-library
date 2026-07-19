package scrapers

import (
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

type NPRIndicatorScraper struct{ http *client.HTTPClient }

func NewNPRIndicatorScraper() *NPRIndicatorScraper {
	return &NPRIndicatorScraper{http: client.New()}
}

func (s *NPRIndicatorScraper) Source() string { return "indicator" }
func (s *NPRIndicatorScraper) Name() string   { return "The Indicator" }

func (s *NPRIndicatorScraper) Search(keywords []string, maxEpisodes int) ([]types.SearchResult, error) {
	pm := &NPRPlanetMoneyScraper{http: s.http}
	episodes, err := pm.getEpisodesFromRSS("https://feeds.npr.org/510325/podcast.xml")
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
			transcript := pm.getTranscript(ep.StoryID)
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
			Source:          "indicator",
		})
	}

	sortResults(results)
	return results, nil
}

func (s *NPRIndicatorScraper) Sync(_ int) ([]types.Episode, error) {
	pm := &NPRPlanetMoneyScraper{http: s.http}
	return pm.getEpisodesFromRSS("https://feeds.npr.org/510325/podcast.xml")
}
