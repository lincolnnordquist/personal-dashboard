package widgets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	SportsWidgetType = "sports"
	SportsTTL        = 15 * time.Minute
)

// sportPaths maps a sport name to its ESPN API path. Add a league here to support it.
var sportPaths = map[string]string{
	"nfl": "football/nfl",
}

// SportPath returns the ESPN API path for a sport, e.g. "nfl" -> "football/nfl".
func SportPath(sport string) (string, error) {
	path, ok := sportPaths[strings.ToLower(sport)]
	if !ok {
		return "", fmt.Errorf("unsupported sport %q", sport)
	}
	return path, nil
}

var teamIDPattern = regexp.MustCompile(`^[a-z0-9]{1,5}$`)

// NormalizeTeamID lowercases a team abbreviation or ESPN team id and validates it.
func NormalizeTeamID(id string) (string, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !teamIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid team id %q", id)
	}
	return id, nil
}

// Cache keys, e.g. "nfl_scoreboard", "nfl_team_sea", "nfl_standings".
func ScoreboardCacheKey(sport string) string           { return sport + "_scoreboard" }
func TeamScheduleCacheKey(sport, teamID string) string { return sport + "_team_" + teamID }
func StandingsCacheKey(sport string) string            { return sport + "_standings" }

// SportsConfig is the widget_config.config shape for a sports widget.
type SportsConfig struct {
	Sport         string
	FeaturedTeam  string
	FavoriteTeams []string
}

// ParseSportsConfig reads a sports widget's stored config, defaulting to the NFL.
func ParseSportsConfig(cfg map[string]any) (SportsConfig, error) {
	sc := SportsConfig{Sport: "nfl"}
	if s, ok := cfg["sport"].(string); ok && s != "" {
		sc.Sport = strings.ToLower(s)
	}
	if _, err := SportPath(sc.Sport); err != nil {
		return sc, err
	}
	if t, ok := cfg["featuredTeam"].(string); ok && t != "" {
		id, err := NormalizeTeamID(t)
		if err != nil {
			return sc, err
		}
		sc.FeaturedTeam = id
	}
	favorites, _ := cfg["favoriteTeams"].([]any)
	for _, v := range favorites {
		s, _ := v.(string)
		id, err := NormalizeTeamID(s)
		if err != nil {
			return sc, err
		}
		sc.FavoriteTeams = append(sc.FavoriteTeams, id)
	}
	return sc, nil
}

// GameStatus is bound to the GraphQL GameStatus enum.
type GameStatus string

const (
	GameStatusScheduled  GameStatus = "SCHEDULED"
	GameStatusInProgress GameStatus = "IN_PROGRESS"
	GameStatusFinal      GameStatus = "FINAL"
	GameStatusPostponed  GameStatus = "POSTPONED"
	GameStatusCanceled   GameStatus = "CANCELED"
)

func (s GameStatus) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(string(s)))
}

func (s *GameStatus) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("GameStatus must be a string")
	}
	*s = GameStatus(str)
	return nil
}

// GameStatusFrom maps ESPN's status state ("pre", "in", "post") and name (e.g.
// "STATUS_POSTPONED") to a GameStatus. Postponed and canceled games can appear in
// either the "pre" or "post" state, so the name is checked first.
func GameStatusFrom(state, name string) GameStatus {
	switch name {
	case "STATUS_POSTPONED", "STATUS_SUSPENDED":
		return GameStatusPostponed
	case "STATUS_CANCELED", "STATUS_FORFEIT":
		return GameStatusCanceled
	}
	switch state {
	case "in":
		return GameStatusInProgress
	case "post":
		return GameStatusFinal
	default:
		return GameStatusScheduled
	}
}

// Team is bound to the GraphQL Team type. Logo and Color may be empty.
type Team struct {
	ID           string `json:"id"`
	Abbreviation string `json:"abbreviation"`
	DisplayName  string `json:"displayName"`
	ShortName    string `json:"shortName"`
	Logo         string `json:"logo"`
	Color        string `json:"color"`
}

// GameTeam is one side of a Game. Score and Winner are nil before kickoff.
type GameTeam struct {
	Team   *Team   `json:"team"`
	Score  *int    `json:"score"`
	Winner *bool   `json:"winner"`
	Record *string `json:"record"`
}

type Game struct {
	ID           string     `json:"id"`
	Week         *int       `json:"week"`
	Date         string     `json:"date"`
	TimeValid    bool       `json:"timeValid"`
	Status       GameStatus `json:"status"`
	StatusDetail string     `json:"statusDetail"`
	Broadcast    *string    `json:"broadcast"`
	Home         *GameTeam  `json:"home"`
	Away         *GameTeam  `json:"away"`
}

