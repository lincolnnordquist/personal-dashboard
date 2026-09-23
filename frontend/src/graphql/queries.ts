import { gql, type TypedDocumentNode } from '@apollo/client'

export interface WidgetConfig {
  id: number
  widgetType: string
  config: Record<string, unknown>
  position: number
  enabled: boolean
}

export const GET_WIDGETS: TypedDocumentNode<{ widgets: WidgetConfig[] }> = gql`
  query GetWidgets {
    widgets {
      id
      widgetType
      config
      position
      enabled
    }
  }
`

export type TemperatureUnit = 'F' | 'C'

export interface WeatherData {
  temperature: number
  condition: string
  high: number
  low: number
  location: string
}

export const GET_WEATHER: TypedDocumentNode<
  { weatherData: WeatherData | null },
  { lat: number; lon: number; unit: TemperatureUnit; location?: string }
> = gql`
  query GetWeather($lat: Float!, $lon: Float!, $unit: TemperatureUnit, $location: String) {
    weatherData(lat: $lat, lon: $lon, unit: $unit, location: $location) {
      temperature
      condition
      high
      low
      location
    }
  }
`
