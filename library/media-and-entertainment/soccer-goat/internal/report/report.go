package report

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/eafc"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/espn"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/source/potential"
)

const potentialUnavailable = "unavailable: Cloudflare (set SOCCER_GOAT_FIFACM_COOKIE for potential)"

type SourceStatus struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type PlayerReport struct {
	Query            string                  `json:"query"`
	Name             string                  `json:"name"`
	TMPlayerID       string                  `json:"tmPlayerId"`
	MarketValue      int64                   `json:"marketValue"`
	MarketValueLabel string                  `json:"marketValueLabel"`
	Club             string                  `json:"club"`
	Position         string                  `json:"position"`
	Foot             string                  `json:"foot"`
	Age              int                     `json:"age"`
	Nationality      string                  `json:"nationality"`
	EAOverall        int                     `json:"eaOverall"`
	Potential        int                     `json:"potential"`
	PotentialSource  string                  `json:"potentialSource"`
	Pace             int                     `json:"pace"`
	Shooting         int                     `json:"shooting"`
	Passing          int                     `json:"passing"`
	Dribbling        int                     `json:"dribbling"`
	Defending        int                     `json:"defending"`
	Physical         int                     `json:"physical"`
	Stats            map[string]int          `json:"stats"`
	EASlug           int                     `json:"eaSlug"`
	ESPN             *espn.Context           `json:"espn"`
	Sources          map[string]SourceStatus `json:"sources"`
}

type TeamReport struct {
	ClubName        string                  `json:"clubName"`
	TMClubID        string                  `json:"tmClubId"`
	SquadValue      int64                   `json:"squadValue"`
	SquadValueLabel string                  `json:"squadValueLabel"`
	Players         []PlayerReport          `json:"players"`
	Sources         map[string]SourceStatus `json:"sources"`
}

// FormatEuros renders a Transfermarkt euro value without locale dependence.
func FormatEuros(value int64) string {
	if value <= 0 {
		return "€0"
	}
	if value >= 1_000_000 {
		return fmt.Sprintf("€%.2fm", float64(value)/1_000_000)
	}
	if value >= 1_000 {
		if value%1_000 == 0 {
			return fmt.Sprintf("€%dk", value/1_000)
		}
		formatted := strconv.FormatFloat(float64(value)/1_000, 'f', 1, 64)
		return "€" + formatted + "k"
	}
	return fmt.Sprintf("€%d", value)
}

type Aggregator struct {
	TM   *client.Client
	EA   *eafc.Client
	Pot  *potential.Client
	ESPN *espn.Client
}

func NewAggregator(tm *client.Client) *Aggregator {
	return &Aggregator{
		TM:   tm,
		EA:   eafc.New(),
		Pot:  potential.New(),
		ESPN: espn.New(),
	}
}

func (a *Aggregator) ResolvePlayer(ctx context.Context, name string) (*PlayerReport, error) {
	if a == nil || a.TM == nil {
		return nil, fmt.Errorf("transfermarkt client is required")
	}
	path := "/players/search/" + url.PathEscape(name)
	raw, err := a.TM.Get(ctx, path, map[string]string{"page_number": "1"})
	if err != nil {
		return nil, fmt.Errorf("transfermarkt player search %q: %w", name, err)
	}
	var response tmPlayerSearchResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("transfermarkt player search %q: decode response: %w", name, err)
	}
	if len(response.Results) == 0 {
		return nil, fmt.Errorf("player not found: %s", name)
	}

	player := response.Results[0]
	report := newPlayerReport(name, string(player.ID), player.Name, player.Club.Name, player.Position, "", int(player.Age), firstName(player.Nationalities), int64(player.MarketValue))

	a.enrichEAAndPotential(ctx, report)
	if a.ESPN == nil {
		report.Sources["espn"] = SourceStatus{Detail: "unavailable: ESPN client not configured"}
	} else if context, ok, lookupErr := a.ESPN.Lookup(ctx, name); lookupErr != nil {
		report.Sources["espn"] = SourceStatus{Detail: "unavailable: " + lookupErr.Error()}
	} else if ok {
		report.ESPN = &context
		report.Sources["espn"] = SourceStatus{OK: true}
	} else {
		report.Sources["espn"] = SourceStatus{Detail: "unavailable: no ESPN athlete result"}
	}
	return report, nil
}

