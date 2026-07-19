package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/scrapers"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

func RegisterTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("trivia_research",
		mcp.WithDescription("Search 9 podcast transcript and blog archives in parallel for deep topic research. Sources: Gastropod, Freakonomics Radio, Planet Money, The Indicator, 20K Hz, 99% Invisible, SatPost, Acquired, Business Breakdowns."),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic or keywords to research")),
		mcp.WithNumber("max_episodes", mcp.Description("Max episodes to scan per source (default: 50)")),
		mcp.WithString("sources", mcp.Description("Comma-separated source list or 'all'")),
	), handleResearch)

	s.AddTool(mcp.NewTool("trivia_list_sources",
		mcp.WithDescription("List available podcast and blog sources with descriptions"),
	), handleListSources)

	s.AddTool(mcp.NewTool("trivia_doctor",
		mcp.WithDescription("Check connectivity to all 9 podcast and blog sources"),
	), handleDoctor)
}

func handleResearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := req.Params.Arguments.(map[string]interface{})
	topic, _ := args["topic"].(string)
	if topic == "" {
		return mcp.NewToolResultError("topic is required"), nil
	}

	maxEpisodes := 50
	if n, ok := args["max_episodes"].(float64); ok && n > 0 {
		maxEpisodes = int(n)
	}

	sourceFilter := "all"
	if s, ok := args["sources"].(string); ok && s != "" {
		sourceFilter = s
	}

	all := map[string]scrapers.Scraper{
		"gastropod":    scrapers.NewGastropodScraper(),
		"freakonomics": scrapers.NewFreakonomicsScraper(),
		"planetmoney":  scrapers.NewNPRPlanetMoneyScraper(),
		"indicator":    scrapers.NewNPRIndicatorScraper(),
		"twentykhz":    scrapers.NewTwentyKHzScraper(),
		"99pi":         scrapers.NewNinetyNinePIScraper(),
		"satpost":      scrapers.NewSatPostScraper(),
		"acquired":     scrapers.NewAcquiredScraper(),
		"colossus":     scrapers.NewColossusScraper(),
	}

	keywords := scrapers.NormalizeTopic(topic)

	type result struct {
		src     scrapers.Scraper
		results []types.SearchResult
		err     error
	}

	activeSources := make([]scrapers.Scraper, 0)
	if sourceFilter == "all" {
		for _, key := range types.AllSources {
			if s, ok := all[key]; ok {
				activeSources = append(activeSources, s)
			}
		}
	} else {
		for _, name := range strings.Split(sourceFilter, ",") {
			if s, ok := all[strings.TrimSpace(name)]; ok {
				activeSources = append(activeSources, s)
			}
		}
	}

	resultsChan := make(chan result, len(activeSources))
	for _, src := range activeSources {
		go func(s scrapers.Scraper) {
			res, err := s.Search(keywords, maxEpisodes)
			resultsChan <- result{src: s, results: res, err: err}
		}(src)
	}

	var sourceResults []types.SourceResult
	totalItems := 0
	for range activeSources {
		wr := <-resultsChan
		sr := types.SourceResult{Source: wr.src.Source(), Name: wr.src.Name()}
		if wr.err != nil {
			sr.Error = wr.err.Error()
		} else {
			sr.Results = wr.results
			sr.Count = len(wr.results)
			totalItems += sr.Count
		}
		sourceResults = append(sourceResults, sr)
	}

	output := types.ResearchOutput{
		Topic:        topic,
		Keywords:     keywords,
		Sources:      sourceResults,
		TotalItems:   totalItems,
		TotalSources: len(sourceResults),
	}

	return mcp.NewToolResultJSON(output)
}

func handleListSources(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	type src struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	var sources []src
	for _, key := range types.AllSources {
		t := "podcast"
		if key == "satpost" {
			t = "blog"
		}
		sources = append(sources, src{Key: key, Name: types.SourceNames[key], Type: t})
	}
	return mcp.NewToolResultJSON(sources)
}

func handleDoctor(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	all := map[string]scrapers.Scraper{
		"gastropod":    scrapers.NewGastropodScraper(),
		"freakonomics": scrapers.NewFreakonomicsScraper(),
		"planetmoney":  scrapers.NewNPRPlanetMoneyScraper(),
		"indicator":    scrapers.NewNPRIndicatorScraper(),
		"twentykhz":    scrapers.NewTwentyKHzScraper(),
		"99pi":         scrapers.NewNinetyNinePIScraper(),
		"satpost":      scrapers.NewSatPostScraper(),
		"acquired":     scrapers.NewAcquiredScraper(),
		"colossus":     scrapers.NewColossusScraper(),
	}

	report := make(map[string]any)
	allOK := true
	totalEpisodes := 0

	for _, key := range types.AllSources {
		src := all[key]
		episodes, err := src.Sync(1)
		if err != nil {
			report[key] = map[string]any{"name": src.Name(), "status": "error", "error": err.Error()}
			allOK = false
		} else {
			report[key] = map[string]any{"name": src.Name(), "status": "ok", "episodes": len(episodes)}
			totalEpisodes += len(episodes)
		}
	}

	status := "healthy"
	if !allOK {
		status = "degraded"
	}

	return mcp.NewToolResultJSON(map[string]any{
		"status":         status,
		"total_episodes": totalEpisodes,
		"sources":        report,
	})
}

func _() {
	_ = fmt.Sprintf
}
