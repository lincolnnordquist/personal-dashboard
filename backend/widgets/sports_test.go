package widgets

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return body
}

func TestGameStatusFrom(t *testing.T) {
	tests := []struct {
		state, name string
		want        GameStatus
	}{
		{"pre", "STATUS_SCHEDULED", GameStatusScheduled},
		{"in", "STATUS_IN_PROGRESS", GameStatusInProgress},
		{"in", "STATUS_HALFTIME", GameStatusInProgress},
		{"in", "STATUS_END_PERIOD", GameStatusInProgress},
		{"post", "STATUS_FINAL", GameStatusFinal},
		{"post", "STATUS_FINAL_OVERTIME", GameStatusFinal},
		{"pre", "STATUS_POSTPONED", GameStatusPostponed},
		{"post", "STATUS_POSTPONED", GameStatusPostponed},
		{"post", "STATUS_CANCELED", GameStatusCanceled},
		{"", "", GameStatusScheduled},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, GameStatusFrom(tt.state, tt.name), "%s/%s", tt.state, tt.name)
	}
}

func TestParseScoreboardScheduledWeek(t *testing.T) {
	sb, err := ParseScoreboard(readFixture(t, "espn_scoreboard_week3.json"))
	require.NoError(t, err)

	assert.Equal(t, 3, sb.Week)
	assert.Equal(t, 2, sb.SeasonType)
	assert.Empty(t, sb.Data.RecentGames)
	require.Len(t, sb.Data.UpcomingGames, 2)

	g := sb.Data.UpcomingGames[0]
	assert.Equal(t, "401872948", g.ID)
	assert.Equal(t, GameStatusScheduled, g.Status)
	assert.Equal(t, "9/24 - 8:15 PM EDT", g.StatusDetail)
	assert.Equal(t, "GB", g.Home.Team.Abbreviation)
	assert.Equal(t, "ATL", g.Away.Team.Abbreviation)
	assert.Nil(t, g.Home.Score, `scheduled games report "0", which is not a real score`)
	assert.Nil(t, g.Home.Winner)
	require.NotNil(t, g.Broadcast)
	assert.Equal(t, "Prime Video", *g.Broadcast)
	require.NotNil(t, g.Home.Record)
	assert.Equal(t, "1-1", *g.Home.Record)
	assert.Equal(t, "https://a.espncdn.com/i/teamlogos/nfl/500/scoreboard/gb.png", g.Home.Team.Logo)
}

func TestParseScoreboardFinalWeek(t *testing.T) {
	sb, err := ParseScoreboard(readFixture(t, "espn_scoreboard_week2.json"))
	require.NoError(t, err)
	require.Len(t, sb.Data.RecentGames, 2)
	assert.Empty(t, sb.Data.UpcomingGames)

	// Most recent first.
	g := sb.Data.RecentGames[0]
	assert.Equal(t, "401872933", g.ID)
	assert.Equal(t, GameStatusFinal, g.Status)
	assert.Equal(t, "ATL", g.Home.Team.Abbreviation)
	assert.Equal(t, 3, *g.Home.Score)
	assert.Equal(t, 34, *g.Away.Score)
	assert.False(t, *g.Home.Winner)
	assert.True(t, *g.Away.Winner)
}

func TestParseScoreboardOrdersLiveGamesFirst(t *testing.T) {
	body := []byte(`{"week":{"number":5},"season":{"type":2},"events":[
		{"id":"1","date":"2026-10-11T17:00Z","competitions":[{"status":{"type":{"state":"post","name":"STATUS_FINAL"}},
			"competitors":[{"homeAway":"home","score":"20","winner":true,"team":{"abbreviation":"SEA"}},
			               {"homeAway":"away","score":"17","winner":false,"team":{"abbreviation":"SF"}}]}]},
		{"id":"2","date":"2026-10-11T20:25Z","competitions":[{"status":{"type":{"state":"in","name":"STATUS_IN_PROGRESS","shortDetail":"3rd 5:12"}},
			"competitors":[{"homeAway":"home","score":"7","team":{"abbreviation":"KC"}},
			               {"homeAway":"away","score":"10","team":{"abbreviation":"DEN"}}]}]},
		{"id":"3","date":"2026-10-11T20:25Z","competitions":[{"status":{"type":{"state":"post","name":"STATUS_POSTPONED"}},
			"competitors":[{"homeAway":"home","score":"0","team":{"abbreviation":"BUF"}},
			               {"homeAway":"away","score":"0","team":{"abbreviation":"MIA"}}]}]}
	]}`)
	sb, err := ParseScoreboard(body)
	require.NoError(t, err)
	require.Len(t, sb.Data.RecentGames, 2, "postponed games are left out")

	live := sb.Data.RecentGames[0]
	assert.Equal(t, GameStatusInProgress, live.Status)
	assert.Equal(t, "3rd 5:12", live.StatusDetail)
	assert.Equal(t, 7, *live.Home.Score)
	assert.Nil(t, live.Home.Winner, "no winner until the game is final")
	assert.Equal(t, GameStatusFinal, sb.Data.RecentGames[1].Status)
}

func TestParseScoreboardInvalid(t *testing.T) {
	_, err := ParseScoreboard([]byte(`{"events":[{"id":"1","competitions":[{"competitors":[{"homeAway":"home","score":"abc"}]}]}]}`))
	assert.Error(t, err)
}

