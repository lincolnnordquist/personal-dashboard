package main

import (
	"context"
	"errors"
	"log"
	"time"

	"dashboard/cache"
	"dashboard/db"
	"dashboard/widgets"
)

// every runs fn immediately and then on each tick until ctx is cancelled.
func every(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		fn(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// refreshWeather re-fetches weather for every enabled weather widget and writes it to the cache.
func refreshWeather(repo *db.WidgetRepo, store cache.Store, client *widgets.WeatherClient) func(context.Context) {
	return func(ctx context.Context) {
		configs, err := repo.ListEnabledByType(ctx, widgets.WeatherWidgetType)
		if err != nil {
			log.Printf("refresh weather: list widgets: %v", err)
			return
		}
		for _, w := range configs {
			wc, err := widgets.ParseWeatherConfig(w.Config)
			if err != nil {
				log.Printf("refresh weather: widget %d: %v", w.ID, err)
				continue
			}
			data, err := client.Fetch(ctx, wc.Lat, wc.Lon, wc.Unit)
			if err != nil {
				log.Printf("refresh weather: widget %d: %v", w.ID, err)
				continue
			}
			key := widgets.WeatherCacheKey(wc.Lat, wc.Lon, wc.Unit)
			if err := cache.Put(ctx, store, widgets.WeatherWidgetType, key, widgets.WeatherTTL, data); err != nil {
				log.Printf("refresh weather: widget %d: cache: %v", w.ID, err)
			}
		}
	}
}

// refreshReddit re-fetches every subreddit used by an enabled reddit widget, one at a time.
func refreshReddit(repo *db.WidgetRepo, store cache.Store, client *widgets.RedditClient) func(context.Context) {
	return func(ctx context.Context) {
		configs, err := repo.ListEnabledByType(ctx, widgets.RedditWidgetType)
		if err != nil {
			log.Printf("refresh reddit: list widgets: %v", err)
			return
		}
		seen := map[string]bool{}
		for _, w := range configs {
			subs, err := widgets.ParseRedditConfig(w.Config)
			if err != nil {
				log.Printf("refresh reddit: widget %d: %v", w.ID, err)
				continue
			}
			for _, sub := range subs {
				if seen[sub] {
					continue
				}
				seen[sub] = true
				feed, err := fetchSubredditPatiently(ctx, client, sub)
				if err != nil {
					log.Printf("refresh reddit: %v", err)
					continue
				}
				if err := cache.Put(ctx, store, widgets.RedditWidgetType, widgets.RedditCacheKey(sub), widgets.RedditTTL, feed); err != nil {
					log.Printf("refresh reddit: r/%s: cache: %v", sub, err)
				}
			}
		}
	}
}

// fetchSubredditPatiently waits out Reddit's rate limit instead of failing. Only background
// refreshes use it; GraphQL requests fail fast and fall back to cached data.
func fetchSubredditPatiently(ctx context.Context, client *widgets.RedditClient, sub string) (*widgets.SubredditFeed, error) {
	for attempt := 1; ; attempt++ {
		if err := sleepUntil(ctx, client.ReadyAt()); err != nil {
			return nil, err
		}
		feed, err := client.Fetch(ctx, sub)
		if !errors.Is(err, widgets.ErrRateLimited) || attempt == 3 {
			return feed, err
		}
	}
}

func sleepUntil(ctx context.Context, t time.Time) error {
	d := time.Until(t)
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// refreshSports re-fetches the scoreboard, standings, and featured team schedule for every
// enabled sports widget.
func refreshSports(repo *db.WidgetRepo, store cache.Store, client *widgets.SportsClient) func(context.Context) {
	return func(ctx context.Context) {
		configs, err := repo.ListEnabledByType(ctx, widgets.SportsWidgetType)
		if err != nil {
			log.Printf("refresh sports: list widgets: %v", err)
			return
		}
		sports := map[string]bool{}
		teams := map[[2]string]bool{}
		for _, w := range configs {
			sc, err := widgets.ParseSportsConfig(w.Config)
			if err != nil {
				log.Printf("refresh sports: widget %d: %v", w.ID, err)
				continue
			}
			sports[sc.Sport] = true
			if sc.FeaturedTeam != "" {
				teams[[2]string{sc.Sport, sc.FeaturedTeam}] = true
			}
		}

		put := func(key string, v any, err error) {
			if err != nil {
				log.Printf("refresh sports: %s: %v", key, err)
				return
			}
			if err := cache.Put(ctx, store, widgets.SportsWidgetType, key, widgets.SportsTTL, v); err != nil {
				log.Printf("refresh sports: %s: cache: %v", key, err)
			}
		}
		for sport := range sports {
			scoreboard, err := client.FetchScoreboard(ctx, sport)
			put(widgets.ScoreboardCacheKey(sport), scoreboard, err)
			standings, err := client.FetchStandings(ctx, sport)
			put(widgets.StandingsCacheKey(sport), standings, err)
		}
		for t := range teams {
			schedule, err := client.FetchTeamSchedule(ctx, t[0], t[1])
			put(widgets.TeamScheduleCacheKey(t[0], t[1]), schedule, err)
		}
	}
}
