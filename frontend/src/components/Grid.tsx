import { type ComponentType, useEffect, useRef, useState } from 'react'
import { useMutation } from '@apollo/client/react'
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

function columnOf(layout: Layout, id: number): Column | null {
  for (const column of COLUMNS) if (layout[column].includes(id)) return column
  return null
}

// Three columns, Glance-style: narrow sides for small widgets, a wide center for feeds. The
// "rearrange" button puts the grid into edit mode: click a widget to select it, then the arrow
// keys move it -- up/down reorders within its column, left/right sends it to the next column
// over, roughly preserving its vertical position. Esc or the same button leaves edit mode.
//
// This replaced an earlier pointer-drag-and-drop implementation (@dnd-kit), which kept crashing:
// dragging across a column boundary moved the widget between columns' arrays live, so React
// unmounted it from the source column and remounted it in the destination mid-drag. @dnd-kit
// tracks the dragged node with a MutationObserver on the whole document body to keep its own
// measurements current, and that remount could retrigger a state update that mutated the DOM
// again -- a genuine infinite loop inside the library (confirmed with a scripted Playwright
// repro against a dev build; the crash traced into @dnd-kit's own measureRect, not this
// component). Even after fixing that specific loop, releasing a drag didn't reliably commit the
// move. Keyboard-driven moves have no continuous pointer tracking and no mid-gesture DOM churn
// for anything to race against, so this entire class of bug doesn't apply.
export default function Grid({ widgets }: { widgets: WidgetConfig[] }) {
  const [moveWidget] = useMutation(MOVE_WIDGET)
  const [layout, setLayout] = useState(() => layoutFromWidgets(widgets))
  const [editing, setEditing] = useState(false)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const byId = new Map(widgets.map((w) => [w.id, w]))

  // Mirrors `layout`, updated synchronously (unlike state, which only takes effect on the next
  // render) so a fast run of arrow-key repeats always builds on the true current layout, never a
  // stale snapshot from before an earlier key's update had applied.
  const layoutRef = useRef(layout)
  function applyLayout(next: Layout) {
    layoutRef.current = next
    setLayout(next)
  }

  // Stay in sync with the server (e.g. a widget toggled elsewhere), but not while a widget is
  // selected, which would fight the optimistic reorder below.
  useEffect(() => {
    if (selectedId === null) applyLayout(layoutFromWidgets(widgets))
  }, [widgets, selectedId])

  useEffect(() => {
    if (!editing) setSelectedId(null)
  }, [editing])

  useEffect(() => {
    // Applies a full next layout, then persists the moved widget's new column/position.
    function commit(id: number, next: Layout, column: Column) {
      applyLayout(next)
      const position = next[column].indexOf(id)
      moveWidget({ variables: { id, column, position } }).catch(() => {
        applyLayout(layoutFromWidgets(widgets))
      })
    }

    const onKey = (e: KeyboardEvent) => {
      if (shortcutsBlocked(e)) return
      if (e.key === 'Escape' && editing) {
        setEditing(false)
        return
      }
      if (!editing || selectedId === null) return
      if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(e.key)) return
      e.preventDefault()

      const id = selectedId
      const prev = layoutRef.current
      const column = columnOf(prev, id)
      if (!column) return
      const items = prev[column]
      const index = items.indexOf(id)

      if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
        const swapWith = e.key === 'ArrowUp' ? index - 1 : index + 1
        if (swapWith < 0 || swapWith >= items.length) return
        const reordered = [...items]
        ;[reordered[index], reordered[swapWith]] = [reordered[swapWith], reordered[index]]
        commit(id, { ...prev, [column]: reordered }, column)
        return
      }

      const columnIndex = COLUMNS.indexOf(column)
      const targetColumn = COLUMNS[e.key === 'ArrowLeft' ? columnIndex - 1 : columnIndex + 1]
      if (!targetColumn) return
      const destBase = prev[targetColumn]
      const insertAt = Math.min(index, destBase.length)
      const destItems = [...destBase.slice(0, insertAt), id, ...destBase.slice(insertAt)]
      commit(id, { ...prev, [column]: items.filter((i) => i !== id), [targetColumn]: destItems }, targetColumn)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [editing, selectedId, widgets, moveWidget])

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
      <div className="columns">
        {COLUMNS.map((column) => (
          <div key={column} className={`column column-${column}`}>
            {layout[column].map((id) => {
              const w = byId.get(id)
              if (!w) return null
              const Widget = widgetComponents[w.widgetType]
              const selected = editing && selectedId === id
              const className = editing ? (selected ? 'editable-widget selected' : 'editable-widget') : undefined
              return (
                <div
                  key={id}
                  className={className}
                  onClick={editing ? () => setSelectedId((cur) => (cur === id ? null : id)) : undefined}
                >
                  {Widget ? (
                    <Widget id={w.id} config={w.config} />
                  ) : (
                    <WidgetCard title={w.widgetType}>
                      <p className="muted">Coming soon</p>
                    </WidgetCard>
                  )}
                </div>
              )
            })}
            {editing && layout[column].length === 0 && <div className="column-empty-hint">Empty</div>}
          </div>
        ))}
      </div>
    </>
  )
}
