// Package config loads runtime settings from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL         string
	Port                string
	AllowedOrigin       string
	YouTubeAPIKey       string
	DefaultLat          float64
	DefaultLon          float64
	DefaultLocationName string
}

// Load reads configuration from the environment, applying defaults for optional values.
func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		Port:                getEnv("PORT", "8080"),
		AllowedOrigin:       getEnv("ALLOWED_ORIGIN", "http://localhost:3000"),
		YouTubeAPIKey:       os.Getenv("YOUTUBE_API_KEY"),
		DefaultLocationName: getEnv("DEFAULT_LOCATION_NAME", "St. George, UT"),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	var err error
	if cfg.DefaultLat, err = getFloat("DEFAULT_LAT", 37.0965); err != nil {
		return nil, err
	}
	if cfg.DefaultLon, err = getFloat("DEFAULT_LON", -113.5684); err != nil {
		return nil, err
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getFloat(key string, fallback float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	return f, nil
}
