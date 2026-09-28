// shortcutsBlocked reports whether a single-key shortcut should be ignored: while typing in a
// field, while a dialog (e.g. the music library) is open, or with Ctrl/Alt/Cmd held so browser
// shortcuts keep working. Shift is allowed, for shortcuts like Shift+B.
export function shortcutsBlocked(e: KeyboardEvent): boolean {
  const target = e.target as HTMLElement
  const typing = target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
  const dialogOpen = document.querySelector('[role="dialog"]') !== null
  return typing || dialogOpen || e.ctrlKey || e.metaKey || e.altKey
}
