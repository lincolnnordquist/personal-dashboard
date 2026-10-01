import type { ReactNode } from 'react'
import { useDragHandle } from './dragHandle'
import { DragHandleIcon } from './icons'

// A widget: a section label above a translucent card. Pass `header` to replace the plain
// title, e.g. with label-style tabs. When Grid is in a drag-and-drop reorder, a handle icon is
// rendered in the card's top-right corner, wired to start the drag (the rest of the card isn't
// draggable, so clicks and scrolling inside it work as normal).
export default function WidgetCard({
  title,
  header,
  children,
}: {
  title?: string
  header?: ReactNode
  children: ReactNode
}) {
  const handle = useDragHandle()
  return (
    <section className="widget-section">
      <div className="widget-heading">{header ?? <h2 className="widget-title">{title}</h2>}</div>
      {handle && (
        <button
          ref={handle.setActivatorNodeRef}
          className="icon-button heading-button drag-handle"
          aria-label="Drag to move this widget"
          title="Drag to move"
          {...handle.attributes}
          {...handle.listeners}
        >
          <DragHandleIcon />
        </button>
      )}
      <div className="widget">{children}</div>
    </section>
  )
}
