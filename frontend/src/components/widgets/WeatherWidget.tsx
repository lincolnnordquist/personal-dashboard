import { useQuery } from '@apollo/client/react'
import { GET_WEATHER, type TemperatureUnit } from '../../graphql/queries'
import type { WidgetProps } from '../Grid'
import WidgetCard from '../WidgetCard'

const REFRESH_MS = 5 * 60 * 1000

export default function WeatherWidget({ config }: WidgetProps) {
  const lat = Number(config.lat)
  const lon = Number(config.lon)
  const unit: TemperatureUnit = config.unit === 'C' ? 'C' : 'F'
  const location = typeof config.location === 'string' ? config.location : undefined

  const { data, loading, error } = useQuery(GET_WEATHER, {
    variables: { lat, lon, unit, location },
    pollInterval: REFRESH_MS,
  })
  const weather = data?.weatherData

  return (
    <WidgetCard title="Weather">
      {loading && !weather && <p className="muted">Loading…</p>}
      {error && <p className="error">{error.message}</p>}
      {weather && (
        <div className="weather">
          <div className="weather-temp">
            {Math.round(weather.temperature)}°{unit}
          </div>
          <div className="weather-condition">{weather.condition}</div>
          <div className="weather-range muted">
            H {Math.round(weather.high)}° · L {Math.round(weather.low)}°
          </div>
          <div className="weather-location muted">{weather.location || `${lat}, ${lon}`}</div>
        </div>
      )}
    </WidgetCard>
  )
}
