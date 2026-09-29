import { useMemo, useState } from 'react'
import { useQuery } from '@apollo/client/react'
import { GET_BACKGROUND_THEMES, GET_WIDGETS } from './graphql/queries'
import Background, { type Scene } from './components/Background'
import { FocusClock, FocusToggle } from './components/FocusMode'
import { useFocusMode } from './components/useFocusMode'
import { useThemeFont } from './components/useThemeFont'
import Grid from './components/Grid'
import MusicPlayer from './components/music/MusicPlayer'
import SearchBar from './components/SearchBar'
import { greeting } from './format'

export default function App() {
  const { data, loading, error } = useQuery(GET_WIDGETS)
  const { data: themeData } = useQuery(GET_BACKGROUND_THEMES)

  // The selected playlist's background theme, reported by the music player once playlists
  // load. undefined means not known yet; null mixes all themes.
  const [theme, setTheme] = useState<string | null | undefined>(undefined)
  const scenes = useMemo((): Scene[] | null => {
    const themes = themeData?.backgroundThemes
    if (!themes || theme === undefined) return null
    // A theme folder missing on this machine falls back to the mix too.
    const chosen = themes.filter((t) => t.name === theme)
    return (chosen.length > 0 ? chosen : themes).flatMap((t) => t.videos)
  }, [themeData, theme])

  // The theme's font applies to the whole page; the mix of all themes has none.
  const themeInfo = themeData?.backgroundThemes.find((t) => t.name === theme)
  useThemeFont(themeInfo?.name ?? null, themeInfo?.fontUrl ?? null)

  const focus = useFocusMode()
  const modeClass = [focus.focused && 'focus-mode', focus.idle && 'focus-idle'].filter(Boolean).join(' ')

  return (
    <div className={modeClass || undefined}>
      <Background scenes={scenes} />
      {/* Hidden in focus mode; inert keeps its links and inputs out of the tab order. */}
      <main className="app" inert={focus.focused}>
        <SearchBar />
        <header className="app-header">
          <h1 className="greeting">{greeting('Lincoln')}</h1>
        </header>
        {loading && <p className="muted">Loading widgets…</p>}
        {error && <p className="error">Could not load widgets: {error.message}</p>}
        {data && <Grid widgets={data.widgets.filter((w) => w.enabled)} />}
      </main>
      <FocusToggle focused={focus.focused} onToggle={focus.toggle} />
      {/* One stage holds the clock and the music controls, so the player stays mounted (and
          keeps playing) when focus mode moves it from the bottom bar to under the clock. */}
      <div className="stage">
        {focus.focused && <FocusClock />}
        <MusicPlayer onThemeChange={setTheme} layout={focus.focused ? 'center' : 'bar'} />
      </div>
    </div>
  )
}
