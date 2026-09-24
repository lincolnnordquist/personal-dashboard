import type { ComponentType } from 'react'
import type { WidgetConfig } from '../graphql/queries'
import WidgetCard from './WidgetCard'
import DockerWidget from './widgets/DockerWidget'
import RedditWidget from './widgets/RedditWidget'
import SportsWidget from './widgets/SportsWidget'
import WeatherWidget from './widgets/WeatherWidget'

export interface WidgetProps {
  config: Record<string, unknown>
}

// Widget types without an entry here render a placeholder until they are built.
const widgetComponents: Record<string, ComponentType<WidgetProps>> = {
  weather: WeatherWidget,
  reddit: RedditWidget,
  sports: SportsWidget,
  docker: DockerWidget,
}

export default function Grid({ widgets }: { widgets: WidgetConfig[] }) {
  return (
    <div className="grid">
      {widgets.map((w) => {
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
  )
}
