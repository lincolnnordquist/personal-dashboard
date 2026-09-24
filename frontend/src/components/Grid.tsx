import type { ComponentType } from 'react'
import type { WidgetConfig } from '../graphql/queries'
import WidgetCard from './WidgetCard'
import CalendarWidget from './widgets/CalendarWidget'
import DockerWidget from './widgets/DockerWidget'
import RedditWidget from './widgets/RedditWidget'
import SportsWidget from './widgets/SportsWidget'
import WeatherWidget from './widgets/WeatherWidget'
import YouTubeWidget from './widgets/YouTubeWidget'

export interface WidgetProps {
  config: Record<string, unknown>
}

// Widget types without an entry here render a placeholder until they are built.
const widgetComponents: Record<string, ComponentType<WidgetProps>> = {
  calendar: CalendarWidget,
  weather: WeatherWidget,
  reddit: RedditWidget,
  sports: SportsWidget,
  docker: DockerWidget,
  youtube: YouTubeWidget,
}

const COLUMNS = ['left', 'center', 'right'] as const

// Three columns, Glance-style: narrow sides for small widgets, a wide center for feeds.
export default function Grid({ widgets }: { widgets: WidgetConfig[] }) {
  return (
    <div className="columns">
      {COLUMNS.map((column) => (
        <div key={column} className={`column column-${column}`}>
          {widgets
            .filter((w) => w.column === column)
            .sort((a, b) => a.position - b.position)
            .map((w) => {
              const Widget = widgetComponents[w.widgetType]
              return Widget ? (
                <Widget key={w.id} config={w.config} />
              ) : (
                <WidgetCard key={w.id} title={w.widgetType}>
                  <p className="muted">Coming soon</p>
                </WidgetCard>
              )
            })}
        </div>
      ))}
    </div>
  )
}
