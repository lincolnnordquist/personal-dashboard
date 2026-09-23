package main

import (
	"context"
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
