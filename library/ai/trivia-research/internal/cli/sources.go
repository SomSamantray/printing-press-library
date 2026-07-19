package cli

import (
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/scrapers"
	"github.com/mvanhorn/printing-press-library/library/ai/trivia-research/internal/types"
)

func classifyError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "not found") || strings.Contains(msg, "404") {
		return notFoundErr(err)
	}
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "401") || strings.Contains(msg, "403") {
		return authErr(err)
	}
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "429") {
		return rateLimitErr(err)
	}
	return apiErr(err)
}

func getActiveSources(sourceFilter string) []scrapers.Scraper {
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
	if sourceFilter == "all" {
		var active []scrapers.Scraper
		for _, src := range types.AllSources {
			if s, ok := all[src]; ok {
				active = append(active, s)
			}
		}
		return active
	}
	var active []scrapers.Scraper
	for _, name := range splitComma(sourceFilter) {
		if s, ok := all[name]; ok {
			active = append(active, s)
		}
	}
	return active
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func countActive(sources []types.SourceResult) int {
	count := 0
	for _, s := range sources {
		if s.Count > 0 && s.Error == "" {
			count++
		}
	}
	return count
}

func _() { _ = os.Args }
