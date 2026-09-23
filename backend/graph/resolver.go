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
	Now        func() time.Time
}

func (r *Resolver) fetchWeather(ctx context.Context, lat, lon float64, unit string) (*widgets.WeatherData, error) {
	key := widgets.WeatherCacheKey(lat, lon, unit)
	return cache.GetOrFetch(ctx, r.Cache, widgets.WeatherWidgetType, key, widgets.WeatherTTL, r.Now,
		func(ctx context.Context) (*widgets.WeatherData, error) {
			return r.Weather.Fetch(ctx, lat, lon, unit)
		})
}