func (a *Aggregator) ResolveTeam(ctx context.Context, clubName string) (*TeamReport, error) {
	if a == nil || a.TM == nil {
		return nil, fmt.Errorf("transfermarkt client is required")
	}
	searchPath := "/clubs/search/" + url.PathEscape(clubName)
	raw, err := a.TM.Get(ctx, searchPath, map[string]string{"page_number": "1"})
	if err != nil {
		return nil, fmt.Errorf("transfermarkt club search %q: %w", clubName, err)
	}
	var search tmClubSearchResponse
	if err := json.Unmarshal(raw, &search); err != nil {
		return nil, fmt.Errorf("transfermarkt club search %q: decode response: %w", clubName, err)
	}
	if len(search.Results) == 0 {
		return nil, fmt.Errorf("club not found: %s", clubName)
	}

	club := search.Results[0]
	rosterPath := "/clubs/" + url.PathEscape(string(club.ID)) + "/players"
	raw, err = a.TM.Get(ctx, rosterPath, nil)
	if err != nil {
		return nil, fmt.Errorf("transfermarkt club players %q: %w", club.Name, err)
	}
	roster, err := decodeRoster(raw)
	if err != nil {
		return nil, fmt.Errorf("transfermarkt club players %q: decode response: %w", club.Name, err)
	}

	team := &TeamReport{
		ClubName: club.Name,
		TMClubID: string(club.ID),
		Players:  make([]PlayerReport, 0, len(roster)),
		Sources:  make(map[string]SourceStatus, 4),
	}
	team.Sources["transfermarkt"] = SourceStatus{OK: true}
	team.Sources["espn"] = SourceStatus{Detail: "not requested for team report"}
	for _, rosterPlayer := range roster {
		value := int64(rosterPlayer.MarketValue)
		team.SquadValue += value
		player := newPlayerReport(
			rosterPlayer.Name,
			string(rosterPlayer.ID),
			rosterPlayer.Name,
			club.Name,
			rosterPlayer.Position,
			rosterPlayer.Foot,
			int(rosterPlayer.Age),
			firstName(rosterPlayer.Nationality),
			value,
		)
		player.Sources["espn"] = SourceStatus{Detail: "not requested for team report"}
		team.Players = append(team.Players, *player)
	}
	team.SquadValueLabel = FormatEuros(team.SquadValue)

	enrichCount := len(team.Players)
	if cliutil.IsDogfoodEnv() && enrichCount > 5 {
		enrichCount = 5
		for index := enrichCount; index < len(team.Players); index++ {
			team.Players[index].Sources["ea-fc"] = SourceStatus{Detail: "skipped: dogfood enrichment limit"}
			team.Players[index].Sources["potential"] = SourceStatus{Detail: "skipped: dogfood enrichment limit"}
		}
	}
	a.enrichTeamPlayers(ctx, team.Players[:enrichCount])

	eaOK, potentialOK := 0, 0
	for index := 0; index < enrichCount; index++ {
		if team.Players[index].Sources["ea-fc"].OK {
			eaOK++
		}
		if team.Players[index].Sources["potential"].OK {
			potentialOK++
		}
	}
	team.Sources["ea-fc"] = aggregateStatus(eaOK, enrichCount, len(team.Players), cliutil.IsDogfoodEnv())
	team.Sources["potential"] = aggregateStatus(potentialOK, enrichCount, len(team.Players), cliutil.IsDogfoodEnv())
	if potentialOK == 0 {
		status := team.Sources["potential"]
		status.Detail = potentialUnavailable
		team.Sources["potential"] = status
	}
	return team, nil
}

func (a *Aggregator) enrichTeamPlayers(ctx context.Context, players []PlayerReport) {
	if len(players) == 0 {
		return
	}
	workers := len(players)
	if workers > 6 {
		workers = 6
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	group.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer group.Done()
			for index := range jobs {
				a.enrichEAAndPotential(ctx, &players[index])
			}
		}()
	}
	for index := range players {
		jobs <- index
	}
	close(jobs)
	group.Wait()
}

