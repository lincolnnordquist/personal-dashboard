import { type ComponentType, type CSSProperties, type ReactNode, useEffect, useRef, useState } from 'react'
import { useMutation } from '@apollo/client/react'
import {
  closestCorners,
  DndContext,
  DragOverlay,
  PointerSensor,
  TouchSensor,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { SortableContext, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
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

// Hoisted so these are referentially stable across renders. useSensor/useSensors only memoize
// when their inputs are (`[sensor, options]` / the sensor list), so passing a fresh object
// literal here every render defeated that memoization, making `sensors` (and everything
// @dnd-kit keys off it internally) a new array on every single render.
const POINTER_SENSOR_OPTIONS = { activationConstraint: { distance: 4 } }
const TOUCH_SENSOR_OPTIONS = { activationConstraint: { delay: 150, tolerance: 5 } }

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

  // Mirrors `layout`, updated synchronously (unlike state, which only takes effect on the next
  // render) so handleDragEnd always reads the true current layout, never a stale snapshot from
  // before an earlier update in the same event batch had applied.
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
    useSensor(PointerSensor, POINTER_SENSOR_OPTIONS),
    useSensor(TouchSensor, TOUCH_SENSOR_OPTIONS),
  )

  function handleDragStart(event: DragStartEvent) {
    setActiveId(Number(event.active.id))
  }

  // The layout only changes once, here at drop -- not live during onDragOver. An earlier version
  // moved the item between columns' arrays on every onDragOver, so dragging across a column
  // boundary meant React unmounting it from the source column and remounting it in the
  // destination mid-drag. That's a real DOM mutation, and @dnd-kit's own internal rect tracking
  // (`useRect`, used for the dragged node and its container) watches the whole document body with
  // a MutationObserver to stay current -- the remount reliably retriggered it, and the resulting
  // state update could trigger another commit that mutated the DOM again, which is the actual
  // "Maximum update depth exceeded" crash (confirmed with a scripted repro against a dev build:
  // the loop was entirely inside @dnd-kit's measureRect, not in this component's own effects).
  // Settling the move only at drop means the real DOM never changes until the gesture is over;
  // @dnd-kit's own SortableContext strategy still animates same-column reordering live, since
  // that's done with CSS transforms, not by us touching the array mid-drag.
  function handleDragEnd(event: DragEndEvent) {
    setActiveId(null)
    const { active, over } = event
    if (!over) return
    const activeId = Number(active.id)
    const prev = layoutRef.current
    const fromColumn = columnOf(prev, activeId)
    const toColumn = columnOf(prev, over.id)
    if (!fromColumn || !toColumn) return

    const destBase = prev[toColumn].filter((id) => id !== activeId)
    const overIndex = isColumn(over.id) ? destBase.length : destBase.indexOf(Number(over.id))
    const insertAt = overIndex === -1 ? destBase.length : overIndex
    const destItems = [...destBase.slice(0, insertAt), activeId, ...destBase.slice(insertAt)]

    const next: Layout = { ...prev, [toColumn]: destItems }
    if (fromColumn !== toColumn) {
      next[fromColumn] = prev[fromColumn].filter((id) => id !== activeId)
    }
    applyLayout(next)

    const position = destItems.indexOf(activeId)
    moveWidget({ variables: { id: activeId, column: toColumn, position } }).catch(() => {
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
        onDragStart={handleDragStart}
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