// HasTeam reports whether id (an abbreviation or ESPN id, any case) plays in the game.
func (g *Game) HasTeam(id string) bool {
	for _, side := range []*GameTeam{g.Home, g.Away} {
		if strings.EqualFold(side.Team.Abbreviation, id) || side.Team.ID == id {
			return true
		}
	}
	return false
}

// SportsData is a league's scoreboard for the current week.
type SportsData struct {
	Week          *int    `json:"week"`
	RecentGames   []*Game `json:"recentGames"`
	UpcomingGames []*Game `json:"upcomingGames"`
}

type TeamSchedule struct {
	Team            *Team   `json:"team"`
	Record          string  `json:"record"`
	StandingSummary string  `json:"standingSummary"`
	ByeWeek         *int    `json:"byeWeek"`
	NextGame        *Game   `json:"nextGame"`
	Games           []*Game `json:"games"`
}

type StandingsConference struct {
	Name         string               `json:"name"`
	Abbreviation string               `json:"abbreviation"`
	Divisions    []*StandingsDivision `json:"divisions"`
}

type StandingsDivision struct {
	Name  string            `json:"name"`
	Teams []*StandingsEntry `json:"teams"`
}

type StandingsEntry struct {
	Team              *Team  `json:"team"`
	Wins              int    `json:"wins"`
	Losses            int    `json:"losses"`
	Ties              int    `json:"ties"`
	WinPercent        string `json:"winPercent"`
	PointDifferential string `json:"pointDifferential"`
	Streak            string `json:"streak"`
	PlayoffSeed       *int   `json:"playoffSeed"`
}

// SportsClient fetches data from ESPN's public (unofficial, keyless) API.
type SportsClient struct {
	HTTP    *http.Client
	BaseURL string
}

func NewSportsClient() *SportsClient {
	return &SportsClient{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		BaseURL: "https://site.api.espn.com/apis",
	}
}

// FetchScoreboard returns the current week's games. Between weeks ESPN's current week
// has no finished games yet, so recent results come from the previous week instead.
func (c *SportsClient) FetchScoreboard(ctx context.Context, sport string) (*SportsData, error) {
	path, err := SportPath(sport)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, "/site/v2/sports/"+path+"/scoreboard")
	if err != nil {
		return nil, err
	}
	sb, err := ParseScoreboard(body)
	if err != nil {
		return nil, err
	}

	if len(sb.Data.RecentGames) == 0 && sb.Week > 1 {
		prevURL := fmt.Sprintf("/site/v2/sports/%s/scoreboard?seasontype=%d&week=%d", path, sb.SeasonType, sb.Week-1)
		prevBody, err := c.get(ctx, prevURL)
		if err != nil {
			return nil, fmt.Errorf("previous week: %w", err)
		}
		prev, err := ParseScoreboard(prevBody)
		if err != nil {
			return nil, fmt.Errorf("previous week: %w", err)
		}
		sb.Data.RecentGames = prev.Data.RecentGames
	}
	return sb.Data, nil
}

func (c *SportsClient) FetchTeamSchedule(ctx context.Context, sport, teamID string) (*TeamSchedule, error) {
	path, err := SportPath(sport)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, "/site/v2/sports/"+path+"/teams/"+teamID+"/schedule")
	if err != nil {
		return nil, err
	}
	return ParseTeamSchedule(body)
}

func (c *SportsClient) FetchStandings(ctx context.Context, sport string) ([]*StandingsConference, error) {
	path, err := SportPath(sport)
	if err != nil {
		return nil, err
	}
	// level=3 groups teams by conference and then division.
	body, err := c.get(ctx, "/v2/sports/"+path+"/standings?level=3")
	if err != nil {
		return nil, err
	}
	return ParseStandings(body)
}

func (c *SportsClient) get(ctx context.Context, pathAndQuery string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+pathAndQuery, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("espn request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("espn read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("espn: status %d for %s", resp.StatusCode, pathAndQuery)
	}
	return body, nil
}

// ESPN response shapes. The scoreboard, team schedule, and standings endpoints describe
// the same things slightly differently, so these types accept every variant.

type espnLogo struct {
	Href string   `json:"href"`
	Rel  []string `json:"rel"`
}

type espnTeam struct {
	ID               string     `json:"id"`
	Abbreviation     string     `json:"abbreviation"`
	DisplayName      string     `json:"displayName"`
	ShortDisplayName string     `json:"shortDisplayName"`
	Name             string     `json:"name"`
	Color            string     `json:"color"`
	Logo             string     `json:"logo"`
	Logos            []espnLogo `json:"logos"`
}

