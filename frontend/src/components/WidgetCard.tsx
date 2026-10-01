import type { ReactNode } from 'react'

// A widget: a section label above a translucent card. Pass `header` to replace the plain
// title, e.g. with label-style tabs.
export default function WidgetCard({
  title,
  header,
  children,
}: {
  title?: string
  header?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="widget-section">
      <div className="widget-heading">{header ?? <h2 className="widget-title">{title}</h2>}</div>
      <div className="widget">{children}</div>
    </section>
  )
}
