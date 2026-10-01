package widgets

import (
	"testing"

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
}

func TestHourlyForecast(t *testing.T) {
	times := []string{
		"2026-10-01T13:00", "2026-10-01T14:00", "2026-10-01T15:00", "2026-10-01T16:00", "2026-10-01T17:00",
	}
	temps := []float64{70, 71, 72, 73, 74}
	codes := []int{0, 1, 2, 61, 71}
	precip := []int{0, 5, 10, 80, 20}

	t.Run("mid-hour current time starts at the hour it falls in", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T14:23", times, temps, codes, precip)
		require.Len(t, got, 4)
		assert.Equal(t, "2026-10-01T14:00", got[0].Time)
		assert.Equal(t, 71.0, got[0].Temperature)
		assert.Equal(t, "Mostly Clear", got[0].Condition)
		assert.Equal(t, 5, got[0].PrecipitationProbability)
	})

	t.Run("exact hour match", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T15:00", times, temps, codes, precip)
		require.Len(t, got, 3)
		assert.Equal(t, "2026-10-01T15:00", got[0].Time)
	})

	t.Run("truncates to the available data instead of panicking", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T16:00", times, temps, codes, precip)
		require.Len(t, got, 2)
		assert.Equal(t, "2026-10-01T17:00", got[1].Time)
	})

	t.Run("current time after all hourly data returns nothing", func(t *testing.T) {
		got := hourlyForecast("2026-10-02T00:00", times, temps, codes, precip)
		assert.Empty(t, got)
	})

	t.Run("malformed current time returns nothing", func(t *testing.T) {
		got := hourlyForecast("bad-time", times, temps, codes, precip)
		assert.Empty(t, got)
	})

	t.Run("missing precipitation entries default to zero instead of panicking", func(t *testing.T) {
		got := hourlyForecast("2026-10-01T13:00", times, temps, codes, nil)
		require.Len(t, got, 5)
		assert.Equal(t, 0, got[0].PrecipitationProbability)
	})
}