func TestParseTeamSchedule(t *testing.T) {
	s, err := ParseTeamSchedule(readFixture(t, "espn_team_schedule_sea.json"))
	require.NoError(t, err)

	assert.Equal(t, "SEA", s.Team.Abbreviation)
	assert.Equal(t, "Seattle Seahawks", s.Team.DisplayName)
	assert.Equal(t, "2-0", s.Record)
	assert.Equal(t, "1st in NFC West", s.StandingSummary)
	require.NotNil(t, s.ByeWeek)
	assert.Equal(t, 11, *s.ByeWeek)
	require.Len(t, s.Games, 5)

	// Team schedule scores are objects ({"value": 13.0}), not strings.
	week1 := s.Games[0]
	assert.Equal(t, 1, *week1.Week)
	assert.Equal(t, 13, *week1.Home.Score)
	assert.Equal(t, 10, *week1.Away.Score)
	assert.True(t, *week1.Home.Winner)
	assert.Equal(t, "NBC", *week1.Broadcast)
	assert.Equal(t, "1-0", *week1.Home.Record)
	assert.Equal(t, "https://a.espncdn.com/i/teamlogos/nfl/500/sea.png", week1.Home.Team.Logo)

	require.NotNil(t, s.NextGame)
	assert.Equal(t, "401872955", s.NextGame.ID, "next game is the first one not yet played")
	assert.Equal(t, "WSH", s.NextGame.Home.Team.Abbreviation)
	assert.Nil(t, s.NextGame.Home.Score)

	week18 := s.Games[4]
	assert.False(t, week18.TimeValid, "kickoff time not set yet")
	assert.Equal(t, "TBD", week18.StatusDetail)
	assert.Nil(t, week18.Broadcast)
}

func TestParseTeamScheduleSeasonOver(t *testing.T) {
	body := []byte(`{"team":{"id":"26","abbreviation":"SEA"},"events":[
		{"id":"1","date":"2027-01-03T18:00Z","competitions":[{"status":{"type":{"state":"post","name":"STATUS_FINAL"}},
			"competitors":[{"homeAway":"home","score":{"value":20},"team":{"abbreviation":"CAR"}},
			               {"homeAway":"away","score":{"value":24},"team":{"abbreviation":"SEA"}}]}]}
	]}`)
	s, err := ParseTeamSchedule(body)
	require.NoError(t, err)
	assert.Nil(t, s.NextGame)
	assert.Nil(t, s.ByeWeek)
}

func TestParseStandings(t *testing.T) {
	conferences, err := ParseStandings(readFixture(t, "espn_standings.json"))
	require.NoError(t, err)
	require.Len(t, conferences, 2)

	nfc := conferences[1]
	assert.Equal(t, "National Football Conference", nfc.Name)
	assert.Equal(t, "NFC", nfc.Abbreviation)
	require.Len(t, nfc.Divisions, 4)

	west := nfc.Divisions[3]
	assert.Equal(t, "NFC West", west.Name)
	require.Len(t, west.Teams, 4)
	var order []string
	for _, e := range west.Teams {
		order = append(order, e.Team.Abbreviation)
	}
	assert.Equal(t, []string{"SEA", "SF", "LAR", "ARI"}, order, "ESPN's tiebreaker order is kept")

	sea := west.Teams[0]
	assert.Equal(t, 2, sea.Wins)
	assert.Equal(t, 0, sea.Losses)
	assert.Equal(t, 0, sea.Ties)
	assert.Equal(t, "1.000", sea.WinPercent)
	assert.Equal(t, "+27", sea.PointDifferential)
	assert.Equal(t, "W2", sea.Streak)
	require.NotNil(t, sea.PlayoffSeed)
	assert.Equal(t, 2, *sea.PlayoffSeed)
}

func TestParseStandingsEmpty(t *testing.T) {
	_, err := ParseStandings([]byte(`{"name":"National Football League"}`))
	assert.Error(t, err)
}

func TestFetchScoreboardFallsBackToPreviousWeek(t *testing.T) {
	week3 := readFixture(t, "espn_scoreboard_week3.json")
	week2 := readFixture(t, "espn_scoreboard_week2.json")
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/site/v2/sports/football/nfl/scoreboard", r.URL.Path)
		queries = append(queries, r.URL.RawQuery)
		if r.URL.Query().Get("week") == "2" {
			w.Write(week2)
			return
		}
		w.Write(week3)
	}))
	defer srv.Close()

	c := NewSportsClient()
	c.BaseURL = srv.URL
	data, err := c.FetchScoreboard(context.Background(), "nfl")
	require.NoError(t, err)

	assert.Equal(t, []string{"", "seasontype=2&week=2"}, queries)
	assert.Equal(t, 3, *data.Week)
	assert.Len(t, data.UpcomingGames, 2, "upcoming games come from the current week")
	assert.Len(t, data.RecentGames, 2, "recent results come from the previous week")
}

func TestFetchUnsupportedSport(t *testing.T) {
	_, err := NewSportsClient().FetchScoreboard(context.Background(), "curling")
	assert.ErrorContains(t, err, "unsupported sport")
}

func TestGameHasTeam(t *testing.T) {
	g := &Game{
		Home: &GameTeam{Team: &Team{ID: "26", Abbreviation: "SEA"}},
		Away: &GameTeam{Team: &Team{ID: "25", Abbreviation: "SF"}},
	}
	assert.True(t, g.HasTeam("sea"))
	assert.True(t, g.HasTeam("25"))
	assert.False(t, g.HasTeam("kc"))
}

func TestParseSportsConfig(t *testing.T) {
	sc, err := ParseSportsConfig(map[string]any{"featuredTeam": "SEA", "favoriteTeams": []any{"SF", "kc"}})
	require.NoError(t, err)
	assert.Equal(t, SportsConfig{Sport: "nfl", FeaturedTeam: "sea", FavoriteTeams: []string{"sf", "kc"}}, sc)

	_, err = ParseSportsConfig(map[string]any{"featuredTeam": "../etc"})
	assert.Error(t, err)
	_, err = ParseSportsConfig(map[string]any{"sport": "curling"})
	assert.Error(t, err)
}
