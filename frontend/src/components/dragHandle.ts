import { createContext, useContext } from 'react'
import type { DraggableAttributes, DraggableSyntheticListeners } from '@dnd-kit/core'

export interface DragHandle {
  attributes: DraggableAttributes
  listeners: DraggableSyntheticListeners
  setActivatorNodeRef: (node: HTMLElement | null) => void
}

// Set by Grid around each widget during a drag-and-drop reorder; read by WidgetCard to render
// the handle icon in the card's corner, wired to start the drag from there (not the whole card).
export const DragHandleContext = createContext<DragHandle | null>(null)

export function useDragHandle() {
  return useContext(DragHandleContext)
}
