import { type ComponentType, type CSSProperties, type ReactNode, useEffect, useState } from 'react'
import { useMutation } from '@apollo/client/react'
import {
  closestCenter,
  DndContext,
  DragOverlay,
  PointerSensor,
  TouchSensor,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { arrayMove, SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { MOVE_WIDGET, type WidgetConfig } from '../graphql/queries'
import { DragHandleContext } from './dragHandle'
import WidgetCard from './WidgetCard'
import CalendarWidget from './widgets/CalendarWidget'
import DockerWidget from './widgets/DockerWidget'
import NotesWidget from './widgets/NotesWidget'
import QuickLinksWidget from './widgets/QuickLinksWidget'
import RedditWidget from './widgets/RedditWidget'
import SportsWidget from './widgets/SportsWidget'
import WeatherWidget from './widgets/WeatherWidget'
import YouTubeWidget from './widgets/YouTubeWidget'

export interface WidgetProps {
  id: number
  config: Record<string, unknown>
}

// Widget types without an entry here render a placeholder until they are built.
const widgetComponents: Record<string, ComponentType<WidgetProps>> = {
  quicklinks: QuickLinksWidget,
  calendar: CalendarWidget,
  weather: WeatherWidget,
  reddit: RedditWidget,
  sports: SportsWidget,
  docker: DockerWidget,
  youtube: YouTubeWidget,
  notes: NotesWidget,
}

const WIDGET_LABELS: Record<string, string> = {
  quicklinks: 'Quick Links',
  calendar: 'Calendar',
  weather: 'Weather',
  reddit: 'Reddit',
  sports: 'Sports',
  docker: 'Docker',
  youtube: 'YouTube',
  notes: 'Notes',
}

const COLUMNS = ['left', 'center', 'right'] as const
type Column = (typeof COLUMNS)[number]
type Layout = Record<Column, number[]>

function layoutFromWidgets(widgets: WidgetConfig[]): Layout {
  const layout: Layout = { left: [], center: [], right: [] }
  for (const column of COLUMNS) {
    layout[column] = widgets
      .filter((w) => w.column === column)
      .sort((a, b) => a.position - b.position)
      .map((w) => w.id)
  }
  return layout
}

// Which column (if any) currently holds this id -- an item id, or a column id when it's the
// container itself (an empty column, or dropped in the gap below the last item).
function columnOf(layout: Layout, id: number | string): Column | null {
  if ((COLUMNS as readonly string[]).includes(id as string)) return id as Column
  for (const column of COLUMNS) if (layout[column].includes(Number(id))) return column
  return null
}

// Three columns, Glance-style: narrow sides for small widgets, a wide center for feeds. Each
// widget can be dragged by the handle in its corner to any position in any column; the move is
// applied locally right away and saved to the backend when the drag ends.
export default function Grid({ widgets }: { widgets: WidgetConfig[] }) {
  const [moveWidget] = useMutation(MOVE_WIDGET)
  const [layout, setLayout] = useState(() => layoutFromWidgets(widgets))
  const [activeId, setActiveId] = useState<number | null>(null)
  const byId = new Map(widgets.map((w) => [w.id, w]))

  // Stay in sync with the server (e.g. a widget toggled elsewhere) except mid-drag, where this
  // would fight the optimistic reorder below.
  useEffect(() => {
    if (activeId === null) setLayout(layoutFromWidgets(widgets))
  }, [widgets, activeId])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 150, tolerance: 5 } }),
  )

  function handleDragStart(event: DragStartEvent) {
    setActiveId(Number(event.active.id))
  }

  // Dragging across a column boundary moves the item between the two columns' arrays right
  // away, so the layout visibly reflows as you drag instead of only snapping on drop.
  function handleDragOver(event: DragOverEvent) {
    const { active, over } = event
    if (!over) return
    const activeId = Number(active.id)
    const fromColumn = columnOf(layout, activeId)
    const toColumn = columnOf(layout, over.id)
    if (!fromColumn || !toColumn || fromColumn === toColumn) return

    setLayout((prev) => {
      const from = prev[fromColumn].filter((id) => id !== activeId)
      const to = [...prev[toColumn]]
      const overIndex = to.indexOf(Number(over.id))
      to.splice(overIndex >= 0 ? overIndex : to.length, 0, activeId)
      return { ...prev, [fromColumn]: from, [toColumn]: to }
    })
  }

  function handleDragEnd(event: DragEndEvent) {
    setActiveId(null)
    const { active, over } = event
    if (!over) return
    const activeId = Number(active.id)
    const column = columnOf(layout, over.id)
    if (!column) return

    // A same-column drop still needs its final reorder applied; a cross-column one was already
    // moved live by handleDragOver, so `layout` already has it in the right place.
    let items = layout[column]
    if (columnOf(layout, activeId) === column) {
      const oldIndex = items.indexOf(activeId)
      const newIndex = (COLUMNS as readonly string[]).includes(String(over.id))
        ? items.length - 1
        : items.indexOf(Number(over.id))
      if (oldIndex !== -1 && newIndex !== -1 && oldIndex !== newIndex) {
        items = arrayMove(items, oldIndex, newIndex)
        setLayout((prev) => ({ ...prev, [column]: items }))
      }
    }

    const position = items.indexOf(activeId)
    moveWidget({ variables: { id: activeId, column, position } }).catch(() => {
      setLayout(layoutFromWidgets(widgets))
    })
  }

  const activeWidget = activeId !== null ? byId.get(activeId) : undefined

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragOver={handleDragOver}
      onDragEnd={handleDragEnd}
    >
      <div className="columns">
        {COLUMNS.map((column) => (
          <DroppableColumn key={column} id={column}>
            <SortableContext items={layout[column]} strategy={verticalListSortingStrategy}>
              {layout[column].map((id) => {
                const w = byId.get(id)
                if (!w) return null
                const Widget = widgetComponents[w.widgetType]
                return (
                  <SortableWidget key={id} id={id}>
                    {Widget ? (
                      <Widget id={w.id} config={w.config} />
                    ) : (
                      <WidgetCard title={w.widgetType}>
                        <p className="muted">Coming soon</p>
                      </WidgetCard>
                    )}
                  </SortableWidget>
                )
              })}
            </SortableContext>
          </DroppableColumn>
        ))}
      </div>
      <DragOverlay>
        {activeWidget && (
          <section className="widget-section widget-drag-preview">
            <div className="widget-heading">
              <h2 className="widget-title">{WIDGET_LABELS[activeWidget.widgetType] ?? activeWidget.widgetType}</h2>
            </div>
            <div className="widget" />
          </section>
        )}
      </DragOverlay>
    </DndContext>
  )
}

function DroppableColumn({ id, children }: { id: Column; children: ReactNode }) {
  const { setNodeRef } = useDroppable({ id })
  return (
    <div ref={setNodeRef} className={`column column-${id}`}>
      {children}
    </div>
  )
}

function SortableWidget({ id, children }: { id: number; children: ReactNode }) {
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id,
  })
  const style: CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  }
  return (
    <div ref={setNodeRef} style={style}>
      <DragHandleContext.Provider value={{ attributes, listeners, setActivatorNodeRef }}>
        {children}
      </DragHandleContext.Provider>
    </div>
  )
}
