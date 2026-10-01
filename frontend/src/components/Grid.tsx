import { type ComponentType, type CSSProperties, type ReactNode, useEffect, useRef, useState } from 'react'
import { useMutation } from '@apollo/client/react'
import {
  closestCorners,
  DndContext,
  DragOverlay,
  MeasuringStrategy,
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
import { shortcutsBlocked } from '../lib/keyboard'
import { CloseIcon, DragHandleIcon } from './icons'
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

function isColumn(id: string | number): id is Column {
  return (COLUMNS as readonly (string | number)[]).includes(id)
}

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
  if (isColumn(id)) return id
  for (const column of COLUMNS) if (layout[column].includes(Number(id))) return column
  return null
}

// Three columns, Glance-style: narrow sides for small widgets, a wide center for feeds. The
// "rearrange" button puts the grid into edit mode: every card wiggles and can be dragged by
// grabbing anywhere on it, to any position in any column, like rearranging iPhone apps. Esc or
// the same button leaves edit mode.
export default function Grid({ widgets }: { widgets: WidgetConfig[] }) {
  const [moveWidget] = useMutation(MOVE_WIDGET)
  const [layout, setLayout] = useState(() => layoutFromWidgets(widgets))
  const [activeId, setActiveId] = useState<number | null>(null)
  const [editing, setEditing] = useState(false)
  const byId = new Map(widgets.map((w) => [w.id, w]))

  // The source of truth while dragging. onDragOver and onDragEnd can both fire, sometimes more
  // than once each, before React commits a single re-render -- reading `layout` (state) directly
  // in those handlers meant some of those calls saw a stale snapshot from before an earlier call
  // in the same batch had applied. A wide, item-dense column (the center one) generates far more
  // of these events per drag, which is why that was the one that kept crashing. The ref is
  // mutated synchronously, so every handler call sees exactly what the last one left behind.
  const layoutRef = useRef(layout)
  function applyLayout(next: Layout) {
    layoutRef.current = next
    setLayout(next)
  }

  // Stay in sync with the server (e.g. a widget toggled elsewhere) except mid-drag, where this
  // would fight the optimistic reorder below.
  useEffect(() => {
    if (activeId === null) applyLayout(layoutFromWidgets(widgets))
  }, [widgets, activeId])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (shortcutsBlocked(e)) return
      if (e.key === 'Escape' && editing) setEditing(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [editing])

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
    const overId = over.id

    const prev = layoutRef.current
    const fromColumn = columnOf(prev, activeId)
    const toColumn = columnOf(prev, overId)
    if (!fromColumn || !toColumn || fromColumn === toColumn) return

    const from = prev[fromColumn].filter((id) => id !== activeId)
    const to = [...prev[toColumn]]
    const overIndex = to.indexOf(Number(overId))
    to.splice(overIndex >= 0 ? overIndex : to.length, 0, activeId)
    applyLayout({ ...prev, [fromColumn]: from, [toColumn]: to })
  }

  function handleDragEnd(event: DragEndEvent) {
    setActiveId(null)
    const { active, over } = event
    if (!over) return
    const activeId = Number(active.id)
    // The item is already in its destination column's array either way (onDragOver put it there
    // for a cross-column move, and it was there all along for a same-column reorder).
    const prev = layoutRef.current
    const column = columnOf(prev, over.id)
    if (!column) return

    const items = prev[column]
    const oldIndex = items.indexOf(activeId)
    if (oldIndex === -1) return
    const newIndex = isColumn(over.id) ? items.length - 1 : items.indexOf(Number(over.id))
    const finalItems = newIndex === -1 || newIndex === oldIndex ? items : arrayMove(items, oldIndex, newIndex)
    if (finalItems !== items) {
      applyLayout({ ...prev, [column]: finalItems })
    }

    const position = finalItems.indexOf(activeId)
    moveWidget({ variables: { id: activeId, column, position } }).catch(() => {
      applyLayout(layoutFromWidgets(widgets))
    })
  }

  const activeWidget = activeId !== null ? byId.get(activeId) : undefined

  return (
    <>
      <button
        className={editing ? 'rearrange-toggle active' : 'rearrange-toggle'}
        onClick={() => setEditing((v) => !v)}
        aria-pressed={editing}
        aria-label={editing ? 'Done rearranging (Esc)' : 'Rearrange widgets'}
        title={editing ? 'Done rearranging (Esc)' : 'Rearrange widgets'}
      >
        {editing ? <CloseIcon /> : <DragHandleIcon />}
      </button>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCorners}
        // The center column's widgets (Reddit, YouTube, Sports) load their content
        // asynchronously and can resize mid-drag; re-measuring continuously instead of once at
        // drag start keeps collision detection accurate if that happens.
        measuring={{ droppable: { strategy: MeasuringStrategy.Always } }}
        onDragStart={handleDragStart}
        onDragOver={handleDragOver}
        onDragEnd={handleDragEnd}
      >
        <div className={editing ? 'columns columns-editing' : 'columns'}>
          {COLUMNS.map((column) => (
            <DroppableColumn key={column} id={column} showHint={editing && layout[column].length === 0}>
              <SortableContext items={layout[column]} strategy={verticalListSortingStrategy}>
                {layout[column].map((id) => {
                  const w = byId.get(id)
                  if (!w) return null
                  const Widget = widgetComponents[w.widgetType]
                  return (
                    <SortableWidget key={id} id={id} editing={editing}>
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
    </>
  )
}

function DroppableColumn({
  id,
  showHint,
  children,
}: {
  id: Column
  showHint: boolean
  children: ReactNode
}) {
  const { setNodeRef } = useDroppable({ id })
  return (
    <div ref={setNodeRef} className={`column column-${id}`}>
      {children}
      {showHint && <div className="column-drop-hint">Drop a widget here</div>}
    </div>
  )
}

function SortableWidget({ id, editing, children }: { id: number; editing: boolean; children: ReactNode }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id,
    disabled: !editing,
  })
  const style: CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  }
  return (
    <div
      ref={setNodeRef}
      style={style}
      className={isDragging ? 'sortable-widget dragging' : 'sortable-widget'}
      {...(editing ? attributes : {})}
      {...(editing ? listeners : {})}
    >
      {children}
    </div>
  )
}