func (a *Aggregator) enrichEAAndPotential(ctx context.Context, report *PlayerReport) {
	if a.EA == nil {
		report.Sources["ea-fc"] = SourceStatus{Detail: "unavailable: EA FC client not configured"}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	player, ok, err := a.EA.Best(ctx, report.Name)
	if err != nil {
		report.Sources["ea-fc"] = SourceStatus{Detail: "unavailable: " + err.Error()}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	if !ok {
		report.Sources["ea-fc"] = SourceStatus{Detail: "unavailable: no EA FC player match"}
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	report.EAOverall = player.Overall
	report.Pace = player.Pace
	report.Shooting = player.Shooting
	report.Passing = player.Passing
	report.Dribbling = player.Dribbling
	report.Defending = player.Defending
	report.Physical = player.Physical
	report.Stats = cloneStats(player.Stats)
	report.EASlug = player.ID
	report.Sources["ea-fc"] = SourceStatus{OK: true}

	if a.Pot == nil || report.EASlug <= 0 {
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	rating, ratingOK, _ := a.Pot.ByEAID(ctx, report.EASlug)
	if !ratingOK {
		report.Sources["potential"] = SourceStatus{Detail: potentialUnavailable}
		return
	}
	report.Potential = rating.Potential
	report.PotentialSource = rating.Source
	report.Sources["potential"] = SourceStatus{OK: true}
}

func newPlayerReport(query, id, name, club, position, foot string, age int, nationality string, value int64) *PlayerReport {
	return &PlayerReport{
		Query:            query,
		Name:             name,
		TMPlayerID:       id,
		MarketValue:      value,
		MarketValueLabel: FormatEuros(value),
		Club:             club,
		Position:         position,
		Foot:             foot,
		Age:              age,
		Nationality:      nationality,
		Stats:            make(map[string]int),
		Sources: map[string]SourceStatus{
			"transfermarkt": {OK: true},
			"ea-fc":         {Detail: "unavailable: not attempted"},
			"potential":     {Detail: potentialUnavailable},
			"espn":          {Detail: "unavailable: not attempted"},
		},
	}
}

func cloneStats(stats map[string]int) map[string]int {
	cloned := make(map[string]int, len(stats))
	for key, value := range stats {
		cloned[key] = value
	}
	return cloned
}

func aggregateStatus(successes, attempted, total int, dogfood bool) SourceStatus {
	detail := fmt.Sprintf("enriched %d/%d players", successes, attempted)
	if dogfood && attempted < total {
		detail += fmt.Sprintf("; dogfood capped enrichment at %d/%d", attempted, total)
	}
	return SourceStatus{OK: successes > 0, Detail: detail}
}

type flexibleString string
type flexibleInt int
type marketValue int64
type nameList []string

func (value *flexibleString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*value = ""
		return nil
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		*value = flexibleString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*value = flexibleString(number.String())
	return nil
}

func (value *flexibleInt) UnmarshalJSON(data []byte) error {
	parsed, err := parseInt64(data)
	*value = flexibleInt(parsed)
	return err
}

func (value *marketValue) UnmarshalJSON(data []byte) error {
	parsed, err := parseMarketValue(data)
	*value = marketValue(parsed)
	return err
}

func (names *nameList) UnmarshalJSON(data []byte) error {
	var stringsList []string
	if err := json.Unmarshal(data, &stringsList); err == nil {
		*names = nameList(stringsList)
		return nil
	}
	var objects []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &objects); err != nil {
		return err
	}
	result := make([]string, 0, len(objects))
	for _, object := range objects {
		if object.Name != "" {
			result = append(result, object.Name)
		}
	}
	*names = nameList(result)
	return nil
}

func firstName(names nameList) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func parseInt64(data []byte) (int64, error) {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" {
		return 0, nil
	}
	if integer, err := strconv.ParseInt(text, 10, 64); err == nil {
		return integer, nil
	}
	float, err := strconv.ParseFloat(text, 64)
	return int64(float), err
}

func parseMarketValue(data []byte) (int64, error) {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if text == "" || text == "null" || text == "-" {
		return 0, nil
	}
	if value, err := strconv.ParseInt(text, 10, 64); err == nil {
		return value, nil
	}
	normalized := strings.ToLower(strings.NewReplacer("€", "", ",", "", " ", "").Replace(text))
	multiplier := float64(1)
	switch {
	case strings.HasSuffix(normalized, "bn"):
		multiplier = 1_000_000_000
		normalized = strings.TrimSuffix(normalized, "bn")
	case strings.HasSuffix(normalized, "b"):
		multiplier = 1_000_000_000
		normalized = strings.TrimSuffix(normalized, "b")
	case strings.HasSuffix(normalized, "m"):
		multiplier = 1_000_000
		normalized = strings.TrimSuffix(normalized, "m")
	case strings.HasSuffix(normalized, "k"):
		multiplier = 1_000
		normalized = strings.TrimSuffix(normalized, "k")
	}
	number, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, err
	}
	return int64(number * multiplier), nil
}

type tmPlayerSearchResponse struct {
	Results []tmSearchPlayer `json:"results"`
}

type tmSearchPlayer struct {
	ID   flexibleString `json:"id"`
	Name string         `json:"name"`
	Club struct {
		Name string `json:"name"`
	} `json:"club"`
	Position      string      `json:"position"`
	Age           flexibleInt `json:"age"`
	Nationalities nameList    `json:"nationalities"`
	MarketValue   marketValue `json:"marketValue"`
}

type tmClubSearchResponse struct {
	Results []tmClubResult `json:"results"`
}

type tmClubResult struct {
	ID   flexibleString `json:"id"`
	Name string         `json:"name"`
}

type tmRosterPlayer struct {
	ID          flexibleString `json:"id"`
	Name        string         `json:"name"`
	Position    string         `json:"position"`
	Age         flexibleInt    `json:"age"`
	Nationality nameList       `json:"nationality"`
	Foot        string         `json:"foot"`
	MarketValue marketValue    `json:"marketValue"`
}

func decodeRoster(data []byte) ([]tmRosterPlayer, error) {
	players := make([]tmRosterPlayer, 0)
	if err := json.Unmarshal(data, &players); err == nil {
		if players == nil {
			players = make([]tmRosterPlayer, 0)
		}
		return players, nil
	}
	var wrapped struct {
		Players []tmRosterPlayer `json:"players"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return players, err
	}
	if wrapped.Players == nil {
		wrapped.Players = make([]tmRosterPlayer, 0)
	}
	return wrapped.Players, nil
}
