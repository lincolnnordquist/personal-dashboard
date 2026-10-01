import { useQuery } from '@apollo/client/react'
import { GET_WEATHER, type HourlyWeather, type TemperatureUnit } from '../../graphql/queries'
import type { WidgetProps } from '../Grid'
import WidgetCard from '../WidgetCard'
import WeatherIcon, { SunHorizonIcon } from './WeatherIcon'

const REFRESH_MS = 5 * 60 * 1000

const hourFormat = new Intl.DateTimeFormat(undefined, { hour: 'numeric' })
const sunEventTimeFormat = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit' })

type HourEntry = { kind: 'hour'; hour: HourlyWeather } | { kind: 'sunrise' | 'sunset'; time: string }

// Interleaves the sunrise/sunset markers into the hourly list in time order, like Apple Weather's
// hourly strip does -- inserted as their own tile, right before the first hour that comes after
// them (hourly entries land on the hour; sun events usually don't).
function buildEntries(hourly: HourlyWeather[], sunrise: string | null, sunset: string | null): HourEntry[] {
  const entries: HourEntry[] = hourly.map((hour) => ({ kind: 'hour', hour }))
  const events: { kind: 'sunrise' | 'sunset'; time: string }[] = []
  if (sunrise) events.push({ kind: 'sunrise', time: sunrise })
  if (sunset) events.push({ kind: 'sunset', time: sunset })

  for (const event of events) {
    const eventTime = new Date(event.time).getTime()
    let insertAt = entries.findIndex((e) => e.kind === 'hour' && new Date(e.hour.time).getTime() > eventTime)
    if (insertAt === -1) insertAt = entries.length
    entries.splice(insertAt, 0, event)
  }
  return entries
}

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

          {weather.hourly.length > 0 && (
            <div className="weather-hourly">
              {buildEntries(weather.hourly, weather.sunrise, weather.sunset).map((entry) => {
                if (entry.kind === 'hour') {
                  const h = entry.hour
                  return (
                    <div className="weather-hour" key={h.time}>
                      <div className="weather-hour-label muted">
                        {h.time === weather.hourly[0].time ? 'Now' : hourFormat.format(new Date(h.time))}
                      </div>
                      <WeatherIcon condition={h.condition} isDay={h.isDay} />
                      {h.precipitationProbability > 0 && (
                        <div className="weather-hour-precip">{h.precipitationProbability}%</div>
                      )}
                      <div className="weather-hour-temp">{Math.round(h.temperature)}°</div>
                    </div>
                  )
                }
                return (
                  <div className="weather-hour weather-sun-event" key={entry.kind}>
                    <div className="weather-hour-label muted">{entry.kind === 'sunrise' ? 'Sunrise' : 'Sunset'}</div>
                    <SunHorizonIcon />
                    <div className="weather-hour-temp">{sunEventTimeFormat.format(new Date(entry.time))}</div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}
    </WidgetCard>
  )
}
