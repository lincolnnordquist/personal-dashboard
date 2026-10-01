// Package widgets fetches and parses data from each widget's external source.
package widgets

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	WeatherWidgetType = "weather"
	WeatherTTL        = 30 * time.Minute
)

// WeatherData is bound to the GraphQL WeatherData type.
type WeatherData struct {
	Temperature float64         `json:"temperature"`
	Condition   string          `json:"condition"`
	High        float64         `json:"high"`
	Low         float64         `json:"low"`
	Location    string          `json:"location"`
	Hourly      []HourlyWeather `json:"hourly"`
}

// HourlyWeather is bound to the GraphQL HourlyWeather type.
type HourlyWeather struct {
	Time                     string  `json:"time"`
	Temperature              float64 `json:"temperature"`
	Condition                string  `json:"condition"`
	PrecipitationProbability int     `json:"precipitationProbability"`
}

// hourlyWindowHours is how many hours of forecast to keep, matching the Apple Weather app's
// hourly strip.
const hourlyWindowHours = 24

type WeatherClient struct {
	HTTP    *http.Client
	BaseURL string
}

func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		BaseURL: "https://api.open-meteo.com/v1/forecast",
	}
}

// WeatherConfig is the widget_config.config shape for a weather widget.
type WeatherConfig struct {
	Lat      float64
	Lon      float64
	Unit     string
	Location string
}

// ParseWeatherConfig reads a weather widget's stored config. Unit defaults to "F".
func ParseWeatherConfig(cfg map[string]any) (WeatherConfig, error) {
	lat, latOK := cfg["lat"].(float64)
	lon, lonOK := cfg["lon"].(float64)
	if !latOK || !lonOK {
		return WeatherConfig{}, fmt.Errorf("weather config needs numeric lat and lon")
	}
	wc := WeatherConfig{Lat: lat, Lon: lon, Unit: "F"}
	if u, _ := cfg["unit"].(string); u == "C" {
		wc.Unit = "C"
	}
	wc.Location, _ = cfg["location"].(string)
	return wc, nil
}

// WeatherCacheKey identifies a location and unit, e.g. "weather_37.0965_-113.5684_F".
func WeatherCacheKey(lat, lon float64, unit string) string {
	return fmt.Sprintf("weather_%.4f_%.4f_%s", lat, lon, unit)
}

// Fetch returns current conditions for a location. unit is "F" or "C".
// Location is left empty; the caller fills in the display name.
func (c *WeatherClient) Fetch(ctx context.Context, lat, lon float64, unit string) (*WeatherData, error) {
	tempUnit := "fahrenheit"
	if unit == "C" {
		tempUnit = "celsius"
	}
	q := url.Values{
		"latitude":         {strconv.FormatFloat(lat, 'f', -1, 64)},
		"longitude":        {strconv.FormatFloat(lon, 'f', -1, 64)},
		"current":          {"temperature_2m,weather_code"},
		"hourly":           {"temperature_2m,weather_code,precipitation_probability"},
		"daily":            {"temperature_2m_max,temperature_2m_min"},
		"temperature_unit": {tempUnit},
		"timezone":         {"auto"},
		// 2 days, not 1: the hourly window needs 24 hours *from the current hour*, which runs
		// past midnight unless it's queried very early in the day.
		"forecast_days": {"2"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("open-meteo request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("open-meteo read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo: status %d: %s", resp.StatusCode, body)
	}
	return ParseWeather(body)
}

type openMeteoResponse struct {
	Current struct {
		Time        *string  `json:"time"`
		Temperature *float64 `json:"temperature_2m"`
		WeatherCode *int     `json:"weather_code"`
	} `json:"current"`
	Hourly struct {
		Time                     []string  `json:"time"`
		Temperature              []float64 `json:"temperature_2m"`
		WeatherCode              []int     `json:"weather_code"`
		PrecipitationProbability []int     `json:"precipitation_probability"`
	} `json:"hourly"`
	Daily struct {
		Max []float64 `json:"temperature_2m_max"`
		Min []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

// ParseWeather converts an Open-Meteo forecast response into WeatherData.
func ParseWeather(body []byte) (*WeatherData, error) {
	var r openMeteoResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("decode open-meteo response: %w", err)
	}
	if r.Current.Temperature == nil || r.Current.WeatherCode == nil || r.Current.Time == nil {
		return nil, fmt.Errorf("open-meteo response missing current conditions")
	}
	if len(r.Daily.Max) == 0 || len(r.Daily.Min) == 0 {
		return nil, fmt.Errorf("open-meteo response missing daily high/low")
	}
	return &WeatherData{
		Temperature: *r.Current.Temperature,
		Condition:   WeatherCondition(*r.Current.WeatherCode),
		High:        r.Daily.Max[0],
		Low:         r.Daily.Min[0],
		Hourly: hourlyForecast(
			*r.Current.Time,
			r.Hourly.Time, r.Hourly.Temperature, r.Hourly.WeatherCode, r.Hourly.PrecipitationProbability,
		),
	}, nil
}

// hourlyForecast finds the hourly entry matching the current hour and returns it plus the
// following hours, up to hourlyWindowHours. Comparing the "YYYY-MM-DDTHH" prefix (rather than
// full equality) is what lines the first entry up with "now": Open-Meteo's current.time carries
// minutes (e.g. 14:23), but hourly.time entries always land on the hour (14:00).
func hourlyForecast(currentTime string, times []string, temps []float64, codes []int, precipitation []int) []HourlyWeather {
	if len(currentTime) < 13 {
		return nil
	}
	currentHour := currentTime[:13]

	start := -1
	for i, t := range times {
		if len(t) >= 13 && t[:13] >= currentHour {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}

	end := min(start+hourlyWindowHours, len(times), len(temps), len(codes))
	hourly := make([]HourlyWeather, 0, end-start)
	for i := start; i < end; i++ {
		precipProbability := 0
		if i < len(precipitation) {
			precipProbability = precipitation[i]
		}
		hourly = append(hourly, HourlyWeather{
			Time:                     times[i],
			Temperature:              temps[i],
			Condition:                WeatherCondition(codes[i]),
			PrecipitationProbability: precipProbability,
		})
	}
	return hourly
}

// WeatherCondition maps a WMO weather code to a short description.
// See the "WMO Weather interpretation codes" table at https://open-meteo.com/en/docs.
func WeatherCondition(code int) string {
	switch code {
	case 0:
		return "Clear"
	case 1:
		return "Mostly Clear"
	case 2:
		return "Partly Cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53, 55:
		return "Drizzle"
	case 56, 57:
		return "Freezing Drizzle"
	case 61, 63, 65:
		return "Rain"
	case 66, 67:
		return "Freezing Rain"
	case 71, 73, 75, 77:
		return "Snow"
	case 80, 81, 82:
		return "Rain Showers"
	case 85, 86:
		return "Snow Showers"
	case 95:
		return "Thunderstorm"
	case 96, 99:
		return "Thunderstorm with Hail"
	default:
		return "Unknown"
	}
}
