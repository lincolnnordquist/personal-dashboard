import type { ReactNode } from 'react'

export default function WidgetCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="widget">
      <h2 className="widget-title">{title}</h2>
      <div className="widget-body">{children}</div>
    </section>
  )
}