func (t espnTeam) toTeam() *Team {
	logo := t.Logo
	for _, l := range t.Logos {
		if slices.Contains(l.Rel, "default") {
			logo = l.Href
			break
		}
	}
	if logo == "" && len(t.Logos) > 0 {
		logo = t.Logos[0].Href
	}
	short := t.ShortDisplayName
	if short == "" {
		short = t.Name
	}
	return &Team{
		ID:           t.ID,
		Abbreviation: t.Abbreviation,
		DisplayName:  t.DisplayName,
		ShortName:    short,
		Logo:         logo,
		Color:        t.Color,
	}
}

// espnScore accepts a score as a string ("24"), an object ({"value": 24.0}), or null.
type espnScore struct {
	value *int
}

func (s *espnScore) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	if len(b) > 0 && b[0] == '{' {
		var obj struct {
			Value *float64 `json:"value"`
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		if obj.Value != nil {
			v := int(*obj.Value)
			s.value = &v
		}
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return fmt.Errorf("score: %w", err)
	}
	if str == "" {
		return nil
	}
	v, err := strconv.Atoi(str)
	if err != nil {
		return fmt.Errorf("score %q: %w", str, err)
	}
	s.value = &v
	return nil
}

type espnCompetitor struct {
	HomeAway string    `json:"homeAway"`
	Winner   *bool     `json:"winner"`
	Score    espnScore `json:"score"`
	Team     espnTeam  `json:"team"`
	// The scoreboard uses "records" with "summary"; the team schedule uses "record" with "displayValue".
	Records []struct {
		Type    string `json:"type"`
		Summary string `json:"summary"`
	} `json:"records"`
	Record []struct {
		Type         string `json:"type"`
		DisplayValue string `json:"displayValue"`
	} `json:"record"`
}

func (c espnCompetitor) overallRecord() *string {
	for _, r := range c.Records {
		if r.Type == "total" && r.Summary != "" {
			return &r.Summary
		}
	}
	for _, r := range c.Record {
		if r.Type == "total" && r.DisplayValue != "" {
			return &r.DisplayValue
		}
	}
	return nil
}

type espnEvent struct {
	ID   string `json:"id"`
	Date string `json:"date"`
	Week struct {
		Number int `json:"number"`
	} `json:"week"`
	Competitions []struct {
		TimeValid *bool `json:"timeValid"`
		Status    struct {
			Type struct {
				Name        string `json:"name"`
				State       string `json:"state"`
				ShortDetail string `json:"shortDetail"`
			} `json:"type"`
		} `json:"status"`
		Competitors []espnCompetitor `json:"competitors"`
		// The scoreboard lists network names; the team schedule nests them under "media".
		Broadcasts []struct {
			Names []string `json:"names"`
			Media struct {
				ShortName string `json:"shortName"`
			} `json:"media"`
		} `json:"broadcasts"`
	} `json:"competitions"`
}

func (e espnEvent) toGame() (*Game, error) {
	if len(e.Competitions) == 0 {
		return nil, fmt.Errorf("event %s has no competition", e.ID)
	}
	comp := e.Competitions[0]
	status := GameStatusFrom(comp.Status.Type.State, comp.Status.Type.Name)
	played := status == GameStatusInProgress || status == GameStatusFinal

	g := &Game{
		ID:           e.ID,
		Date:         e.Date,
		TimeValid:    comp.TimeValid == nil || *comp.TimeValid,
		Status:       status,
		StatusDetail: comp.Status.Type.ShortDetail,
	}
	if e.Week.Number > 0 {
		week := e.Week.Number
		g.Week = &week
	}

	var networks []string
	for _, b := range comp.Broadcasts {
		networks = append(networks, b.Names...)
		if b.Media.ShortName != "" {
			networks = append(networks, b.Media.ShortName)
		}
	}
	if len(networks) > 0 {
		joined := strings.Join(slices.Compact(networks), "/")
		g.Broadcast = &joined
	}

	for _, c := range comp.Competitors {
		side := &GameTeam{Team: c.Team.toTeam(), Record: c.overallRecord()}
		// Scheduled games report a score of "0"; only games that have started have scores.
		if played {
			side.Score = c.Score.value
		}
		if status == GameStatusFinal {
			side.Winner = c.Winner
		}
		switch c.HomeAway {
		case "home":
			g.Home = side
		case "away":
			g.Away = side
		}
	}
	if g.Home == nil || g.Away == nil {
		return nil, fmt.Errorf("event %s is missing a home or away team", e.ID)
	}
	return g, nil
}

// Scoreboard is a parsed scoreboard response plus the week it covers.
type Scoreboard struct {
	Week       int
	SeasonType int
	Data       *SportsData
}

