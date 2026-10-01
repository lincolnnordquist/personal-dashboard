package widgets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWeather(t *testing.T) {
	body := readFixture(t, "open_meteo_forecast.json")
	data, err := ParseWeather(body)
	require.NoError(t, err)

	assert.Equal(t, 90.8, data.Temperature)
	assert.Equal(t, "Clear", data.Condition)

	require.NotEmpty(t, data.Hourly)
	assert.Len(t, data.Hourly, hourlyWindowHours)
	// The fixture's current.time is exactly on the hour (2026-10-01T16:00), so the window starts there.
	assert.Equal(t, "2026-10-01T16:00", data.Hourly[0].Time)
	assert.Equal(t, "2026-10-02T15:00", data.Hourly[hourlyWindowHours-1].Time)

	// 16:00 is before the fixture's 19:17 sunset, so still day; 20:00 (the hour after) isn't.
	assert.True(t, data.Hourly[0].IsDay, "4pm should be day")
	assert.True(t, data.Hourly[3].IsDay, "7pm should still be day")
	assert.False(t, data.Hourly[4].IsDay, "8pm should be night")

	// Fixture's daily sunrise/sunset: today 07:29/19:17, tomorrow 07:30/19:15. Today's sunrise
	// already passed (current time is 16:00) and tomorrow's sunset falls after the 24h window
	// closes, so only today's sunset and tomorrow's sunrise should appear.
	require.NotNil(t, data.Sunset)
	assert.Equal(t, "2026-10-01T19:17", *data.Sunset)
	require.NotNil(t, data.Sunrise)
	assert.Equal(t, "2026-10-02T07:30", *data.Sunrise)
}

func TestHourlyForecast(t *testing.T) {
	times := []string{
		"2026-10-01T13:00", "2026-10-01T14:00", "2026-10-01T15:00", "2026-10-01T16:00", "2026-10-01T17:00",
	}
	temps := []float64{70, 71, 72, 73, 74}
	codes := []int{0, 1, 2, 61, 71}
	precip := []int{0, 5, 10, 80, 20}
	isDay := []int{1, 1, 1, 0, 0}

	t.Run("mid-hour current time starts at the hour it falls in", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T14:23", times, temps, codes, precip, isDay)
		require.Len(t, got, 4)
		assert.Equal(t, "2026-10-01T14:00", got[0].Time)
		assert.Equal(t, 71.0, got[0].Temperature)
		assert.Equal(t, "Mostly Clear", got[0].Condition)
		assert.Equal(t, 5, got[0].PrecipitationProbability)
		assert.True(t, got[0].IsDay)
	})

	t.Run("exact hour match", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T15:00", times, temps, codes, precip, isDay)
		require.Len(t, got, 3)
		assert.Equal(t, "2026-10-01T15:00", got[0].Time)
	})

	t.Run("truncates to the available data instead of panicking", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T16:00", times, temps, codes, precip, isDay)
		require.Len(t, got, 2)
		assert.Equal(t, "2026-10-01T17:00", got[1].Time)
		assert.False(t, got[0].IsDay, "16:00 is marked night in the fixture data")
	})

	t.Run("current time after all hourly data returns nothing", func(t *testing.T) {
		got := hourlyForecast("2026-10-02T00:00", times, temps, codes, precip, isDay)
		assert.Empty(t, got)
	})

	t.Run("malformed current time returns nothing", func(t *testing.T) {
		got := hourlyForecast("bad-time", times, temps, codes, precip, isDay)
		assert.Empty(t, got)
	})

	t.Run("missing precipitation and is_day entries default instead of panicking", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T13:00", times, temps, codes, nil, nil)
		require.Len(t, got, 5)
		assert.Equal(t, 0, got[0].PrecipitationProbability)
		assert.True(t, got[0].IsDay, "missing is_day should default to day, not night")
	})
}

func TestNextSunEvent(t *testing.T) {
	current := parseOpenMeteoTime(t, "2026-10-01T16:00")
	windowEnd := current.Add(24 * time.Hour)
	times := []string{"2026-10-01T07:29", "2026-10-02T07:30"}

	t.Run("skips an event already in the past", func(t *testing.T) {
		got := nextSunEvent(times, current, windowEnd)
		require.NotNil(t, got)
		assert.Equal(t, "2026-10-02T07:30", *got, "today's already passed, so this should be tomorrow's")
	})

	t.Run("excludes an event past the window", func(t *testing.T) {
		got := nextSunEvent([]string{"2026-10-03T07:30"}, current, windowEnd)
		assert.Nil(t, got)
	})

	t.Run("skips malformed entries instead of erroring", func(t *testing.T) {
		got := nextSunEvent([]string{"not-a-time", "2026-10-01T18:00"}, current, windowEnd)
		require.NotNil(t, got)
		assert.Equal(t, "2026-10-01T18:00", *got)
	})

	t.Run("no events found", func(t *testing.T) {
		got := nextSunEvent(nil, current, windowEnd)
		assert.Nil(t, got)
	})
}

func parseOpenMeteoTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(openMeteoTimeLayout, s)
	require.NoError(t, err)
	return parsed
}
