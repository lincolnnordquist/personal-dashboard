package graph

import (
	"context"
	"time"

	"dashboard/cache"
	"dashboard/db"
	"dashboard/widgets"
)

// Resolver holds the dependencies shared by all resolvers.
type Resolver struct {
	WidgetRepo *db.WidgetRepo
	Cache      cache.Store
	Weather    *widgets.WeatherClient
	Reddit     *widgets.RedditClient
	Sports     *widgets.SportsClient
	YouTube    *widgets.YouTubeClient
	Docker     *widgets.DockerClient // nil when the Docker client could not be created
	Now        func() time.Time
}

func (r *Resolver) fetchWeather(ctx context.Context, lat, lon float64, unit string) (*widgets.WeatherData, error) {
	key := widgets.WeatherCacheKey(lat, lon, unit)
	return cache.GetOrFetch(ctx, r.Cache, widgets.WeatherWidgetType, key, widgets.WeatherTTL, r.Now,
		func(ctx context.Context) (*widgets.WeatherData, error) {
			return r.Weather.Fetch(ctx, lat, lon, unit)
		})
}

func (r *Resolver) fetchSubreddit(ctx context.Context, subreddit string) (*widgets.SubredditFeed, error) {
	return cache.GetOrFetch(ctx, r.Cache, widgets.RedditWidgetType, widgets.RedditCacheKey(subreddit), widgets.RedditTTL, r.Now,
		func(ctx context.Context) (*widgets.SubredditFeed, error) {
			return r.Reddit.Fetch(ctx, subreddit)
		})
}

func (r *Resolver) fetchScoreboard(ctx context.Context, sport string) (*widgets.SportsData, error) {
	return cache.GetOrFetch(ctx, r.Cache, widgets.SportsWidgetType, widgets.ScoreboardCacheKey(sport), widgets.SportsTTL, r.Now,
		func(ctx context.Context) (*widgets.SportsData, error) {
			return r.Sports.FetchScoreboard(ctx, sport)
		})
}

func (r *Resolver) fetchTeamSchedule(ctx context.Context, sport, teamID string) (*widgets.TeamSchedule, error) {
	return cache.GetOrFetch(ctx, r.Cache, widgets.SportsWidgetType, widgets.TeamScheduleCacheKey(sport, teamID), widgets.SportsTTL, r.Now,
		func(ctx context.Context) (*widgets.TeamSchedule, error) {
			return r.Sports.FetchTeamSchedule(ctx, sport, teamID)
		})
}

func (r *Resolver) fetchStandings(ctx context.Context, sport string) ([]*widgets.StandingsConference, error) {
	return cache.GetOrFetch(ctx, r.Cache, widgets.SportsWidgetType, widgets.StandingsCacheKey(sport), widgets.SportsTTL, r.Now,
		func(ctx context.Context) ([]*widgets.StandingsConference, error) {
			return r.Sports.FetchStandings(ctx, sport)
		})
}

func (r *Resolver) fetchYouTubeChannel(ctx context.Context, ref string) (*widgets.YoutubeChannel, error) {
	return cache.GetOrFetch(ctx, r.Cache, widgets.YouTubeWidgetType, widgets.YouTubeCacheKey(ref), widgets.YouTubeTTL, r.Now,
		func(ctx context.Context) (*widgets.YoutubeChannel, error) {
			return r.YouTube.Fetch(ctx, ref)
		})
}
