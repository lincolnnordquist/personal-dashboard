import { useEffect, useRef, useState } from 'react'
import { useMutation } from '@apollo/client/react'
import { UPDATE_WIDGET_CONFIG } from '../../graphql/queries'
import type { WidgetProps } from '../Grid'
import WidgetCard from '../WidgetCard'

const SAVE_DELAY_MS = 600

// A single scratchpad textarea, autosaved into the widget's own config. Local-only: it is
// never synced between machines.
export default function NotesWidget({ id, config }: WidgetProps) {
  const saved = typeof config.content === 'string' ? config.content : ''
  const [text, setText] = useState(saved)
  const [updateConfig] = useMutation(UPDATE_WIDGET_CONFIG)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const lastSaved = useRef(saved)

  // The config prop updates once our own save round-trips; don't let that clobber active typing.
  useEffect(() => {
    if (saved !== lastSaved.current) {
      lastSaved.current = saved
      setText(saved)
    }
  }, [saved])

  useEffect(() => () => clearTimeout(timer.current), [])

  function save(value: string) {
    clearTimeout(timer.current)
    lastSaved.current = value
    updateConfig({ variables: { id, config: { content: value } } })
  }

  function onChange(value: string) {
    setText(value)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => save(value), SAVE_DELAY_MS)
  }

  return (
    <WidgetCard title="Notes">
      <textarea
        className="notes-textarea"
        value={text}
        onChange={(e) => onChange(e.target.value)}
        onBlur={() => text !== lastSaved.current && save(text)}
        placeholder="Jot something down…"
        spellCheck={false}
      />
    </WidgetCard>
  )
}