// ParseScoreboard converts an ESPN scoreboard response. Recent games are live games
// followed by finished games (most recent first); upcoming games are soonest first.
func ParseScoreboard(body []byte) (*Scoreboard, error) {
	var r struct {
		Week struct {
			Number int `json:"number"`
		} `json:"week"`
		Season struct {
			Type int `json:"type"`
		} `json:"season"`
		Events []espnEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode espn scoreboard: %w", err)
	}

	data := &SportsData{RecentGames: []*Game{}, UpcomingGames: []*Game{}}
	if r.Week.Number > 0 {
		week := r.Week.Number
		data.Week = &week
	}
	for _, e := range r.Events {
		g, err := e.toGame()
		if err != nil {
			return nil, err
		}
		switch g.Status {
		case GameStatusInProgress, GameStatusFinal:
			data.RecentGames = append(data.RecentGames, g)
		case GameStatusScheduled:
			data.UpcomingGames = append(data.UpcomingGames, g)
		}
		// Postponed and canceled games are left out.
	}

	slices.SortStableFunc(data.RecentGames, func(a, b *Game) int {
		aLive, bLive := a.Status == GameStatusInProgress, b.Status == GameStatusInProgress
		if aLive != bLive {
			if aLive {
				return -1
			}
			return 1
		}
		return strings.Compare(b.Date, a.Date)
	})
	slices.SortStableFunc(data.UpcomingGames, func(a, b *Game) int {
		return strings.Compare(a.Date, b.Date)
	})
	return &Scoreboard{Week: r.Week.Number, SeasonType: r.Season.Type, Data: data}, nil
}

// ParseTeamSchedule converts an ESPN team schedule response. NextGame is the first game
// that is live or not yet played.
func ParseTeamSchedule(body []byte) (*TeamSchedule, error) {
	var r struct {
		Team struct {
			espnTeam
			RecordSummary   string `json:"recordSummary"`
			StandingSummary string `json:"standingSummary"`
		} `json:"team"`
		ByeWeek int         `json:"byeWeek"`
		Events  []espnEvent `json:"events"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode espn team schedule: %w", err)
	}
	if r.Team.ID == "" {
		return nil, fmt.Errorf("espn team schedule has no team")
	}

	s := &TeamSchedule{
		Team:            r.Team.toTeam(),
		Record:          r.Team.RecordSummary,
		StandingSummary: r.Team.StandingSummary,
		Games:           make([]*Game, 0, len(r.Events)),
	}
	if r.ByeWeek > 0 {
		bye := r.ByeWeek
		s.ByeWeek = &bye
	}
	for _, e := range r.Events {
		g, err := e.toGame()
		if err != nil {
			return nil, err
		}
		s.Games = append(s.Games, g)
		if s.NextGame == nil && (g.Status == GameStatusScheduled || g.Status == GameStatusInProgress) {
			s.NextGame = g
		}
	}
	return s, nil
}

type espnStandingsNode struct {
	Name         string              `json:"name"`
	Abbreviation string              `json:"abbreviation"`
	Children     []espnStandingsNode `json:"children"`
	Standings    struct {
		Entries []struct {
			Team  espnTeam `json:"team"`
			Stats []struct {
				Name         string   `json:"name"`
				Value        *float64 `json:"value"`
				DisplayValue string   `json:"displayValue"`
			} `json:"stats"`
		} `json:"entries"`
	} `json:"standings"`
}

// ParseStandings converts an ESPN standings response requested with level=3
// (league -> conferences -> divisions). Teams keep ESPN's order, which applies tiebreakers.
func ParseStandings(body []byte) ([]*StandingsConference, error) {
	var league espnStandingsNode
	if err := json.Unmarshal(body, &league); err != nil {
		return nil, fmt.Errorf("decode espn standings: %w", err)
	}
	if len(league.Children) == 0 {
		return nil, fmt.Errorf("espn standings has no conferences")
	}

	conferences := make([]*StandingsConference, 0, len(league.Children))
	for _, conf := range league.Children {
		c := &StandingsConference{Name: conf.Name, Abbreviation: conf.Abbreviation}
		for _, div := range conf.Children {
			d := &StandingsDivision{Name: div.Name}
			for _, e := range div.Standings.Entries {
				entry := &StandingsEntry{Team: e.Team.toTeam()}
				for _, stat := range e.Stats {
					value := 0
					if stat.Value != nil {
						value = int(*stat.Value)
					}
					switch stat.Name {
					case "wins":
						entry.Wins = value
					case "losses":
						entry.Losses = value
					case "ties":
						entry.Ties = value
					case "winPercent":
						entry.WinPercent = stat.DisplayValue
					case "differential":
						entry.PointDifferential = stat.DisplayValue
					case "streak":
						entry.Streak = stat.DisplayValue
					case "playoffSeed":
						if value > 0 {
							entry.PlayoffSeed = &value
						}
					}
				}
				d.Teams = append(d.Teams, entry)
			}
			c.Divisions = append(c.Divisions, d)
		}
		conferences = append(conferences, c)
	}
	return conferences, nil
}
